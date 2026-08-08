package core

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/devproje/mininaru/config"
	"github.com/devproje/mininaru/util"
)

func chunk(delta string) string {
	return fmt.Sprintf(`data: {"id":"1","object":"chat.completion.chunk","created":1,"model":"m","choices":[{"index":0,"delta":%s}]}`+"\n\n", delta)
}

func thinkingServer(t *testing.T, body *string, field string) *httptest.Server {
	var srv *httptest.Server

	t.Helper()

	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var raw []byte

		raw, _ = io.ReadAll(r.Body)
		*body = string(raw)

		w.Header().Set("Content-Type", "text/event-stream")
		io.WriteString(w, chunk(fmt.Sprintf(`{"role":"assistant","%s":"let me "}`, field)))
		io.WriteString(w, chunk(fmt.Sprintf(`{"%s":"consider it"}`, field)))
		io.WriteString(w, chunk(`{"content":"the answer"}`))
		io.WriteString(w, "data: [DONE]\n\n")
	}))
	t.Cleanup(srv.Close)

	return srv
}

func thinkingSetup(t *testing.T, srvURL string) (*Session, *NaruAgent) {
	var prov *Provider
	var agent *NaruAgent
	var session *Session

	var err error

	t.Helper()

	err = util.InitFS(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}

	util.DB, err = util.InitDatabase(filepath.Join(t.TempDir(), "t.db"))
	if err != nil {
		t.Fatal(err)
	}

	Providers = nil
	DefaultProvider = nil
	ProviderCreate(Provider{Name: "p", BaseURL: srvURL, ApiKey: "k"})
	prov = Providers[0]

	agent = AgentNew("naru", "", "", "m", prov)
	session, err = SessionCreate(agent, "s")
	if err != nil {
		t.Fatal(err)
	}

	return session, agent
}

func runChat(t *testing.T, session *Session, agent *NaruAgent) (string, string) {
	var answer strings.Builder
	var thought strings.Builder

	var err error

	t.Helper()

	_, err = Chat(context.Background(), session, agent, "hi",
		func(d string) { answer.WriteString(d) },
		func(d string) { thought.WriteString(d) },
	)
	if err != nil {
		t.Fatal(err)
	}

	return thought.String(), answer.String()
}

func TestThinkingStreamSurfaced(t *testing.T) {
	var field string
	var session *Session
	var agent *NaruAgent
	var body, thought, answer string

	for _, field = range []string{"reasoning_content", "reasoning"} {
		t.Run(field, func(t *testing.T) {
			session, agent = thinkingSetup(t, thinkingServer(t, &body, field).URL)
			config.Client.Thinking = config.Thinking{Level: config.ThinkingHigh, Show: true}

			thought, answer = runChat(t, session, agent)

			if thought != "let me consider it" {
				t.Fatalf("reasoning from %q = %q, want the streamed thought", field, thought)
			}
			if answer != "the answer" {
				t.Fatalf("content = %q, want the answer", answer)
			}
		})
	}
}

func TestThinkingLevelSentAsReasoningEffort(t *testing.T) {
	var session *Session
	var agent *NaruAgent
	var body string
	var payload map[string]any

	session, agent = thinkingSetup(t, thinkingServer(t, &body, "reasoning_content").URL)
	config.Client.Thinking = config.Thinking{Level: config.ThinkingMax, Show: true}

	runChat(t, session, agent)

	json.Unmarshal([]byte(body), &payload)
	if payload["reasoning_effort"] != "max" {
		t.Fatalf("reasoning_effort = %v, want max; body=%s", payload["reasoning_effort"], body)
	}
}

func TestThinkingOffSendsNoReasoningEffort(t *testing.T) {
	var session *Session
	var agent *NaruAgent
	var body string
	var payload map[string]any
	var ok bool

	session, agent = thinkingSetup(t, thinkingServer(t, &body, "reasoning_content").URL)
	config.Client.Thinking = config.Thinking{Level: config.ThinkingOff, Show: true}

	runChat(t, session, agent)

	json.Unmarshal([]byte(body), &payload)
	_, ok = payload["reasoning_effort"]
	if ok {
		t.Fatalf("reasoning_effort must be absent when thinking is off; body=%s", body)
	}
}

func TestReasoningPersistedAndReplayed(t *testing.T) {
	var session *Session
	var agent *NaruAgent
	var body string
	var saved *Message
	var history []*Message

	var err error

	session, agent = thinkingSetup(t, thinkingServer(t, &body, "reasoning_content").URL)
	config.Client.Thinking = config.Thinking{Level: config.ThinkingHigh, Show: true}

	saved, err = Chat(context.Background(), session, agent, "hi", nil, nil)
	if err != nil {
		t.Fatal(err)
	}

	if saved.Reasoning != "let me consider it" {
		t.Fatalf("returned message reasoning = %q, want the streamed thought", saved.Reasoning)
	}

	history, err = MessageList(session.Id)
	if err != nil {
		t.Fatal(err)
	}

	if len(history) != 2 {
		t.Fatalf("history has %d messages, want user + assistant", len(history))
	}

	if history[0].Reasoning != "" {
		t.Fatalf("user message carries reasoning %q", history[0].Reasoning)
	}

	if history[1].Reasoning != "let me consider it" {
		t.Fatalf("reasoning did not survive the round trip: %q", history[1].Reasoning)
	}
}
