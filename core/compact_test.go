// SPDX-FileCopyrightText: 2026 Wonhyeok Kim (Project_IO)
// SPDX-License-Identifier: GPL-3.0-or-later

package core

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/devproje/mininaru/config"
	"github.com/devproje/mininaru/util"
)

func compactSetup(t *testing.T, srvURL string, maxChars int, compact bool) (*Session, *NaruAgent) {
	var previous config.ClientConfig
	var session *Session
	var agent *NaruAgent

	t.Helper()

	previous = config.Client
	t.Cleanup(func() { config.Client = previous })

	session, agent = thinkingSetup(t, srvURL)

	config.Client.Context.MaxChars = maxChars
	config.Client.Context.Compact = compact

	return session, agent
}

func seedTurns(t *testing.T, sessionId string, turns int) {
	var err error

	t.Helper()

	for range turns {
		_, err = MessageSave(sessionId, "user", strings.Repeat("q", 64), "")
		if err != nil {
			t.Fatal(err)
		}

		_, err = MessageSave(sessionId, "assistant", strings.Repeat("a", 64), "")
		if err != nil {
			t.Fatal(err)
		}
	}
}

func compactServer(t *testing.T, requests *[]string) *httptest.Server {
	t.Helper()

	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body []byte

		body, _ = io.ReadAll(r.Body)
		*requests = append(*requests, string(body))
		w.Header().Set("Content-Type", "text/event-stream")

		if strings.Contains(string(body), "turns-to-fold-in") {
			io.WriteString(w, toolChunk("s", `{"role":"assistant","content":"the user likes tea"}`, `"stop"`))
		} else {
			io.WriteString(w, toolChunk("c", `{"role":"assistant","content":"answer"}`, `"stop"`))
		}

		io.WriteString(w, "data: [DONE]\n\n")
	}))
}

func summaryRow(t *testing.T, sessionId string) *Summary {
	var found *Summary

	var err error

	t.Helper()

	found, err = SummaryLoad(sessionId)
	if err != nil {
		t.Fatal(err)
	}

	return found
}

func TestCompactionStaysOutOfTheWayInsideTheBudget(t *testing.T) {
	var requests []string
	var srv *httptest.Server
	var session *Session
	var agent *NaruAgent

	var err error

	srv = compactServer(t, &requests)
	defer srv.Close()

	session, agent = compactSetup(t, srv.URL, 100000, true)
	seedTurns(t, session.Id, 2)

	_, err = ChatWithTools(context.Background(), session, agent, "hello", nil, nil, nil)
	if err != nil {
		t.Fatal(err)
	}

	if len(requests) != 1 {
		t.Fatalf("request count = %d, want 1 with nothing to compact", len(requests))
	}
	if summaryRow(t, session.Id) != nil {
		t.Fatal("a summary was written for a conversation that fits the budget")
	}
}

func TestCompactionSummarisesTheTurnsThatFallOut(t *testing.T) {
	var requests []string
	var srv *httptest.Server
	var session *Session
	var agent *NaruAgent
	var saved *Summary

	var err error

	srv = compactServer(t, &requests)
	defer srv.Close()

	session, agent = compactSetup(t, srv.URL, 100, true)
	seedTurns(t, session.Id, 2)

	_, err = ChatWithTools(context.Background(), session, agent, "hello", nil, nil, nil)
	if err != nil {
		t.Fatal(err)
	}

	if len(requests) != 2 {
		t.Fatalf("request count = %d, want a summary call and the chat call", len(requests))
	}
	if !strings.Contains(requests[0], "turns-to-fold-in") {
		t.Fatalf("the first call was not the summary: %s", requests[0])
	}

	saved = summaryRow(t, session.Id)
	if saved == nil {
		t.Fatal("no summary was saved")
	}
	if saved.Content != "the user likes tea" {
		t.Fatalf("saved summary = %q", saved.Content)
	}

	if !strings.Contains(requests[1], "mininaru-summary") || !strings.Contains(requests[1], "the user likes tea") {
		t.Fatalf("the chat call did not carry the summary: %s", requests[1])
	}
	if strings.Contains(requests[1], strings.Repeat("q", 64)) {
		t.Fatalf("the chat call still replayed a compacted turn: %s", requests[1])
	}
}

