// SPDX-FileCopyrightText: 2026 Wonhyeok Kim (Project_IO)
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"fmt"
	"net/http"
	"os"
	"strings"

	"github.com/devproje/mininaru/core"
	"github.com/devproje/mininaru/modules/client"
	"github.com/gorilla/websocket"
)

func shortPrompt(prompt string) error {
	var base string
	var apiKey string
	var session *core.Session
	var cwd string
	var conn *websocket.Conn

	var err error

	prompt = strings.TrimSpace(prompt)
	if prompt == "" {
		return fmt.Errorf("prompt is empty")
	}

	apiKey = client.ResolveApiKey(promptApiKeyRef, promptUrlRef)

	base, err = client.ApiBase(promptUrlRef)
	if err != nil {
		return err
	}

	session, err = client.Session(base, apiKey, promptSessionRef, promptAgentRef)
	if err != nil {
		return err
	}

	if promptSessionRef == "" {
		defer client.Api(http.MethodDelete, base+"/sessions/"+session.Id, apiKey, nil, nil)
	}

	cwd, err = os.Getwd()
	if err != nil {
		return err
	}

	conn, err = client.Dial(promptUrlRef, apiKey)
	if err != nil {
		return err
	}
	defer conn.Close()

	err = conn.WriteJSON(client.Frame{Type: "attach", SessionId: session.Id})
	if err != nil {
		return err
	}

	err = conn.WriteJSON(client.Frame{SessionId: session.Id, Content: prompt, Cwd: cwd})
	if err != nil {
		return err
	}

	return client.Receive(conn, session.Id, nil)
}
