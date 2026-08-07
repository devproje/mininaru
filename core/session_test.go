package core

import (
	"path/filepath"
	"testing"

	"github.com/devproje/mininaru/util"
)

func TestSessionLatestSkipsEmptySessions(t *testing.T) {
	var agent *NaruAgent
	var older, newer, empty *Session
	var got *Session

	var err error

	util.DB, err = util.InitDatabase(filepath.Join(t.TempDir(), "t.db"))
	if err != nil {
		t.Fatal(err)
	}

	agent = &NaruAgent{Id: "agent-1", Name: "naru"}

	older, err = SessionCreate(agent, "older")
	if err != nil {
		t.Fatal(err)
	}
	_, err = MessageSave(older.Id, "user", "first", "")
	if err != nil {
		t.Fatal(err)
	}

	newer, err = SessionCreate(agent, "newer")
	if err != nil {
		t.Fatal(err)
	}
	_, err = MessageSave(newer.Id, "user", "second", "")
	if err != nil {
		t.Fatal(err)
	}

	empty, err = SessionCreate(agent, "empty")
	if err != nil {
		t.Fatal(err)
	}

	got, err = SessionLatest(agent.Id)
	if err != nil {
		t.Fatal(err)
	}
	if got == nil {
		t.Fatal("expected a session, got nil")
	}
	if got.Id != newer.Id {
		t.Fatalf("resolved %q, want %q; empty session %q must be skipped", got.Name, newer.Name, empty.Name)
	}
}

func TestSessionLatestWithoutHistory(t *testing.T) {
	var got *Session

	var err error

	util.DB, err = util.InitDatabase(filepath.Join(t.TempDir(), "t.db"))
	if err != nil {
		t.Fatal(err)
	}

	got, err = SessionLatest("no-such-agent")
	if err != nil {
		t.Fatal(err)
	}
	if got != nil {
		t.Fatalf("expected nil so the caller starts a new session, got %v", got)
	}
}

func TestSessionUpdateRenamesSession(t *testing.T) {
	var session, got *Session
	var err error

	util.DB, err = util.InitDatabase(filepath.Join(t.TempDir(), "t.db"))
	if err != nil {
		t.Fatal(err)
	}

	session, err = SessionCreate(&NaruAgent{Id: "agent-1"}, "old")
	if err != nil {
		t.Fatal(err)
	}
	err = SessionUpdate(session.Id, "new")
	if err != nil {
		t.Fatal(err)
	}
	got, err = SessionFind(session.Id)
	if err != nil {
		t.Fatal(err)
	}
	if got.Name != "new" {
		t.Fatalf("session name = %q, want new", got.Name)
	}
}