func TestCompactionFoldsThePreviousSummaryIn(t *testing.T) {
	var requests []string
	var srv *httptest.Server
	var session *Session
	var agent *NaruAgent
	var first *Summary

	var err error

	srv = compactServer(t, &requests)
	defer srv.Close()

	session, agent = compactSetup(t, srv.URL, 100, true)
	seedTurns(t, session.Id, 2)

	_, err = ChatWithTools(context.Background(), session, agent, "one", nil, nil, nil)
	if err != nil {
		t.Fatal(err)
	}

	first = summaryRow(t, session.Id)
	if first == nil {
		t.Fatal("the first turn saved no summary")
	}

	requests = nil

	_, err = ChatWithTools(context.Background(), session, agent, "two", nil, nil, nil)
	if err != nil {
		t.Fatal(err)
	}

	if len(requests) != 2 {
		t.Fatalf("second turn made %d requests, want a summary call and the chat call", len(requests))
	}
	if !strings.Contains(requests[0], "summary-so-far") || !strings.Contains(requests[0], first.Content) {
		t.Fatalf("the second summary call did not fold the first one in: %s", requests[0])
	}
}

func TestCompactionKeepsOnlyTheTurnsAfterTheSummarisedOne(t *testing.T) {
	var requests []string
	var srv *httptest.Server
	var session *Session
	var agent *NaruAgent
	var saved *Summary
	var history []*Message

	var err error

	srv = compactServer(t, &requests)
	defer srv.Close()

	session, agent = compactSetup(t, srv.URL, 100, true)
	seedTurns(t, session.Id, 2)

	_, err = ChatWithTools(context.Background(), session, agent, "one", nil, nil, nil)
	if err != nil {
		t.Fatal(err)
	}

	saved = summaryRow(t, session.Id)
	history, err = MessageList(session.Id)
	if err != nil {
		t.Fatal(err)
	}

	if len(summaryTail(history, saved.ThroughMessageId)) != len(history)-4 {
		t.Fatalf("summaryTail kept %d of %d messages, want the four seeded ones gone",
			len(summaryTail(history, saved.ThroughMessageId)), len(history))
	}
}

func TestCompactionFallsBackToDroppingWhenTheSummaryFails(t *testing.T) {
	var requests []string
	var srv *httptest.Server
	var session *Session
	var agent *NaruAgent
	var message *Message

	var err error

	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body []byte

		body, _ = io.ReadAll(r.Body)
		requests = append(requests, string(body))

		if strings.Contains(string(body), "turns-to-fold-in") {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "text/event-stream")
		io.WriteString(w, toolChunk("c", `{"role":"assistant","content":"answer"}`, `"stop"`))
		io.WriteString(w, "data: [DONE]\n\n")
	}))
	defer srv.Close()

	session, agent = compactSetup(t, srv.URL, 100, true)
	seedTurns(t, session.Id, 2)

	message, err = ChatWithTools(context.Background(), session, agent, "hello", nil, nil, nil)
	if err != nil {
		t.Fatalf("a failed summary broke the turn: %v", err)
	}
	if message.Content != "answer" {
		t.Fatalf("answer = %q", message.Content)
	}
	if summaryRow(t, session.Id) != nil {
		t.Fatal("a summary was saved even though the model call failed")
	}
}

func TestCompactionOffDropsWithoutCallingTheModel(t *testing.T) {
	var requests []string
	var srv *httptest.Server
	var session *Session
	var agent *NaruAgent

	var err error

	srv = compactServer(t, &requests)
	defer srv.Close()

	session, agent = compactSetup(t, srv.URL, 100, false)
	seedTurns(t, session.Id, 2)

	_, err = ChatWithTools(context.Background(), session, agent, "hello", nil, nil, nil)
	if err != nil {
		t.Fatal(err)
	}

	if len(requests) != 1 {
		t.Fatalf("request count = %d, want only the chat call with compaction off", len(requests))
	}
	if summaryRow(t, session.Id) != nil {
		t.Fatal("compaction was off but a summary was written")
	}
}

func TestSummaryTailReplaysEverythingWhenTheMarkerIsGone(t *testing.T) {
	var history []*Message

	history = []*Message{{Id: "a"}, {Id: "b"}}

	if len(summaryTail(history, "missing")) != 2 {
		t.Fatal("summaryTail dropped turns for a marker that is not in the history")
	}
	if len(summaryTail(history, "a")) != 1 {
		t.Fatal("summaryTail did not cut at the marker")
	}
}

func TestSummaryIsRemovedWithItsSession(t *testing.T) {
	var session *Session
	var agent *NaruAgent
	var count int

	var err error

	session, agent = compactSetup(t, "http://127.0.0.1:1", 100, true)
	_ = agent

	err = SummarySave(session.Id, "kept for now", "m1")
	if err != nil {
		t.Fatal(err)
	}

	err = SessionDelete(session.Id)
	if err != nil {
		t.Fatal(err)
	}

	err = util.DB.QueryRow("SELECT COUNT(*) FROM session_summaries WHERE session_id = ?;", session.Id).Scan(&count)
	if err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatalf("summary rows left after the session was deleted = %d", count)
	}
}

