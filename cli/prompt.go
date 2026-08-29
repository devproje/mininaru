// SPDX-FileCopyrightText: 2026 Wonhyeok Kim (Project_IO)
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"bufio"
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
	"golang.org/x/term"
)

const (
	promptDialTimeout time.Duration = 3 * time.Second
	promptDefaultUrl  string        = "ws://127.0.0.1:8223/ws"
	promptDim         string        = "\x1b[2m"
	promptReset       string        = "\x1b[0m"
)

type promptFrame struct {
	Type      string `json:"type,omitempty"`
	SessionId string `json:"session_id"`
	Content   string `json:"content,omitempty"`
	Cwd       string `json:"cwd,omitempty"`
	Decision  string `json:"decision,omitempty"`
}

type promptReply struct {
	Type      string                      `json:"type"`
	SessionId string                      `json:"session_id"`
	Chunk     *openai.ChatCompletionChunk `json:"chunk,omitempty"`
	Reasoning string                      `json:"reasoning,omitempty"`
	Message   string                      `json:"message,omitempty"`
	Name      string                      `json:"name,omitempty"`
	Status    string                      `json:"status,omitempty"`
	Arguments string                      `json:"arguments,omitempty"`
}

func promptApiBase(endpoint string) (string, error) {
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

func promptApi(method string, endpoint string, apiKey string, payload any, out any) error {
	var body []byte
	var reader io.Reader
	var req *http.Request
	var res *http.Response

	var err error

	if payload != nil {
		body, err = json.Marshal(payload)
		if err != nil {
			return err
		}

		reader = bytes.NewReader(body)
	}

	req, err = http.NewRequest(method, endpoint, reader)
	if err != nil {
		return err
	}

	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	if apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+apiKey)
	}

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

	if out == nil || len(bytes.TrimSpace(body)) == 0 {
		return nil
	}

	return json.Unmarshal(body, out)
}

func promptAgentId(base string, apiKey string, name string) (string, error) {
	var list []*core.Agent
	var item *core.Agent

	var err error

	err = promptApi(http.MethodGet, base+"/agents", apiKey, nil, &list)
	if err != nil {
		return "", err
	}

	if len(list) == 0 {
		return "", fmt.Errorf("no agent is registered on the server")
	}

	if name == "" {
		return list[0].Id, nil
	}

	for _, item = range list {
		if item.Id == name || item.Name == name {
			return item.Id, nil
		}
	}

	return "", fmt.Errorf("agent %q not found", name)
}

func promptSession(base string, apiKey string, seed string, agent string) (string, error) {
	var session core.Session
	var created core.Session
	var agentId string

	var err error

	if seed != "" {
		err = promptApi(http.MethodGet, base+"/sessions/"+seed, apiKey, nil, &session)
		if err != nil {
			return "", err
		}

		return session.Id, nil
	}

	agentId, err = promptAgentId(base, apiKey, agent)
	if err != nil {
		return "", err
	}

	err = promptApi(http.MethodPost, base+"/sessions", apiKey, map[string]string{"agent_id": agentId}, &created)
	if err != nil {
		return "", err
	}

	return created.Id, nil
}

func isLoopbackURL(endpoint string) bool {
	var parsed *url.URL
	var host string

	var err error

	parsed, err = url.Parse(endpoint)
	if err != nil {
		return false
	}

	host = parsed.Hostname()

	return host == "127.0.0.1" || host == "localhost" || host == "::1"
}

func resolveApiKey(explicit string, endpoint string) string {
	var fromEnv string
	var fromFile []byte

	var err error

	if explicit != "" {
		return explicit
	}

	fromEnv = os.Getenv("MININARU_API_KEY")
	if fromEnv != "" {
		return fromEnv
	}

	if !isLoopbackURL(endpoint) {
		return ""
	}

	fromFile, err = os.ReadFile(util.Path("mininaru.key"))
	if err != nil {
		return ""
	}

	return strings.TrimSpace(string(fromFile))
}

func promptDial(endpoint string, apiKey string) (*websocket.Conn, error) {
	var dialer websocket.Dialer
	var header http.Header
	var conn *websocket.Conn

	var err error

	dialer = websocket.Dialer{HandshakeTimeout: promptDialTimeout}

	if apiKey != "" {
		header = http.Header{"Authorization": []string{"Bearer " + apiKey}}
	}

	conn, _, err = dialer.Dial(endpoint, header)
	if err != nil {
		return nil, err
	}

	return conn, nil
}

