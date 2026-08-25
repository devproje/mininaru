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
	"strings"

	"github.com/devproje/mininaru/core"
	"github.com/gorilla/websocket"
	"github.com/openai/openai-go"
)

type frame struct {
	SessionId string `json:"session_id"`
	Content   string `json:"content"`
}

type reply struct {
	Type      string                      `json:"type"`
	SessionId string                      `json:"session_id"`
	Chunk     *openai.ChatCompletionChunk `json:"chunk,omitempty"`
	Reasoning string                      `json:"reasoning,omitempty"`
	Message   string                      `json:"message,omitempty"`
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

func apiGet(endpoint string, out any) error {
	var res *http.Response
	var body []byte

	var err error

	res, err = http.Get(endpoint)
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

func apiPost(endpoint string, payload any, out any) error {
	var body []byte
	var res *http.Response

	var err error

	body, err = json.Marshal(payload)
	if err != nil {
		return err
	}

	res, err = http.Post(endpoint, "application/json", bytes.NewReader(body))
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

func pickAgent(sh *state, base string) (*core.Agent, error) {
	var list []*core.Agent
	var item *core.Agent

	var err error

	err = apiGet(base+"/agents", &list)
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

	err = apiGet(base+"/sessions/"+sh.seed, &session)
	if err != nil {
		return nil, err
	}

	err = apiGet(base+"/agents/"+session.AgentId, &agent)
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

	err = apiPost(base+"/sessions", map[string]string{"agent_id": agent.Id, "name": "shell"}, &session)
	if err != nil {
		return "", err
	}

	sh.name = agent.Name

	return session.Id, nil
}

func connect(sh *state) error {
	var dialer websocket.Dialer
	var conn *websocket.Conn
	var session string

	var err error

	session, err = openSession(sh)
	if err != nil {
		return err
	}

	dialer = websocket.Dialer{HandshakeTimeout: DIAL_TIMEOUT}

	conn, _, err = dialer.Dial(sh.url, nil)
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

func sendAgent(sh *state, content string) error {
	var stop func()
	var reply reply
	var text string
	var thinking bool
	var streaming bool

	var err error

	err = sh.conn.WriteJSON(frame{SessionId: sh.session, Content: content})
	if err != nil {
		return err
	}

	stop = spinner("thinking…")

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
	}

	sh.mode = MODE_AGENT
}
