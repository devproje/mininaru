// SPDX-FileCopyrightText: 2026 Wonhyeok Kim (Project_IO)
// SPDX-License-Identifier: GPL-3.0-or-later

package core

import (
	"github.com/openai/openai-go"
)

var mirrorChunk func(sessionId string, chunk openai.ChatCompletionChunk)

var mirrorTool func(sessionId, name, status, message string)

var liveSessionIds func() []string

func SetSessionRouter(chunk func(sessionId string, chunk openai.ChatCompletionChunk), tool func(sessionId, name, status, message string)) {
	mirrorChunk = chunk
	mirrorTool = tool
}

func SetLiveSessionsLister(fn func() []string) {
	liveSessionIds = fn
}
