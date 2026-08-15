// SPDX-FileCopyrightText: 2026 Wonhyeok Kim (Project_IO)
// SPDX-License-Identifier: GPL-3.0-or-later

package core

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/devproje/mininaru/config"
	"github.com/devproje/mininaru/modules"
	"github.com/devproje/mininaru/util"
	"github.com/openai/openai-go"
)

func usageOf(prompt, completion int64) openai.CompletionUsage {
	return openai.CompletionUsage{
		PromptTokens: prompt, CompletionTokens: completion, TotalTokens: prompt + completion,
	}
}

func usageChunk(id string, prompt, completion int) string {
	return `data: {"id":"` + id + `","object":"chat.completion.chunk","created":1,"model":"m","choices":[],` +
		`"usage":{"prompt_tokens":` + strconv.Itoa(prompt) + `,"completion_tokens":` + strconv.Itoa(completion) +
		`,"total_tokens":` + strconv.Itoa(prompt+completion) + `}}` + "\n\n"
}

func usageRows(t *testing.T, sessionId, kind string) int {
	var count int

	var err error

	t.Helper()

	err = util.DB.QueryRow("SELECT COUNT(*) FROM token_usage WHERE session_id = ? AND kind = ?;",
		sessionId, kind).Scan(&count)
	if err != nil {
		t.Fatal(err)
	}

	return count
}

func TestTurnUsageIsRecordedAndSummedAcrossRounds(t *testing.T) {
	var requests int
	var srv *httptest.Server
	var session *Session
	var agent *NaruAgent
	var def modules.Def
	var totals *UsageTotals

	var err error

	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		w.Header().Set("Content-Type", "text/event-stream")

		if requests == 1 {
			io.WriteString(w, toolChunk("r1",
				`{"role":"assistant","tool_calls":[{"index":0,"id":"c1","type":"function","function":{"name":"echo","arguments":"{}"}}]}`,
				`"tool_calls"`))
			io.WriteString(w, usageChunk("r1", 100, 10))
		} else {
			io.WriteString(w, toolChunk("r2", `{"role":"assistant","content":"done"}`, `"stop"`))
			io.WriteString(w, usageChunk("r2", 200, 20))
		}

		io.WriteString(w, "data: [DONE]\n\n")
	}))
	defer srv.Close()

	session, agent = compactSetup(t, srv.URL, 100000, true)

	def = modules.Def{
		Name: "echo", Description: "echo", Permission: modules.PermissionSafe,
		Parameters: map[string]any{"type": "object"},
		Execute:    func(context.Context, string) (string, error) { return "echoed", nil },
	}

	_, err = ChatWithTools(context.Background(), session, agent, "hi", []modules.Def{def}, nil, nil)
	if err != nil {
		t.Fatal(err)
	}

	if requests != 2 {
		t.Fatalf("request count = %d, want two rounds", requests)
	}

	totals, err = SessionUsage(session.Id)
	if err != nil {
		t.Fatal(err)
	}

	if totals.PromptTokens != 300 || totals.CompletionTokens != 30 || totals.TotalTokens != 330 {
		t.Fatalf("totals = %+v, want both rounds summed", totals)
	}
	if usageRows(t, session.Id, UsageTurn) != 1 {
		t.Fatal("the turn should be one row carrying the sum, not one per round")
	}
}

func TestCompactionUsageIsRecordedSeparately(t *testing.T) {
	var srv *httptest.Server
	var session *Session
	var agent *NaruAgent
	var totals *UsageTotals

	var err error

	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body []byte

		body, _ = io.ReadAll(r.Body)
		w.Header().Set("Content-Type", "text/event-stream")

		if strings.Contains(string(body), "turns-to-fold-in") {
			io.WriteString(w, toolChunk("s", `{"role":"assistant","content":"a summary"}`, `"stop"`))
			io.WriteString(w, usageChunk("s", 50, 5))
		} else {
			io.WriteString(w, toolChunk("c", `{"role":"assistant","content":"answer"}`, `"stop"`))
			io.WriteString(w, usageChunk("c", 70, 7))
		}

		io.WriteString(w, "data: [DONE]\n\n")
	}))
	defer srv.Close()

	session, agent = compactSetup(t, srv.URL, 100, true)
	seedTurns(t, session.Id, 2)

	_, err = ChatWithTools(context.Background(), session, agent, "hi", nil, nil, nil)
	if err != nil {
		t.Fatal(err)
	}

	if usageRows(t, session.Id, UsageCompaction) != 1 {
		t.Fatal("compaction usage was not recorded")
	}
	if usageRows(t, session.Id, UsageTurn) != 1 {
		t.Fatal("turn usage was not recorded")
	}

	totals, err = SessionUsage(session.Id)
	if err != nil {
		t.Fatal(err)
	}
	if totals.TotalTokens != 55+77 {
		t.Fatalf("total = %d, want the turn and the compaction added up", totals.TotalTokens)
	}
	if len(totals.Lines) != 2 {
		t.Fatalf("lines = %+v, want one per kind", totals.Lines)
	}
}

