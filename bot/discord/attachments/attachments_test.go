// SPDX-FileCopyrightText: 2026 Wonhyeok Kim (Project_IO)
// SPDX-License-Identifier: GPL-3.0-or-later

package attachments

import "testing"

func TestAllowedURLOnlyAcceptsDiscordCDN(t *testing.T) {
	if !allowedURL("https://cdn.discordapp.com/attachments/1/2/file.png") {
		t.Fatal("Discord CDN URL was rejected")
	}
	if allowedURL("https://example.com/file.png") || allowedURL("http://cdn.discordapp.com/file.png") {
		t.Fatal("non-Discord or insecure URL was accepted")
	}
}

func TestAttachmentTypes(t *testing.T) {
	if !imageType("image/png") || imageType("image/svg+xml") {
		t.Fatal("image allowlist is incorrect")
	}
	if !textType("application/octet-stream", "main.go") || textType("application/octet-stream", "app.exe") {
		t.Fatal("text extension allowlist is incorrect")
	}
}
