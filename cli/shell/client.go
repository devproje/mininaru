// SPDX-FileCopyrightText: 2026 Wonhyeok Kim (Project_IO)
// SPDX-License-Identifier: GPL-3.0-or-later

package shell

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/devproje/mininaru/core"
	"github.com/devproje/mininaru/util"
	"github.com/gorilla/websocket"
	"github.com/openai/openai-go"
)

const interruptPollInterval time.Duration = 100 * time.Millisecond

const (
	pongWait   time.Duration = 60 * time.Second
	pingPeriod time.Duration = 25 * time.Second
	writeWait  time.Duration = 10 * time.Second
)

type frame struct {
	Type      string `json:"type,omitempty"`
	SessionId string `json:"session_id"`
	Content   string `json:"content,omitempty"`
	Cwd       string `json:"cwd,omitempty"`
}

type approvalResponse struct {
	Type      string `json:"type"`
	SessionId string `json:"session_id"`
	Decision  string `json:"decision"`
}

type dialConfig struct {
	url     string
	apiKey  string
	seed    string
	agent   string
	session string
}

type dialResult struct {
	conn          *websocket.Conn
	session       string
	name          string
	thinkingLevel string
	agentId       string
	err           error
}

type renderState struct {
	thinking    bool
	streaming   bool
	toolStack   []string
	toolSpin    func()
	stopSpinner func()
	watch       *interruptWatch
	active      bool
}

type inbound struct {
	reply reply
	err   error
}

type interruptWatch struct {
	done        chan struct{}
	interrupted <-chan struct{}
	exited      <-chan struct{}
	captured    *[]byte
	stopped     bool
}

type reply struct {
	Type      string                      `json:"type"`
	SessionId string                      `json:"session_id"`
	Chunk     *openai.ChatCompletionChunk `json:"chunk,omitempty"`
	Reasoning string                      `json:"reasoning,omitempty"`
	Message   string                      `json:"message,omitempty"`
	Name      string                      `json:"name,omitempty"`
	Status    string                      `json:"status,omitempty"`
	Arguments string                      `json:"arguments,omitempty"`
}

func apiBase(endpoint string) (string, error) {
	var parsed *url.URL

	var err error

	parsed, err = url.Parse(endpoint)
	if err != nil {
		return "", err
	}

	switch parsed.Scheme {
	case "ws":
		parsed.Scheme = "http"
	case "wss":
		parsed.Scheme = "https"
	}

	parsed.Path = "/api"
	parsed.RawQuery = ""

	return parsed.String(), nil
}

func authorize(req *http.Request, apiKey string) {
	if apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+apiKey)
	}
}

func apiGet(endpoint string, apiKey string, out any) error {
	var req *http.Request
	var res *http.Response
	var body []byte

	var err error

	req, err = http.NewRequest(http.MethodGet, endpoint, nil)
	if err != nil {
		return err
	}

	authorize(req, apiKey)

	res, err = http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()

	body, err = io.ReadAll(res.Body)
	if err != nil {
		return err
	}

	if res.StatusCode >= http.StatusBadRequest {
		return fmt.Errorf("%s: %s", res.Status, strings.TrimSpace(string(body)))
	}

	return json.Unmarshal(body, out)
}

func apiSend(method string, endpoint string, apiKey string, payload any, out any) error {
	var body []byte
	var req *http.Request
	var res *http.Response

	var err error

	body, err = json.Marshal(payload)
	if err != nil {
		return err
	}

	req, err = http.NewRequest(method, endpoint, bytes.NewReader(body))
	if err != nil {
		return err
	}

	req.Header.Set("Content-Type", "application/json")
	authorize(req, apiKey)

	res, err = http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()

	body, err = io.ReadAll(res.Body)
	if err != nil {
		return err
	}

	if res.StatusCode >= http.StatusBadRequest {
		return fmt.Errorf("%s: %s", res.Status, strings.TrimSpace(string(body)))
	}

	return json.Unmarshal(body, out)
}

