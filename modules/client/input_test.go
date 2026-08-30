// SPDX-FileCopyrightText: 2026 Wonhyeok Kim (Project_IO)
// SPDX-License-Identifier: GPL-3.0-or-later

package client

import (
	"testing"
)

func TestCsiUCode(t *testing.T) {
	var code int
	var mods int

	code, mods = csiUCode("13;2")
	if code != 13 || mods != 2 {
		t.Fatalf("shift+enter: got %d;%d", code, mods)
	}

	code, mods = csiUCode("99;5")
	if code != 'c' || !modHas(mods, 0x04) {
		t.Fatalf("ctrl+c: got %d;%d ctrl=%v", code, mods, modHas(mods, 0x04))
	}

	code, mods = csiUCode("27")
	if code != 27 || mods != 1 {
		t.Fatalf("bare esc: got %d;%d", code, mods)
	}

	code, mods = csiUCode("97;1:2")
	if code != 97 || mods != 1 {
		t.Fatalf("event-type suffix: got %d;%d", code, mods)
	}
}
