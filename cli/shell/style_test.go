// SPDX-FileCopyrightText: 2026 Wonhyeok Kim (Project_IO)
// SPDX-License-Identifier: GPL-3.0-or-later

package shell

import (
	"strings"
	"testing"

	"github.com/devproje/mininaru/util"
	"github.com/gorilla/websocket"
)

func TestBannerShowsTheConnectedAgentName(t *testing.T) {
	var sh state
	var output string

	sh = state{conn: &websocket.Conn{}, name: "naru", url: "ws://127.0.0.1:8223/ws", session: "s1"}

	output = captureStdout(t, func() {
		banner(&sh)
	})

	if !strings.Contains(output, "agent") || !strings.Contains(output, "   naru") {
		t.Fatalf("banner output = %q, want it to mention the connected agent", output)
	}
}

func TestBannerShowsAnUpdateNoticeWhenACachedTagIsNewer(t *testing.T) {
	var sh state
	var output string
	var previousVersion string

	setupTestNaruPath(t)

	previousVersion = util.AppVersion
	util.AppVersion = "v1.0.0-alpha.1"
	t.Cleanup(func() { util.AppVersion = previousVersion })

	util.UpdateCacheWrite("v1.0.0-alpha.2")

	sh = state{name: "naru"}

	output = captureStdout(t, func() {
		banner(&sh)
	})

	if !strings.Contains(output, "v1.0.0-alpha.2") {
		t.Fatalf("banner output = %q, want it to mention the newer cached tag", output)
	}
}