func apiPost(endpoint string, apiKey string, payload any, out any) error {
	return apiSend(http.MethodPost, endpoint, apiKey, payload, out)
}

func apiPatch(endpoint string, apiKey string, payload any, out any) error {
	return apiSend(http.MethodPatch, endpoint, apiKey, payload, out)
}

func refreshYoloMode(sh *state) {
	var base string
	var resp map[string]any

	var err error

	base, err = apiBase(sh.url)
	if err != nil {
		return
	}

	err = apiGet(base+"/yolo?cwd="+url.QueryEscape(sh.cwd), sh.apiKey, &resp)
	if err != nil {
		return
	}

	sh.yoloMode, _ = resp["mode"].(string)
}

func resolveAgentByIdOrName(sh *state, base, idOrName string) (*core.Agent, error) {
	var agent core.Agent
	var list []*core.Agent
	var item *core.Agent

	var err error

	err = apiGet(base+"/agents/"+idOrName, sh.apiKey, &agent)
	if err == nil {
		return &agent, nil
	}

	err = apiGet(base+"/agents", sh.apiKey, &list)
	if err != nil {
		return nil, err
	}

	for _, item = range list {
		if item.Name == idOrName {
			return item, nil
		}
	}

	return nil, fmt.Errorf("agent %q not found", idOrName)
}

func pickAgent(cfg dialConfig, base string) (*core.Agent, error) {
	var list []*core.Agent
	var item *core.Agent

	var err error

	err = apiGet(base+"/agents", cfg.apiKey, &list)
	if err != nil {
		return nil, err
	}

	if len(list) == 0 {
		return nil, fmt.Errorf("no agent is registered on the server")
	}

	if cfg.agent == "" {
		return list[0], nil
	}

	for _, item = range list {
		if item.Name == cfg.agent {
			return item, nil
		}
	}

	return nil, fmt.Errorf("agent %q not found", cfg.agent)
}

func seedAgent(cfg dialConfig, base string) (*core.Agent, error) {
	var session core.Session
	var agent core.Agent

	var err error

	err = apiGet(base+"/sessions/"+cfg.seed, cfg.apiKey, &session)
	if err != nil {
		return nil, err
	}

	err = apiGet(base+"/agents/"+session.AgentId, cfg.apiKey, &agent)
	if err != nil {
		return nil, err
	}

	return &agent, nil
}

func openSession(cfg dialConfig) (string, string, string, string, error) {
	var base string
	var agent *core.Agent

	var err error

	base, err = apiBase(cfg.url)
	if err != nil {
		return "", "", "", "", err
	}

	if cfg.seed != "" {
		agent, err = seedAgent(cfg, base)
		if err != nil {
			return "", "", "", "", err
		}

		return cfg.seed, agent.Name, agent.ThinkingLevel, agent.Id, nil
	}

	agent, err = pickAgent(cfg, base)
	if err != nil {
		return "", "", "", "", err
	}

	return "", agent.Name, agent.ThinkingLevel, agent.Id, nil
}

func shellDialConfig(sh *state) dialConfig {
	var cfg dialConfig

	cfg.url = sh.url
	cfg.apiKey = sh.apiKey
	cfg.seed = sh.seed
	cfg.agent = sh.agent
	cfg.session = sh.session

	return cfg
}

func dialAgent(cfg dialConfig) dialResult {
	var result dialResult
	var dialer websocket.Dialer
	var header http.Header

	result.session = cfg.session
	if result.session == "" {
		result.session, result.name, result.thinkingLevel, result.agentId, result.err = openSession(cfg)
		if result.err != nil {
			return result
		}
	}

	dialer = websocket.Dialer{HandshakeTimeout: DIAL_TIMEOUT}

	if cfg.apiKey != "" {
		header = http.Header{"Authorization": []string{"Bearer " + cfg.apiKey}}
	}

	result.conn, _, result.err = dialer.Dial(cfg.url, header)

	return result
}

