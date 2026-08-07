package core

import (
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/devproje/mininaru/config"
	"github.com/devproje/mininaru/util"
)

func TestChatFailureDoesNotPersistPartialTurn(t *testing.T) {
	var srv *httptest.Server
	var session *Session
	var agent *NaruAgent
	var messages []*Message
	var status, errorText string

	var err error

	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "failed", http.StatusBadGateway)
	}))
	defer srv.Close()

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
	ProviderCreate(Provider{Name: "broken", BaseURL: srv.URL, ApiKey: "k"})
	agent = AgentNew("naru", "", "", "m", Providers[0])
	session, err = SessionCreate(agent, "failed chat")
	if err != nil {
		t.Fatal(err)
	}
	config.Client.Thinking = config.Thinking{Level: config.ThinkingOff}

	_, err = Chat(context.Background(), session, agent, "do not keep me", nil, nil)
	if err == nil {
		t.Fatal("chat unexpectedly succeeded")
	}

	messages, err = MessageList(session.Id)
	if err != nil {
		t.Fatal(err)
	}
	if len(messages) != 0 {
		t.Fatalf("failed chat persisted %d messages, want none", len(messages))
	}

	err = util.DB.QueryRow("SELECT status, error FROM messages WHERE session_id = ?;", session.Id).Scan(&status, &errorText)
	if err != nil {
		t.Fatal(err)
	}
	if status != MessageFailed || errorText == "" {
		t.Fatalf("failure audit = status %q error %q", status, errorText)
	}
}
