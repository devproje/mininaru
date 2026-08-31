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
	var path string
	var id string
	var images []string
	var conn *websocket.Conn

	var err error

	prompt = strings.TrimSpace(prompt)
	if prompt == "" {
		return fmt.Errorf("prompt is empty")
	}

	if !client.ValidFormat(promptFormatRef) {
		return fmt.Errorf("unknown --format %q (want string|json|xml)", promptFormatRef)
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

	for _, path = range promptImageRef {
		id, err = client.Upload(base, apiKey, session.Id, path)
		if err != nil {
			return fmt.Errorf("uploading %s: %w", path, err)
		}

		images = append(images, id)
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

	err = conn.WriteJSON(client.Frame{SessionId: session.Id, Content: prompt, Cwd: cwd, Images: images})
	if err != nil {
		return err
	}

	return client.Receive(conn, client.Pump(conn), session.Id, nil, promptFormatRef)
}