func writeJSON(sh *state, v any) error {
	sh.connMu.Lock()
	defer sh.connMu.Unlock()

	if sh.conn == nil {
		return fmt.Errorf("not connected")
	}

	sh.conn.SetWriteDeadline(time.Now().Add(writeWait))
	return sh.conn.WriteJSON(v)
}

func pingLoop(sh *state, conn *websocket.Conn) {
	var tick *time.Ticker
	var err error

	tick = time.NewTicker(pingPeriod)
	defer tick.Stop()

	for range tick.C {
		sh.connMu.Lock()
		if sh.conn != conn {
			sh.connMu.Unlock()
			return
		}

		conn.SetWriteDeadline(time.Now().Add(writeWait))
		err = conn.WriteControl(websocket.PingMessage, nil, time.Now().Add(writeWait))
		sh.connMu.Unlock()

		if err != nil {
			return
		}
	}
}

func adoptConn(sh *state, result dialResult) {
	var err error

	sh.conn = result.conn
	sh.session = result.session
	sh.mirror = &renderState{}
	sh.retryDelay = 0
	sh.retryAt = time.Time{}

	if result.name != "" {
		sh.name = result.name
	}

	if result.thinkingLevel != "" {
		sh.thinkingLevel = result.thinkingLevel
	}

	if result.agentId != "" {
		sh.agentId = result.agentId
	}

	sh.conn.SetReadDeadline(time.Now().Add(pongWait))
	sh.conn.SetPongHandler(func(string) error {
		return result.conn.SetReadDeadline(time.Now().Add(pongWait))
	})

	ensureReader(sh)
	go pingLoop(sh, sh.conn)

	if sh.session != "" {
		err = writeJSON(sh, frame{Type: "attach", SessionId: sh.session})
		if err != nil {
			util.Log.Debug("shell attach frame failed", "error", err)
		}
	}
}

func armRetry(sh *state) {
	if sh.retryDelay == 0 {
		sh.retryDelay = RETRY_MIN
	} else {
		sh.retryDelay = sh.retryDelay * 2
	}

	if sh.retryDelay > RETRY_MAX {
		sh.retryDelay = RETRY_MAX
	}

	sh.retryAt = time.Now().Add(sh.retryDelay)
}

func startDial(sh *state) {
	var cfg dialConfig
	var out chan dialResult

	if sh.dial != nil || sh.conn != nil {
		return
	}

	cfg = shellDialConfig(sh)
	out = make(chan dialResult, 1)
	sh.dial = out

	go func() {
		out <- dialAgent(cfg)
	}()
}

func adoptDial(sh *state, block bool) bool {
	var result dialResult
	var ok bool

	if sh.dial == nil {
		return false
	}

	if block {
		result, ok = <-sh.dial
	} else {
		select {
		case result, ok = <-sh.dial:
		default:
			return false
		}
	}

	sh.dial = nil

	if !ok || result.err != nil {
		armRetry(sh)
		return false
	}

	adoptConn(sh, result)
	refreshYoloMode(sh)

	if sh.wasAgent {
		sh.mode = MODE_AGENT
		sh.wasAgent = false
	}

	write("\r\x1b[0J")
	notice(GREEN, "●", "%sreconnected%s %s", GREEN, RESET, connectionDetail(sh))

	return true
}

func retryConnect(sh *state) bool {
	var adopted bool

	adopted = adoptDial(sh, false)

	if sh.conn == nil && sh.dial == nil && !sh.retryAt.IsZero() && !time.Now().Before(sh.retryAt) {
		startDial(sh)
	}

	return adopted
}

func connect(sh *state) error {
	var result dialResult

	result = dialAgent(shellDialConfig(sh))
	if result.err != nil {
		armRetry(sh)
		return result.err
	}

	adoptConn(sh, result)

	return nil
}

