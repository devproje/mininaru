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
	"github.com/gorilla/websocket"
	"github.com/openai/openai-go"
)

const interruptPollInterval time.Duration = 100 * time.Millisecond

type frame struct {
	SessionId string `json:"session_id"`
	Content   string `json:"content"`
	Cwd       string `json:"cwd,omitempty"`
}

type approvalResponse struct {
	Type      string `json:"type"`
	SessionId string `json:"session_id"`
	Decision  string `json:"decision"`
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

func apiPost(endpoint string, apiKey string, payload any, out any) error {
	var body []byte
	var req *http.Request
	var res *http.Response

	var err error

	body, err = json.Marshal(payload)
	if err != nil {
		return err
	}

	req, err = http.NewRequest(http.MethodPost, endpoint, bytes.NewReader(body))
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

func pickAgent(sh *state, base string) (*core.Agent, error) {
	var list []*core.Agent
	var item *core.Agent

	var err error

	err = apiGet(base+"/agents", sh.apiKey, &list)
	if err != nil {
		return nil, err
	}

	if len(list) == 0 {
		return nil, fmt.Errorf("no agent is registered on the server")
	}

	if sh.agent == "" {
		return list[0], nil
	}

	for _, item = range list {
		if item.Name == sh.agent {
			return item, nil
		}
	}

	return nil, fmt.Errorf("agent %q not found", sh.agent)
}

func seedAgent(sh *state, base string) (*core.Agent, error) {
	var session core.Session
	var agent core.Agent

	var err error

	err = apiGet(base+"/sessions/"+sh.seed, sh.apiKey, &session)
	if err != nil {
		return nil, err
	}

	err = apiGet(base+"/agents/"+session.AgentId, sh.apiKey, &agent)
	if err != nil {
		return nil, err
	}

	return &agent, nil
}

func openSession(sh *state) (string, error) {
	var base string
	var agent *core.Agent
	var session core.Session

	var err error

	base, err = apiBase(sh.url)
	if err != nil {
		return "", err
	}

	if sh.seed != "" {
		agent, err = seedAgent(sh, base)
		if err != nil {
			return "", err
		}

		sh.name = agent.Name

		return sh.seed, nil
	}

	agent, err = pickAgent(sh, base)
	if err != nil {
		return "", err
	}

	err = apiPost(base+"/sessions", sh.apiKey, map[string]string{"agent_id": agent.Id, "name": "shell"}, &session)
	if err != nil {
		return "", err
	}

	sh.name = agent.Name

	return session.Id, nil
}

func connect(sh *state) error {
	var dialer websocket.Dialer
	var header http.Header
	var conn *websocket.Conn
	var session string

	var err error

	session = sh.session
	if session == "" {
		session, err = openSession(sh)
		if err != nil {
			return err
		}
	}

	dialer = websocket.Dialer{HandshakeTimeout: DIAL_TIMEOUT}

	if sh.apiKey != "" {
		header = http.Header{"Authorization": []string{"Bearer " + sh.apiKey}}
	}

	conn, _, err = dialer.Dial(sh.url, header)
	if err != nil {
		return err
	}

	sh.conn = conn
	sh.session = session

	return nil
}

func disconnect(sh *state, reason error) {
	if sh.conn == nil {
		return
	}

	sh.conn.Close()
	sh.conn = nil
	sh.mode = MODE_BASH

	notice(RED, "●", "%sdisconnected%s %s", RED, RESET, DIM+reason.Error()+", back to bash mode"+RESET)
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

			if buf[0] == 0x1b {
				close(interrupted)
				return
			}

			captured = append(captured, buf[0])
		}
	}()

	return interrupted, exited, &captured
}

type interruptWatch struct {
	done        chan struct{}
	interrupted <-chan struct{}
	exited      <-chan struct{}
	captured    *[]byte
	stopped     bool
}

