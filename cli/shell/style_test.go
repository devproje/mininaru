// SPDX-FileCopyrightText: 2026 Wonhyeok Kim (Project_IO)
// SPDX-License-Identifier: GPL-3.0-or-later

package shell

import (
	"strings"
	"testing"

	"github.com/gorilla/websocket"
)

func TestBannerShowsTheConnectedAgentName(t *testing.T) {
	var sh state
	var output string

	sh = state{conn: &websocket.Conn{}, name: "naru", url: "ws://127.0.0.1:8223/ws", session: "s1"}

	output = captureStdout(t, func() {
		banner(&sh)
	})

	if !strings.Contains(output, "agent naru") {
		t.Fatalf("banner output = %q, want it to mention the connected agent", output)
	}
}
