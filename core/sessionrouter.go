// SPDX-FileCopyrightText: 2026 Wonhyeok Kim (Project_IO)
// SPDX-License-Identifier: GPL-3.0-or-later

package core

import (
	"github.com/openai/openai-go"
)

var mirrorMessage func(sessionId, origin, content string)

var mirrorChunk func(sessionId string, chunk openai.ChatCompletionChunk)

var mirrorTool func(sessionId, name, status, message string)

var mirrorDone func(sessionId, failure string)

var liveSessionIds func() []string

func SetSessionRouter(
	message func(sessionId, origin, content string),
	chunk func(sessionId string, chunk openai.ChatCompletionChunk),
	tool func(sessionId, name, status, message string),
	done func(sessionId, failure string),
) {
	mirrorMessage = message
	mirrorChunk = chunk
	mirrorTool = tool
	mirrorDone = done
}

func SetLiveSessionsLister(fn func() []string) {
	liveSessionIds = fn
}
