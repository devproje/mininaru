// SPDX-FileCopyrightText: 2026 Wonhyeok Kim (Project_IO)
// SPDX-License-Identifier: GPL-3.0-or-later

package handlers

import (
	"io"
	"net/http"
	"strconv"
	"strings"
	"testing"

	"github.com/bwmarrin/discordgo"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return f(request)
}

func TestSplitMessageStaysUnderTheLimit(t *testing.T) {
	var chunks []string
	var chunk string

	chunks = splitMessage(strings.Repeat("a", 4500), messageLimit)
	if len(chunks) != 3 {
		t.Fatalf("chunks = %d, want 3", len(chunks))
	}

	for _, chunk = range chunks {
		if len([]rune(chunk)) > messageLimit {
			t.Fatalf("chunk of %d runes exceeds the limit", len([]rune(chunk)))
		}
	}

	if strings.Join(chunks, "") != strings.Repeat("a", 4500) {
		t.Fatal("splitting lost or duplicated content")
	}
}

func TestSplitMessagePrefersLineBreaks(t *testing.T) {
	var text string
	var chunks []string

	text = strings.Repeat("x", 1500) + "\n" + strings.Repeat("y", 1000)

	chunks = splitMessage(text, messageLimit)
	if len(chunks) != 2 {
		t.Fatalf("chunks = %d, want 2", len(chunks))
	}

	if chunks[0] != strings.Repeat("x", 1500) || chunks[1] != strings.Repeat("y", 1000) {
		t.Fatalf("split did not fall on the newline: %d / %d runes", len(chunks[0]), len(chunks[1]))
	}
}

func TestSplitMessageCountsRunesNotBytes(t *testing.T) {
	var chunks []string
	var chunk string

	chunks = splitMessage(strings.Repeat("가", 2500), messageLimit)

	for _, chunk = range chunks {
		if len([]rune(chunk)) > messageLimit {
			t.Fatalf("chunk of %d runes exceeds the limit", len([]rune(chunk)))
		}
	}

	if strings.Join(chunks, "") != strings.Repeat("가", 2500) {
		t.Fatal("splitting corrupted multibyte text")
	}
}

func TestSplitMessageKeepsShortTextIntact(t *testing.T) {
	var chunks []string

	chunks = splitMessage("hello", messageLimit)
	if len(chunks) != 1 || chunks[0] != "hello" {
		t.Fatalf("chunks = %#v", chunks)
	}

	chunks = splitMessage("", messageLimit)
	if len(chunks) != 1 || chunks[0] != "" {
		t.Fatalf("empty text = %#v", chunks)
	}
}

func TestExecutionStatusWithoutMessageReturnsWholeReply(t *testing.T) {
	var status executionStatus
	var chunks []string
	var text string

	text = strings.Repeat("a", 2500)
	status = executionStatus{}
	chunks = status.finish("✅", text, silentMentions())

	if len(chunks) != 2 || !strings.Contains(chunks[0], "Part 1/2") || !strings.Contains(chunks[1], "Part 2/2") {
		t.Fatalf("fallback chunks = %d, want complete reply", len(chunks))
	}
}

func TestExecutionStatusReplacesProgressWithFirstReplyChunk(t *testing.T) {
	var gateway *discordgo.Session
	var status executionStatus
	var chunks []string
	var text string
	var requestBody string

	var err error

	gateway, err = discordgo.New("Bot test")
	if err != nil {
		t.Fatal(err)
	}
	gateway.Client = &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		var body []byte

		var readErr error

		body, readErr = io.ReadAll(request.Body)
		if readErr != nil {
			return nil, readErr
		}
		requestBody = string(body)
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     make(http.Header),
			Body:       io.NopCloser(strings.NewReader(`{"id":"status","channel_id":"channel"}`)),
			Request:    request,
		}, nil
	})}

	text = strings.Repeat("a", 2500)
	status = executionStatus{gateway: gateway, channelId: "channel", messageId: "status", current: "Thinking"}
	chunks = status.finish("✅", text, silentMentions())

	if len(chunks) != 1 || !strings.Contains(chunks[0], strings.Repeat("a", 500)) || !strings.Contains(chunks[0], "Part 2/2") {
		t.Fatalf("continuation chunks = %#v", chunks)
	}
	if !strings.Contains(requestBody, strings.Repeat("a", messageLimit-replyChunkReserve)) || !strings.Contains(requestBody, "Part 1/2") {
		t.Fatal("status edit does not contain the first reply chunk")
	}
	if strings.Contains(requestBody, "Thinking") {
		t.Fatal("status edit still contains the progress text")
	}
}

func TestSplitReplyNumbersLongResponses(t *testing.T) {
	var chunks []string
	var chunk string
	var index int

	chunks = splitReply(strings.Repeat("paragraph text\n", 80), 240)
	if len(chunks) < 2 {
		t.Fatalf("chunks = %d, want multiple", len(chunks))
	}
	for index, chunk = range chunks {
		if len([]rune(chunk)) > 240 {
			t.Fatalf("chunk %d has %d runes", index, len([]rune(chunk)))
		}
		if !strings.Contains(chunk, "Part "+strconv.Itoa(index+1)+"/"+strconv.Itoa(len(chunks))) {
			t.Fatalf("chunk %d has no position label: %q", index, chunk)
		}
	}
}

func TestSplitReplyKeepsCodeBlocksRenderable(t *testing.T) {
	var text string
	var chunks []string
	var chunk string

	text = "Before\n```go\n" + strings.Repeat("fmt.Println(\"hello\")\n", 40) + "```\nAfter"
	chunks = splitReply(text, 240)
	if len(chunks) < 2 {
		t.Fatalf("chunks = %d, want multiple", len(chunks))
	}
	for _, chunk = range chunks {
		if len([]rune(chunk)) > 240 {
			t.Fatalf("chunk has %d runes", len([]rune(chunk)))
		}
		if strings.Count(chunk, "```")%2 != 0 {
			t.Fatalf("chunk has an unbalanced code fence: %q", chunk)
		}
	}
	if !strings.HasPrefix(chunks[1], "```go\n") {
		t.Fatalf("continued code block did not reopen with its language: %q", chunks[1])
	}
}

func TestSplitReplyLeavesShortResponsesUnlabelled(t *testing.T) {
	var chunks []string

	chunks = splitReply("short answer", messageLimit)
	if len(chunks) != 1 || chunks[0] != "short answer" {
		t.Fatalf("short reply = %#v", chunks)
	}
}
