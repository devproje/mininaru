// SPDX-FileCopyrightText: 2026 Wonhyeok Kim (Project_IO)
// SPDX-License-Identifier: GPL-3.0-or-later

package util

import (
	"database/sql"
	"path/filepath"
	"testing"
)

func TestMigrationsCreateExpectedTables(t *testing.T) {
	var db *sql.DB
	var table string
	var count int

	var err error

	db, err = NewDatabase(filepath.Join(t.TempDir(), "schema.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	for _, table = range []string{"providers", "agents", "sessions", "messages"} {
		err = db.QueryRow("SELECT COUNT(*) FROM sqlite_master WHERE type = 'table' AND name = ?;", table).Scan(&count)
		if err != nil {
			t.Fatalf("checking table %q failed: %v", table, err)
		}
		if count != 1 {
			t.Fatalf("table %q was not created", table)
		}
	}
}

func TestMigrationsRecordVersion(t *testing.T) {
	var db *sql.DB
	var version string

	var err error

	db, err = NewDatabase(filepath.Join(t.TempDir(), "version.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	err = db.QueryRow("SELECT version FROM migrations WHERE version = '0001_initial_schema';").Scan(&version)
	if err != nil {
		t.Fatalf("migration version was not recorded: %v", err)
	}
}

func TestMigrationsIdempotentOnReopen(t *testing.T) {
	var path string
	var db *sql.DB
	var count int

	var err error

	path = filepath.Join(t.TempDir(), "reopen.db")

	db, err = NewDatabase(path)
	if err != nil {
		t.Fatal(err)
	}
	err = db.Close()
	if err != nil {
		t.Fatal(err)
	}

	db, err = NewDatabase(path)
	if err != nil {
		t.Fatalf("reopening a migrated database failed: %v", err)
	}
	defer db.Close()

	err = db.QueryRow("SELECT COUNT(*) FROM migrations WHERE version = '0001_initial_schema';").Scan(&count)
	if err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("migration version recorded %d times, want 1", count)
	}
}

func TestProvidersActiveUniqueIndex(t *testing.T) {
	var db *sql.DB

	var err error

	db, err = NewDatabase(filepath.Join(t.TempDir(), "providers.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	_, err = db.Exec("INSERT INTO providers (id, name, active) VALUES ('p1', 'one', 1);")
	if err != nil {
		t.Fatalf("first active provider insert failed: %v", err)
	}

	_, err = db.Exec("INSERT INTO providers (id, name, active) VALUES ('p2', 'two', 1);")
	if err == nil {
		t.Fatal("expected a unique index violation for two active providers")
	}
}

func TestAgentsThinkingLevelCheckConstraint(t *testing.T) {
	var db *sql.DB

	var err error

	db, err = NewDatabase(filepath.Join(t.TempDir(), "agents.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	_, err = db.Exec("INSERT INTO agents (id, name, model, thinking_level) VALUES ('a1', 'naru', 'gpt-4o-mini', 'extreme');")
	if err == nil {
		t.Fatal("expected a check constraint violation for an invalid thinking_level")
	}
}

func TestMessagesStatusCheckConstraint(t *testing.T) {
	var db *sql.DB

	var err error

	db, err = NewDatabase(filepath.Join(t.TempDir(), "messages.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	_, err = db.Exec("INSERT INTO agents (id, name, model) VALUES ('a1', 'naru', 'gpt-4o-mini');")
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec("INSERT INTO sessions (id, agent_id) VALUES ('s1', 'a1');")
	if err != nil {
		t.Fatal(err)
	}

	_, err = db.Exec("INSERT INTO messages (id, session_id, role, content, status) VALUES ('m1', 's1', 'user', 'hi', 'unknown');")
	if err == nil {
		t.Fatal("expected a check constraint violation for an invalid status")
	}
}