func disconnect(sh *state, reason error) {
	if sh.conn == nil {
		return
	}

	sh.connMu.Lock()
	sh.conn.Close()
	sh.conn = nil
	sh.connMu.Unlock()

	sh.frames = nil
	sh.mirror = nil
	sh.wasAgent = sh.mode == MODE_AGENT
	sh.mode = MODE_SHELL
	sh.retryDelay = 0

	armRetry(sh)

	notice(RED, "●", "%sdisconnected%s %s", RED, RESET, DIM+reason.Error()+", reconnecting…"+RESET)
}

func chunkText(chunk *openai.ChatCompletionChunk) string {
	if chunk == nil || len(chunk.Choices) == 0 {
		return ""
	}

	return chunk.Choices[0].Delta.Content
}

func watchInterrupt(done <-chan struct{}) (<-chan struct{}, <-chan struct{}, *[]byte) {
	var interrupted chan struct{}
	var exited chan struct{}
	var captured []byte

	interrupted = make(chan struct{})
	exited = make(chan struct{})

	go func() {
		var buf []byte
		var count int

		var err error

		defer close(exited)

		buf = make([]byte, 1)

		for {
			select {
			case <-done:
				return
			default:
			}

			if !pollStdin(interruptPollInterval) {
				continue
			}

			count, err = os.Stdin.Read(buf)
			if err != nil || count == 0 {
				continue
			}

			if buf[0] == 0x1b || buf[0] == 0x03 {
				close(interrupted)
				return
			}

			captured = append(captured, buf[0])
		}
	}()

	return interrupted, exited, &captured
}

func newInterruptWatch() *interruptWatch {
	var w interruptWatch

	w.done = make(chan struct{})
	w.interrupted, w.exited, w.captured = watchInterrupt(w.done)

	return &w
}

func (w *interruptWatch) pause() {
	if w.stopped {
		return
	}

	close(w.done)
	<-w.exited
	w.stopped = true
}

func confirmPrompt(question string) bool {
	var buf [1]byte

	var err error

	write("%s%s [y/N]: %s", YELLOW, question, RESET)

	_, err = os.Stdin.Read(buf[:])

	write("\n")

	if err != nil {
		return false
	}

	return buf[0] == 'y' || buf[0] == 'Y'
}

func approvalPrompt(sh *state, name, arguments string) string {
	var buf [1]byte

	var err error

	write("%s⚠ %s wants to run %s%s%s\n", YELLOW, agentLabel(sh), PURPLE, name, RESET)
	if arguments != "" {
		write("%s%s%s\n", DIM, arguments, RESET)
	}
	write("%sAllow this call? [y]es once / [a]lways this session / [N]o: %s", YELLOW, RESET)

	_, err = os.Stdin.Read(buf[:])

	write("\n")

	if err != nil {
		return "deny"
	}

	switch buf[0] {
	case 'y', 'Y':
		return "once"
	case 'a', 'A':
		return "session"
	default:
		return "deny"
	}
}

func isReasoningFiller(s string) bool {
	var dotSeen bool
	var r rune

	for _, r = range s {
		if r == '.' {
			dotSeen = true
			continue
		}
		if r == ' ' || r == '\t' || r == '\n' || r == '\r' {
			continue
		}

		return false
	}

	return dotSeen
}

func removeToolName(stack []string, name string) []string {
	var index int

	for index = len(stack) - 1; index >= 0; index-- {
		if stack[index] == name {
			return append(stack[:index], stack[index+1:]...)
		}
	}

	return stack
}

func (rs *renderState) stop() {
	if rs.stopSpinner == nil {
		return
	}

	rs.stopSpinner()
	rs.stopSpinner = nil
}

func (rs *renderState) closeBlocks() {
	if rs.thinking && !rs.streaming {
		write("%s\n", RESET)
		rs.thinking = false
	}

	if rs.streaming {
		write("%s\n", RESET)
		rs.streaming = false
	}
}

func (rs *renderState) stopToolSpin() {
	if rs.toolSpin == nil {
		return
	}

	rs.toolSpin()
	rs.toolSpin = nil
}

func (rs *renderState) startToolSpin() {
	if len(rs.toolStack) == 0 {
		return
	}

	rs.toolSpin = spinner(rs.toolStack[len(rs.toolStack)-1])
}