func TestCompactNowFoldsTheWholeConversation(t *testing.T) {
	var requests []string
	var srv *httptest.Server
	var session *Session
	var agent *NaruAgent
	var compacted bool
	var saved *Summary
	var history []*Message
	var before int
	var after int

	var err error

	srv = compactServer(t, &requests)
	defer srv.Close()

	session, agent = compactSetup(t, srv.URL, 100000, true)
	seedTurns(t, session.Id, 2)
	before, err = SessionContextChars(session.Id)
	if err != nil {
		t.Fatal(err)
	}

	compacted, err = CompactNow(context.Background(), agent, session)
	if err != nil {
		t.Fatal(err)
	}
	if !compacted {
		t.Fatal("CompactNow reported nothing to do for a seeded conversation")
	}

	if len(requests) != 1 || !strings.Contains(requests[0], "turns-to-fold-in") {
		t.Fatalf("requests = %#v, want one summary call", requests)
	}

	history, err = MessageList(session.Id)
	if err != nil {
		t.Fatal(err)
	}

	saved = summaryRow(t, session.Id)
	if saved == nil {
		t.Fatal("no summary was saved")
	}
	if saved.ThroughMessageId != history[len(history)-1].Id {
		t.Fatal("the summary does not reach the newest message")
	}
	if len(summaryTail(history, saved.ThroughMessageId)) != 0 {
		t.Fatal("turns were left outside the summary")
	}
	after, err = SessionContextChars(session.Id)
	if err != nil {
		t.Fatal(err)
	}
	if after != len(saved.Content) || after >= before {
		t.Fatalf("context chars = %d → %d, summary has %d", before, after, len(saved.Content))
	}
}

func TestCompactNowDoesNothingOnAnEmptyConversation(t *testing.T) {
	var requests []string
	var srv *httptest.Server
	var session *Session
	var agent *NaruAgent
	var compacted bool

	var err error

	srv = compactServer(t, &requests)
	defer srv.Close()

	session, agent = compactSetup(t, srv.URL, 100000, true)

	compacted, err = CompactNow(context.Background(), agent, session)
	if err != nil {
		t.Fatal(err)
	}
	if compacted {
		t.Fatal("CompactNow compacted an empty conversation")
	}
	if len(requests) != 0 {
		t.Fatalf("CompactNow called the model %d times with nothing to fold", len(requests))
	}
}

func TestCompactNowIgnoresTheAutomaticToggle(t *testing.T) {
	var requests []string
	var srv *httptest.Server
	var session *Session
	var agent *NaruAgent
	var compacted bool

	var err error

	srv = compactServer(t, &requests)
	defer srv.Close()

	session, agent = compactSetup(t, srv.URL, 100000, false)
	seedTurns(t, session.Id, 1)

	compacted, err = CompactNow(context.Background(), agent, session)
	if err != nil {
		t.Fatal(err)
	}
	if !compacted {
		t.Fatal("an explicit compact request was refused because automatic compaction is off")
	}
	if summaryRow(t, session.Id) == nil {
		t.Fatal("no summary was saved with automatic compaction off")
	}
}

func TestCompactNowFoldsAnExistingSummaryIn(t *testing.T) {
	var requests []string
	var srv *httptest.Server
	var session *Session
	var agent *NaruAgent

	var err error

	srv = compactServer(t, &requests)
	defer srv.Close()

	session, agent = compactSetup(t, srv.URL, 100000, true)
	seedTurns(t, session.Id, 1)

	_, err = CompactNow(context.Background(), agent, session)
	if err != nil {
		t.Fatal(err)
	}

	seedTurns(t, session.Id, 1)
	requests = nil

	_, err = CompactNow(context.Background(), agent, session)
	if err != nil {
		t.Fatal(err)
	}

	if len(requests) != 1 {
		t.Fatalf("second compact made %d requests, want 1", len(requests))
	}
	if !strings.Contains(requests[0], "summary-so-far") {
		t.Fatalf("the second compact did not fold the first summary in: %s", requests[0])
	}
}

func TestCompactNowReturnsTheFailureInsteadOfSwallowingIt(t *testing.T) {
	var srv *httptest.Server
	var session *Session
	var agent *NaruAgent

	var err error

	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	session, agent = compactSetup(t, srv.URL, 100000, true)
	seedTurns(t, session.Id, 1)

	_, err = CompactNow(context.Background(), agent, session)
	if err == nil {
		t.Fatal("CompactNow swallowed a failed summary call")
	}
	if summaryRow(t, session.Id) != nil {
		t.Fatal("a summary was saved even though the call failed")
	}
}