func promptAnswer(reader *bufio.Reader) string {
	var fd int
	var state *term.State
	var buf []byte
	var line string

	var err error

	fd = int(os.Stdin.Fd())

	if !term.IsTerminal(fd) {
		line, err = reader.ReadString('\n')
		if err != nil && line == "" {
			return ""
		}

		return strings.ToLower(strings.TrimSpace(line))
	}

	state, err = term.MakeRaw(fd)
	if err != nil {
		return ""
	}
	defer term.Restore(fd, state)

	buf = make([]byte, 1)

	_, err = os.Stdin.Read(buf)
	if err != nil {
		return ""
	}

	return strings.ToLower(string(buf))
}

func promptDecide(reader *bufio.Reader, name string, arguments string) string {
	var answer string

	fmt.Printf("\n%stool %s wants to run%s %s\n", promptDim, name, promptReset, strings.TrimSpace(arguments))
	fmt.Printf("allow? [y/N]: ")

	answer = promptAnswer(reader)

	switch answer {
	case "y", "yes":
		fmt.Println("y")
		return "once"
	}

	fmt.Println("N")

	return "deny"
}

func promptEndLine(mode string) string {
	if mode != "" {
		fmt.Println(promptReset)
	}

	return ""
}

func promptReceive(conn *websocket.Conn, session string) error {
	var reply promptReply
	var reader *bufio.Reader
	var text string
	var mode string
	var next string

	var err error

	reader = bufio.NewReader(os.Stdin)

	for {
		reply = promptReply{}

		err = conn.ReadJSON(&reply)
		if err != nil {
			return err
		}

		text = ""
		next = ""

		switch reply.Type {
		case "chunk":
			if reply.Reasoning != "" {
				text = reply.Reasoning
				next = "reasoning"
				break
			}

			if reply.Chunk != nil && len(reply.Chunk.Choices) > 0 {
				text = reply.Chunk.Choices[0].Delta.Content
				next = "content"
			}
		case "message":
			text = reply.Message
			next = "content"
		case "tool":
			mode = promptEndLine(mode)
			fmt.Printf("%s· %s %s %s%s\n", promptDim, reply.Name, reply.Status, reply.Message, promptReset)
		case "approval_request":
			mode = promptEndLine(mode)

			err = conn.WriteJSON(promptFrame{
				Type:      "approval",
				SessionId: session,
				Decision:  promptDecide(reader, reply.Name, reply.Arguments),
			})
			if err != nil {
				return err
			}
		case "error", "done":
			promptEndLine(mode)

			if reply.Type == "error" {
				return fmt.Errorf("%s", reply.Message)
			}

			return nil
		}

		if text == "" {
			continue
		}

		if next != mode {
			promptEndLine(mode)

			if next == "reasoning" {
				fmt.Print(promptDim)
			}

			mode = next
		}

		fmt.Print(text)
	}
}

func shortPrompt(prompt string) error {
	var base string
	var apiKey string
	var session string
	var cwd string
	var conn *websocket.Conn

	var err error

	prompt = strings.TrimSpace(prompt)
	if prompt == "" {
		return fmt.Errorf("prompt is empty")
	}

	apiKey = resolveApiKey(promptApiKeyRef, promptUrlRef)

	base, err = promptApiBase(promptUrlRef)
	if err != nil {
		return err
	}

	session, err = promptSession(base, apiKey, promptSessionRef, promptAgentRef)
	if err != nil {
		return err
	}

	if promptSessionRef == "" {
		defer promptApi(http.MethodDelete, base+"/sessions/"+session, apiKey, nil, nil)
	}

	cwd, err = os.Getwd()
	if err != nil {
		return err
	}

	conn, err = promptDial(promptUrlRef, apiKey)
	if err != nil {
		return err
	}
	defer conn.Close()

	err = conn.WriteJSON(promptFrame{Type: "attach", SessionId: session})
	if err != nil {
		return err
	}

	err = conn.WriteJSON(promptFrame{SessionId: session, Content: prompt, Cwd: cwd})
	if err != nil {
		return err
	}

	return promptReceive(conn, session)
}