func renderFrame(sh *state, rs *renderState, reply reply) (bool, error) {
	var text string
	var decision string

	var err error

	switch reply.Type {
	case "message":
		rs.stop()
		rs.closeBlocks()

		write("%s↘ from session %s%s\n", GRAY, reply.Name, RESET)
		write("%s  %s%s\n\n", DIM, reply.Message, RESET)
	case "chunk":
		text = chunkText(reply.Chunk)

		if reply.Reasoning != "" && !rs.streaming && !isReasoningFiller(reply.Reasoning) {
			if !rs.thinking {
				rs.stop()
				write("%s◇ thinking%s\n%s", GRAY, RESET, DIM)
				rs.thinking = true
			}

			write("%s", reply.Reasoning)
		}

		if text == "" {
			return false, nil
		}

		if !rs.streaming {
			rs.stop()

			if rs.thinking {
				write("%s\n\n", RESET)
			}

			write("%s◆ %s%s\n", PURPLE, agentLabel(sh), RESET)
			rs.streaming = true
		}

		write("%s", text)
	case "tool":
		rs.stop()
		rs.closeBlocks()
		rs.stopToolSpin()

		switch reply.Status {
		case "started":
			if reply.Message != "" {
				write("%s  %s%s\n", DIM, reply.Message, RESET)
			}

			rs.toolStack = append(rs.toolStack, reply.Name)
		case "finished":
			rs.toolStack = removeToolName(rs.toolStack, reply.Name)
			write("%s✔ %s%s\n", GREEN, reply.Name, RESET)
		case "failed":
			rs.toolStack = removeToolName(rs.toolStack, reply.Name)
			write("%s✖ %s failed%s\n", RED, reply.Name, RESET)
			if reply.Message != "" {
				write("%s  %s%s\n", DIM, reply.Message, RESET)
			}
		}

		rs.startToolSpin()
	case "approval_request":
		rs.stop()
		rs.closeBlocks()
		rs.stopToolSpin()

		if rs.watch != nil {
			rs.watch.pause()
		}

		decision = approvalPrompt(sh, reply.Name, reply.Arguments)

		err = writeJSON(sh, approvalResponse{Type: "approval", SessionId: reply.SessionId, Decision: decision})
		if err != nil {
			return false, err
		}

		rs.startToolSpin()
	case "error":
		rs.stop()

		if rs.thinking && !rs.streaming {
			write("%s\n", RESET)
		}

		rs.stopToolSpin()
		notice(RED, "✖", "%s", reply.Message)

		return true, nil
	case "done":
		rs.stop()

		if rs.thinking && !rs.streaming {
			write("%s", RESET)
		}

		rs.stopToolSpin()
		write("\n\n")

		return true, nil
	}

	return false, nil
}

func receiveAgent(sh *state, stop func(), watch *interruptWatch) error {
	var rs renderState
	var item inbound
	var ok bool
	var terminal bool

	var err error

	rs.stopSpinner = stop
	rs.watch = watch

	for {
		item, ok = nextFrame(sh, true)
		if !ok {
			rs.stop()
			return io.ErrUnexpectedEOF
		}
		if item.err != nil {
			rs.stop()
			return item.err
		}

		terminal, err = renderFrame(sh, &rs, item.reply)
		if err != nil {
			return err
		}
		if terminal {
			return nil
		}
	}
}

func readFrames(conn *websocket.Conn, frames chan<- inbound) {
	var item inbound

	defer close(frames)

	for {
		item = inbound{}

		item.err = conn.ReadJSON(&item.reply)
		frames <- item

		if item.err != nil {
			return
		}
	}
}

func ensureReader(sh *state) {
	if sh.conn == nil || sh.frames != nil {
		return
	}

	sh.frames = make(chan inbound, 64)

	go readFrames(sh.conn, sh.frames)
}

