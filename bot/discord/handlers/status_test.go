// SPDX-FileCopyrightText: 2026 Wonhyeok Kim (Project_IO)
// SPDX-License-Identifier: GPL-3.0-or-later

package handlers

import (
	"encoding/json"
	"io"
	"net/http"
	"strconv"
	"strings"
	"testing"

	"github.com/bwmarrin/discordgo"
)

func recordingGateway(t *testing.T, sent *[]string) *discordgo.Session {
	var gateway *discordgo.Session

	var err error

	t.Helper()

	gateway, err = discordgo.New("Bot test")
	if err != nil {
		t.Fatal(err)
	}

	gateway.Client = &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		var body []byte

		var readErr error

		if request.Body != nil {
			body, readErr = io.ReadAll(request.Body)
			if readErr != nil {
				return nil, readErr
			}
			*sent = append(*sent, request.Method+" "+request.URL.Path+" "+string(body))
		}

		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     make(http.Header),
			Body:       io.NopCloser(strings.NewReader(`{"id":"status","channel_id":"channel"}`)),
			Request:    request,
		}, nil
	})}

	return gateway
}

func TestStatusStartsThinkingWithoutClaimingASkill(t *testing.T) {
	var status executionStatus
	var text string

	status = executionStatus{stateIcon: statusThinkingIcon, stateText: statusThinkingText}
	text = status.render()

	if !strings.Contains(text, statusThinkingText) {
		t.Fatalf("opening state = %q", text)
	}
	if strings.Contains(strings.ToLower(text), "skill") {
		t.Fatalf("the opening state still claims a skill: %q", text)
	}
}

func TestStatusKeepsHeaderAboveTheLog(t *testing.T) {
	var status executionStatus
	var text string
	var header int
	var first int
	var second int

	status = executionStatus{stateIcon: "🔧", stateText: "Running `web_search`"}
	status.steps = []statusStep{{icon: "✓", text: "`skill - pr-review`"}, {icon: "✗", text: "`bash_exec` — user denied"}}
	text = status.render()

	header = strings.Index(text, "Running `web_search`")
	first = strings.Index(text, "pr-review")
	second = strings.Index(text, "bash_exec")

	if header < 0 || first < 0 || second < 0 {
		t.Fatalf("rendered card = %q", text)
	}
	if !(header < first && first < second) {
		t.Fatalf("header and log are out of order: %q", text)
	}
	if !strings.HasPrefix(text, "### ") {
		t.Fatalf("the card does not open with a heading: %q", text)
	}
}

func TestStatusRecordsFailedToolsWithTheirReason(t *testing.T) {
	var status executionStatus
	var text string

	status = executionStatus{stateIcon: statusThinkingIcon, stateText: statusThinkingText}
	status.steps = []statusStep{{icon: "✗", text: "`bash_exec` — " + toolFailureReason("user denied dangerous tool \"bash_exec\"")}}
	text = status.render()

	if !strings.Contains(text, "✗") || !strings.Contains(text, "user denied") {
		t.Fatalf("failed step = %q", text)
	}
}

func TestToolFailureReasonCollapsesAndBounds(t *testing.T) {
	var reason string

	reason = toolFailureReason("  something\n went    wrong  ")
	if reason != "something went wrong" {
		t.Fatalf("reason = %q", reason)
	}

	if toolFailureReason("   ") != "failed" {
		t.Fatalf("empty reason = %q", toolFailureReason("   "))
	}

	reason = toolFailureReason(strings.Repeat("x", toolReasonLimit+50))
	if len([]rune(reason)) != toolReasonLimit+1 {
		t.Fatalf("reason was not bounded: %d runes", len([]rune(reason)))
	}
}

func TestStatusHidesOldStepsPastTheLimit(t *testing.T) {
	var status executionStatus
	var index int
	var text string

	status = executionStatus{stateIcon: statusThinkingIcon, stateText: statusThinkingText}
	for index = 0; index < maxStatusSteps+5; index++ {
		status.steps = append(status.steps, statusStep{icon: "✓", text: "step" + strconv.Itoa(index)})
	}
	text = status.render()

	if !strings.Contains(text, "5 earlier steps hidden") {
		t.Fatalf("hidden marker missing: %q", text)
	}
	if strings.Contains(text, "step0 ") || strings.Contains(text, "step4\n") {
		t.Fatalf("old steps were kept: %q", text)
	}
	if !strings.Contains(text, "step"+strconv.Itoa(maxStatusSteps+4)) {
		t.Fatalf("the newest step is missing: %q", text)
	}
}

