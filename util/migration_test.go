// SPDX-FileCopyrightText: 2026 Wonhyeok Kim (Project_IO)
// SPDX-License-Identifier: GPL-3.0-or-later

package util

import (
	"database/sql"
	"path/filepath"
	"testing"
)

func TestReasoningColumnAddedToExistingDatabase(t *testing.T) {
	var db *sql.DB
	var schema []byte
	var count int
	var content, reasoning string

	var err error

	db, err = sql.Open("sqlite", filepath.Join(t.TempDir(), "old.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	schema, err = files.ReadFile("migrations/0001_schema.sql")
	if err != nil {
		t.Fatal(err)
	}

	_, err = db.Exec(migrationSchema)
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec(string(schema))
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec("INSERT INTO migrations (version) VALUES ('0001_schema');")
	if err != nil {
		t.Fatal(err)
	}

	_, err = db.Exec("INSERT INTO sessions (id, agent_id, name) VALUES ('s1','a1','old chat');")
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec("INSERT INTO messages (id, session_id, role, content) VALUES ('m1','s1','user','kept');")
	if err != nil {
		t.Fatal(err)
	}

	err = migrations(db)
	if err != nil {
		t.Fatalf("upgrading an existing database failed: %v", err)
	}

	err = db.QueryRow("SELECT COUNT(*) FROM pragma_table_info('messages') WHERE name = 'reasoning';").Scan(&count)
	if err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatal("messages table has no reasoning column after migrating")
	}

	err = db.QueryRow("SELECT content, reasoning FROM messages WHERE id = 'm1';").Scan(&content, &reasoning)
	if err != nil {
		t.Fatal(err)
	}
	if content != "kept" {
		t.Fatalf("pre-existing message lost content: %q", content)
	}
	if reasoning != "" {
		t.Fatalf("pre-existing message should default to empty reasoning, got %q", reasoning)
	}
}

func TestMessageStatusColumnsAddedToExistingDatabase(t *testing.T) {
	var db *sql.DB
	var schema, reasoningMigration []byte
	var status, errorText string

	var err error

	db, err = sql.Open("sqlite", filepath.Join(t.TempDir(), "old.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	schema, err = files.ReadFile("migrations/0001_schema.sql")
	if err != nil {
		t.Fatal(err)
	}
	reasoningMigration, err = files.ReadFile("migrations/0002_message_reasoning.sql")
	if err != nil {
		t.Fatal(err)
	}

	_, err = db.Exec(migrationSchema)
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec(string(schema))
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec(string(reasoningMigration))
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec("INSERT INTO migrations (version) VALUES ('0001_schema'), ('0002_message_reasoning');")
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec("INSERT INTO sessions (id, agent_id, name) VALUES ('s1','a1','old chat');")
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec("INSERT INTO messages (id, session_id, role, content) VALUES ('m1','s1','user','kept');")
	if err != nil {
		t.Fatal(err)
	}

	err = migrations(db)
	if err != nil {
		t.Fatal(err)
	}
	err = db.QueryRow("SELECT status, error FROM messages WHERE id = 'm1';").Scan(&status, &errorText)
	if err != nil {
		t.Fatal(err)
	}
	if status != "completed" || errorText != "" {
		t.Fatalf("migrated message status=%q error=%q", status, errorText)
	}
}

func TestToolCallsTableCreated(t *testing.T) {
	var db *sql.DB
	var count int

	var err error

	db, err = InitDatabase(filepath.Join(t.TempDir(), "tools.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	err = db.QueryRow("SELECT COUNT(*) FROM pragma_table_info('tool_calls');").Scan(&count)
	if err != nil {
		t.Fatal(err)
	}
	if count != 9 {
		t.Fatalf("tool_calls has %d columns, want 9", count)
	}
}

func TestSessionOriginColumnsAddedToExistingDatabase(t *testing.T) {
	var path string
	var db *sql.DB
	var origin, externalId string
	var count int

	var err error

	path = filepath.Join(t.TempDir(), "legacy.db")

	db, err = InitDatabase(path)
	if err != nil {
		t.Fatal(err)
	}

	_, err = db.Exec("INSERT INTO sessions (id, agent_id, name) VALUES ('s1', 'a1', 'legacy');")
	if err != nil {
		t.Fatal(err)
	}

	err = db.Close()
	if err != nil {
		t.Fatal(err)
	}

	db, err = InitDatabase(path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	err = db.QueryRow("SELECT origin, external_id FROM sessions WHERE id = 's1';").Scan(&origin, &externalId)
	if err != nil {
		t.Fatal(err)
	}

	if origin != "" || externalId != "" {
		t.Fatalf("legacy session = origin %q external %q, want both blank", origin, externalId)
	}

	_, err = db.Exec("INSERT INTO sessions (id, agent_id, name) VALUES ('s2', 'a1', 'another legacy');")
	if err != nil {
		t.Fatalf("blank external ids collided on the unique index: %v", err)
	}

	_, err = db.Exec("INSERT INTO sessions (id, agent_id, name, origin, external_id) VALUES ('s3', 'a1', 'x', 'discord', 'c1');")
	if err != nil {
		t.Fatal(err)
	}

	_, err = db.Exec("INSERT INTO sessions (id, agent_id, name, origin, external_id) VALUES ('s4', 'a1', 'y', 'discord', 'c1');")
	if err == nil {
		t.Fatal("two live sessions bound to the same channel were allowed")
	}

	err = db.QueryRow("SELECT count(*) FROM sessions;").Scan(&count)
	if err != nil {
		t.Fatal(err)
	}
	if count != 3 {
		t.Fatalf("sessions = %d, want 3", count)
	}
}

func TestSessionSummariesTableCreated(t *testing.T) {
	var db *sql.DB
	var schema []byte
	var count int

	var err error

	db, err = sql.Open("sqlite", filepath.Join(t.TempDir(), "old.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	schema, err = files.ReadFile("migrations/0001_schema.sql")
	if err != nil {
		t.Fatal(err)
	}

	_, err = db.Exec(migrationSchema)
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec(string(schema))
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec("INSERT INTO migrations (version) VALUES ('0001_schema');")
	if err != nil {
		t.Fatal(err)
	}

	err = migrations(db)
	if err != nil {
		t.Fatalf("upgrading an existing database failed: %v", err)
	}

	err = db.QueryRow("SELECT COUNT(*) FROM sqlite_master WHERE type = 'table' AND name = 'session_summaries';").Scan(&count)
	if err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatal("session_summaries was not created on an existing database")
	}
}

func TestTokenUsageTableCreated(t *testing.T) {
	var db *sql.DB
	var schema []byte
	var count int
	var rows *sql.Rows

	var err error

	db, err = sql.Open("sqlite", filepath.Join(t.TempDir(), "old.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	schema, err = files.ReadFile("migrations/0001_schema.sql")
	if err != nil {
		t.Fatal(err)
	}

	_, err = db.Exec(migrationSchema)
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec(string(schema))
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec("INSERT INTO migrations (version) VALUES ('0001_schema');")
	if err != nil {
		t.Fatal(err)
	}

	err = migrations(db)
	if err != nil {
		t.Fatalf("upgrading an existing database failed: %v", err)
	}

	err = db.QueryRow("SELECT COUNT(*) FROM sqlite_master WHERE type = 'table' AND name = 'token_usage';").Scan(&count)
	if err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatal("token_usage was not created on an existing database")
	}
	rows, err = db.Query("SELECT context_tokens, context_window, cached_tokens FROM token_usage LIMIT 0;")
	if err != nil {
		t.Fatalf("token_usage context columns were not added: %v", err)
	}
	rows.Close()
}

func TestCachedTokensColumnRepairedAfterMigrationAlreadyRecorded(t *testing.T) {
	var db *sql.DB
	var schema []byte
	var count int

	var err error

	db, err = sql.Open("sqlite", filepath.Join(t.TempDir(), "old.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	schema, err = files.ReadFile("migrations/0001_schema.sql")
	if err != nil {
		t.Fatal(err)
	}

	_, err = db.Exec(migrationSchema)
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec(string(schema))
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec(`CREATE TABLE token_usage (
		id VARCHAR(36) PRIMARY KEY,
		session_id VARCHAR(36) NOT NULL,
		message_id VARCHAR(36) NOT NULL DEFAULT '',
		kind VARCHAR(16) NOT NULL,
		prompt_tokens INTEGER NOT NULL DEFAULT 0,
		completion_tokens INTEGER NOT NULL DEFAULT 0,
		total_tokens INTEGER NOT NULL DEFAULT 0,
		context_tokens INTEGER NOT NULL DEFAULT 0,
		context_window INTEGER NOT NULL DEFAULT 0
	);`)
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec(`INSERT INTO migrations (version) VALUES
		('0001_schema'), ('0011_token_usage'), ('0012_context_tokens');`)
	if err != nil {
		t.Fatal(err)
	}

	err = migrations(db)
	if err != nil {
		t.Fatalf("repairing an already-recorded migration failed: %v", err)
	}

	err = db.QueryRow("SELECT COUNT(*) FROM pragma_table_info('token_usage') WHERE name = 'cached_tokens';").Scan(&count)
	if err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("cached_tokens column count = %d, want 1", count)
	}
}
