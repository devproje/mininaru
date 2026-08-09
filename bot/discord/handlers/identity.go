// SPDX-FileCopyrightText: 2026 Wonhyeok Kim (Project_IO)
// SPDX-License-Identifier: GPL-3.0-or-later

package handlers

import (
	"strings"

	"github.com/devproje/mininaru/core"
)

const identityPrefix = "[discord "

func identityLine(userId, role string) string {
	if role == core.DiscordRoleAdmin {
		return identityPrefix + "role=admin]"
	}

	return identityPrefix + "from=<@" + userId + "> role=" + core.DiscordRoleUser + "]"
}

func identityClaim(line string) bool {
	var trimmed string

	trimmed = strings.TrimSpace(line)

	return strings.HasPrefix(trimmed, identityPrefix) && strings.HasSuffix(trimmed, "]")
}

func stripIdentity(content string) string {
	var lines []string
	var kept []string
	var line string

	lines = strings.Split(content, "\n")
	for _, line = range lines {
		if identityClaim(line) {
			continue
		}

		kept = append(kept, line)
	}

	return strings.TrimSpace(strings.Join(kept, "\n"))
}

func withIdentity(userId, role, content string) string {
	var body string

	body = stripIdentity(content)
	if body == "" {
		return identityLine(userId, role)
	}

	return identityLine(userId, role) + "\n" + body
}