func TestStatusStaysWithinTheMessageLimit(t *testing.T) {
	var status executionStatus
	var index int
	var text string

	status = executionStatus{stateIcon: statusThinkingIcon, stateText: statusThinkingText, note: strings.Repeat("n", 500)}
	for index = 0; index < maxStatusSteps; index++ {
		status.steps = append(status.steps, statusStep{icon: "✓", text: strings.Repeat("t", 300)})
	}
	text = status.render()

	if len([]rune(text)) > messageLimit {
		t.Fatalf("rendered card = %d runes, want at most %d", len([]rune(text)), messageLimit)
	}
}

func TestStatusOmitsTheNoteWhenThereIsNone(t *testing.T) {
	var status executionStatus

	status = executionStatus{stateIcon: statusThinkingIcon, stateText: statusThinkingText}

	if strings.Contains(status.render(), "-# ") {
		t.Fatalf("an empty note still rendered: %q", status.render())
	}
}

func TestStatusSkipsTheEditWhenNothingChanged(t *testing.T) {
	var sent []string
	var status executionStatus
	var before int

	status = executionStatus{
		gateway: recordingGateway(t, &sent), channelId: "channel", messageId: "status",
		stateIcon: statusThinkingIcon, stateText: statusThinkingText,
	}
	status.rendered = status.render()

	status.publish()
	before = len(sent)

	status.publish()
	if len(sent) != before {
		t.Fatalf("an unchanged card was edited again: %v", sent)
	}

	status.stateText = "Reasoning"
	status.publish()
	if len(sent) == before {
		t.Fatal("a changed card was not edited")
	}
}

func TestStatusFinishAddsTheFooterAndLeavesTheReplyAlone(t *testing.T) {
	var sent []string
	var status executionStatus
	var payload string

	status = executionStatus{
		gateway: recordingGateway(t, &sent), channelId: "channel", messageId: "status",
		stateIcon: statusThinkingIcon, stateText: statusThinkingText,
	}
	status.steps = []statusStep{{icon: "✓", text: "`skill`"}}
	status.rendered = status.render()

	status.finish("✅", "Answered")

	if len(sent) == 0 {
		t.Fatal("finish did not edit the card")
	}
	payload = strings.Join(sent, "\n")

	if !strings.Contains(payload, "Answered") {
		t.Fatalf("final state missing: %s", payload)
	}
	if !strings.Contains(payload, "1 tool") {
		t.Fatalf("footer missing the tool count: %s", payload)
	}
	if strings.Contains(payload, "Part 1/") {
		t.Fatalf("finish is still writing reply chunks into the card: %s", payload)
	}
}

func sentBody(t *testing.T, entry string) map[string]any {
	var start int
	var payload map[string]any

	var err error

	t.Helper()

	start = strings.Index(entry, "{")
	if start < 0 {
		t.Fatalf("no json body in %q", entry)
	}

	err = json.Unmarshal([]byte(entry[start:]), &payload)
	if err != nil {
		t.Fatal(err)
	}

	return payload
}

func TestReplyIsSentAsAPlainMessage(t *testing.T) {
	var sent []string
	var bot Discord
	var payload map[string]any
	var flags float64
	var ok bool

	bot = Discord{gateway: recordingGateway(t, &sent)}
	bot.sendReply("channel", "a plain answer")

	if len(sent) != 1 {
		t.Fatalf("requests = %v", sent)
	}
	payload = sentBody(t, sent[0])

	if payload["content"] != "a plain answer" {
		t.Fatalf("content = %v", payload["content"])
	}
	if payload["components"] != nil {
		t.Fatalf("the answer was wrapped in components: %v", payload["components"])
	}

	flags, ok = payload["flags"].(float64)
	if ok && int(flags)&int(discordgo.MessageFlagsIsComponentsV2) != 0 {
		t.Fatalf("the answer carries the components v2 flag: %v", payload["flags"])
	}
}

func TestStatusCardKeepsUsingComponents(t *testing.T) {
	var sent []string
	var status *executionStatus
	var payload map[string]any
	var flags float64

	status = newExecutionStatus(recordingGateway(t, &sent), "channel", "", "", threadStartedNote)

	if status == nil || len(sent) != 1 {
		t.Fatalf("requests = %v", sent)
	}
	payload = sentBody(t, sent[0])

	if payload["components"] == nil {
		t.Fatalf("the status card lost its container: %v", payload)
	}

	flags, _ = payload["flags"].(float64)
	if int(flags)&int(discordgo.MessageFlagsIsComponentsV2) == 0 {
		t.Fatalf("the status card is not a components v2 message: %v", payload["flags"])
	}
}