func nextFrame(sh *state, block bool) (inbound, bool) {
	var item inbound
	var ok bool

	if sh.frames == nil {
		return inbound{}, false
	}

	if block {
		item, ok = <-sh.frames
		return item, ok
	}

	select {
	case item, ok = <-sh.frames:
		return item, ok
	default:
		return inbound{}, false
	}
}

func drainMirror(sh *state) bool {
	var rendered bool
	var item inbound
	var ok bool
	var terminal bool

	var err error

	if sh.conn == nil {
		return false
	}

	ensureReader(sh)

	if sh.mirror == nil {
		sh.mirror = &renderState{}
	}

	for {
		item, ok = nextFrame(sh, sh.mirror.active)
		if !ok {
			return rendered
		}

		if item.err != nil {
			disconnect(sh, item.err)
			return rendered
		}

		if !rendered {
			write("\r\x1b[0J")
			rendered = true
		}

		sh.mirror.active = true

		terminal, err = renderFrame(sh, sh.mirror, item.reply)
		if err != nil {
			disconnect(sh, err)
			return rendered
		}

		if terminal {
			sh.mirror = &renderState{}
			return rendered
		}
	}
}

func awaitMirror(sh *state) {
	for sh.conn != nil && sh.mirror != nil && sh.mirror.active {
		drainMirror(sh)
	}
}

func ensureSession(sh *state) error {
	var base string
	var created core.Session

	var err error

	if sh.session != "" {
		return nil
	}

	if sh.agentId == "" {
		return fmt.Errorf("no agent to start a session with")
	}

	base, err = apiBase(sh.url)
	if err != nil {
		return err
	}

	err = apiPost(base+"/sessions", sh.apiKey, map[string]string{"agent_id": sh.agentId}, &created)
	if err != nil {
		return err
	}

	sh.session = created.Id

	return writeJSON(sh, frame{Type: "attach", SessionId: sh.session})
}

func sendAgent(sh *state, content string) error {
	var stop func()
	var watch *interruptWatch
	var result chan error

	var err error

	ensureReader(sh)
	awaitMirror(sh)

	if sh.conn == nil {
		return fmt.Errorf("not connected")
	}

	err = ensureSession(sh)
	if err != nil {
		return err
	}

	sh.mirror = &renderState{}

	err = writeJSON(sh, frame{SessionId: sh.session, Content: content, Cwd: sh.cwd})
	if err != nil {
		return err
	}

	stop = spinnerWords(thinkingWords)

	watch = newInterruptWatch()
	result = make(chan error, 1)

	go func() {
		result <- receiveAgent(sh, stop, watch)
	}()

	select {
	case <-watch.interrupted:
		stop()
		sh.conn.Close()
		sh.conn = nil
		sh.frames = nil
		sh.mirror = nil

		watch.pause()
		sh.pendingInput = append(sh.pendingInput, *watch.captured...)

		notice(YELLOW, "○", "%sinterrupted%s", YELLOW, RESET)

		err = connect(sh)
		if err != nil {
			sh.wasAgent = sh.mode == MODE_AGENT
			sh.mode = MODE_SHELL
			notice(RED, "●", "%sdisconnected%s %s", RED, RESET, DIM+err.Error()+", reconnecting…"+RESET)
			return nil
		}

		return nil
	case err = <-result:
		watch.pause()
		sh.pendingInput = append(sh.pendingInput, *watch.captured...)

		return err
	}
}

func toggleMode(sh *state) {
	var err error

	if sh.mode == MODE_AGENT {
		sh.mode = MODE_SHELL
		return
	}

	if sh.conn == nil && sh.dial != nil {
		adoptDial(sh, true)
	}

	if sh.conn == nil {
		err = connect(sh)
		if err != nil {
			notice(YELLOW, "○", "%sstill offline%s %s", YELLOW, RESET, DIM+err.Error()+RESET)
			return
		}

		notice(GREEN, "●", "%sconnected%s %s", GREEN, RESET, connectionDetail(sh))
		refreshYoloMode(sh)
	}

	sh.mode = MODE_AGENT
}