func TestSubagentUsageIsRecordedAgainstTheCallingSession(t *testing.T) {
	var requests int
	var srv *httptest.Server
	var session *Session
	var parent *NaruAgent

	var err error

	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		w.Header().Set("Content-Type", "text/event-stream")

		switch requests {
		case 1:
			io.WriteString(w, toolChunk("r1", delegationCall("worker", "do it"), `"tool_calls"`))
			io.WriteString(w, usageChunk("r1", 10, 1))
		case 2:
			io.WriteString(w, toolChunk("r2", `{"role":"assistant","content":"worker done"}`, `"stop"`))
			io.WriteString(w, usageChunk("r2", 40, 4))
		default:
			io.WriteString(w, toolChunk("r3", `{"role":"assistant","content":"final"}`, `"stop"`))
			io.WriteString(w, usageChunk("r3", 20, 2))
		}

		io.WriteString(w, "data: [DONE]\n\n")
	}))
	defer srv.Close()

	session, parent, _ = subagentSetup(t, srv.URL)

	config.Client.Context.MaxChars = 100000

	_, err = ChatWithTools(context.Background(), session, parent, "go",
		[]modules.Def{AgentCallTool()}, nil, nil)
	if err != nil {
		t.Fatal(err)
	}

	if usageRows(t, session.Id, UsageSubagent) != 1 {
		t.Fatal("the delegated turn was not billed to the calling session")
	}
	if usageRows(t, session.Id, UsageTurn) != 1 {
		t.Fatal("the parent turn was not recorded")
	}
}

func TestUsageIsNotRecordedWithoutASessionOrTokens(t *testing.T) {
	var count int

	var err error

	_, _ = compactSetup(t, "http://127.0.0.1:1", 100000, true)

	usageRecord("", "m1", UsageTurn, usageOf(10, 1))
	usageRecord("s-nonexistent", "m1", UsageTurn, usageOf(0, 0))

	err = util.DB.QueryRow("SELECT COUNT(*) FROM token_usage;").Scan(&count)
	if err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatalf("token_usage rows = %d, want none for a missing session or empty usage", count)
	}
}

func TestUsageFailureDoesNotBreakTheTurn(t *testing.T) {
	var srv *httptest.Server
	var session *Session
	var agent *NaruAgent
	var message *Message

	var err error

	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		io.WriteString(w, toolChunk("c", `{"role":"assistant","content":"answer"}`, `"stop"`))
		io.WriteString(w, usageChunk("c", 10, 1))
		io.WriteString(w, "data: [DONE]\n\n")
	}))
	defer srv.Close()

	session, agent = compactSetup(t, srv.URL, 100000, true)

	_, err = util.DB.Exec("DROP TABLE token_usage;")
	if err != nil {
		t.Fatal(err)
	}

	message, err = ChatWithTools(context.Background(), session, agent, "hi", nil, nil, nil)
	if err != nil {
		t.Fatalf("a broken usage table failed the turn: %v", err)
	}
	if message.Content != "answer" {
		t.Fatalf("answer = %q", message.Content)
	}
}

func TestSessionUsageIsRemovedWithItsSession(t *testing.T) {
	var session *Session
	var count int

	var err error

	session, _ = compactSetup(t, "http://127.0.0.1:1", 100000, true)

	usageRecord(session.Id, "m1", UsageTurn, usageOf(10, 1))

	err = SessionDelete(session.Id)
	if err != nil {
		t.Fatal(err)
	}

	err = util.DB.QueryRow("SELECT COUNT(*) FROM token_usage WHERE session_id = ?;", session.Id).Scan(&count)
	if err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatalf("usage rows left after the session was deleted = %d", count)
	}
}

func TestSessionUsageAllGroupsBySession(t *testing.T) {
	var first *Session
	var agent *NaruAgent
	var second *Session
	var totals map[string]int64

	var err error

	first, agent = compactSetup(t, "http://127.0.0.1:1", 100000, true)

	second, err = SessionCreate(agent, "second")
	if err != nil {
		t.Fatal(err)
	}

	usageRecord(first.Id, "", UsageTurn, usageOf(10, 1))
	usageRecord(first.Id, "", UsageCompaction, usageOf(20, 2))
	usageRecord(second.Id, "", UsageTurn, usageOf(30, 3))

	totals, err = SessionUsageAll(agent.Id)
	if err != nil {
		t.Fatal(err)
	}

	if totals[first.Id] != 33 {
		t.Fatalf("first session total = %d, want both kinds added", totals[first.Id])
	}
	if totals[second.Id] != 33 {
		t.Fatalf("second session total = %d", totals[second.Id])
	}
}