func newInterruptWatch() *interruptWatch {
	var w interruptWatch

	w.done = make(chan struct{})
	w.interrupted, w.exited, w.captured = watchInterrupt(w.done)

	return &w
}

// pause stops the interrupt watcher so its stdin-reading goroutine isn't
// racing a synchronous read elsewhere (e.g. an approval prompt). It is not
// resumed afterward — ESC-to-interrupt stays unavailable for the remainder
// of that turn, but nothing already typed is lost: unread bytes simply stay
// buffered in the terminal until the next readLine() call picks them up.
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

func receiveAgent(sh *state, stop func(), watch *interruptWatch) error {
	var reply reply
	var text string
	var thinking bool
	var streaming bool
	var decision string

	var err error

	for {
		err = sh.conn.ReadJSON(&reply)
		if err != nil {
			stop()
			return err
		}

		switch reply.Type {
		case "chunk":
			text = chunkText(reply.Chunk)

			if reply.Reasoning != "" && !streaming {
				if !thinking {
					stop()
					write("%s◇ thinking%s\n%s", GRAY, RESET, DIM)
					thinking = true
				}

				write("%s", reply.Reasoning)
			}

			if text == "" {
				continue
			}

			if !streaming {
				stop()

				if thinking {
					write("%s\n\n", RESET)
				}

				write("%s◆ %s%s\n", PURPLE, agentLabel(sh), RESET)
				streaming = true
			}

			write("%s", text)
		case "tool":
			stop()

			if thinking && !streaming {
				write("%s", RESET)
				thinking = false
			}
			if streaming {
				write("%s\n", RESET)
				streaming = false
			}

			switch reply.Status {
			case "started":
				write("%s⚙ %s%s\n", GRAY, reply.Name, RESET)
			case "failed":
				write("%s✖ %s failed%s\n", RED, reply.Name, RESET)
			}
		case "approval_request":
			stop()

			if thinking && !streaming {
				write("%s", RESET)
				thinking = false
			}
			if streaming {
				write("%s\n", RESET)
				streaming = false
			}

			watch.pause()
			decision = approvalPrompt(sh, reply.Name, reply.Arguments)

			err = sh.conn.WriteJSON(approvalResponse{Type: "approval", SessionId: reply.SessionId, Decision: decision})
			if err != nil {
				return err
			}
		case "error":
			stop()

			if thinking && !streaming {
				write("%s\n", RESET)
			}

			notice(RED, "✖", "%s", reply.Message)
			return nil
		case "done":
			stop()

			if thinking && !streaming {
				write("%s", RESET)
			}

			write("\n\n")
			return nil
		}
	}
}

func sendAgent(sh *state, content string) error {
	var stop func()
	var watch *interruptWatch
	var result chan error

	var err error

	err = sh.conn.WriteJSON(frame{SessionId: sh.session, Content: content, Cwd: sh.cwd})
	if err != nil {
		return err
	}

	stop = spinner("thinking…")

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

		watch.pause()
		sh.pendingInput = append(sh.pendingInput, *watch.captured...)

		notice(YELLOW, "○", "%sinterrupted%s", YELLOW, RESET)

		err = connect(sh)
		if err != nil {
			sh.mode = MODE_BASH
			notice(RED, "●", "%sdisconnected%s %s", RED, RESET, DIM+err.Error()+", back to bash mode"+RESET)
			return nil
		}

		notice(GREEN, "●", "%sreconnected%s %s", GREEN, RESET, DIM+sh.url+" · session "+sh.session+RESET)

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
		sh.mode = MODE_BASH
		return
	}

	if sh.conn == nil {
		err = connect(sh)
		if err != nil {
			notice(YELLOW, "○", "%sstill offline%s %s", YELLOW, RESET, DIM+err.Error()+RESET)
			return
		}

		notice(GREEN, "●", "%sconnected%s %s", GREEN, RESET, DIM+sh.url+" · session "+sh.session+RESET)
		refreshYoloMode(sh)
	}

	sh.mode = MODE_AGENT
}
