// SPDX-FileCopyrightText: 2026 Wonhyeok Kim (Project_IO)
// SPDX-License-Identifier: GPL-3.0-or-later

package client

import (
	"testing"

	"github.com/devproje/mininaru/core"
)

func TestContextLabel(t *testing.T) {
	var usage core.ContextUsage

	usage = core.ContextUsage{Used: 4800, Limit: 19200}
	if contextLabel(&usage) != "ctx:4800/19200 (25%)" {
		t.Fatalf("contextLabel() = %q", contextLabel(&usage))
	}
}
