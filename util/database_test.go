// SPDX-FileCopyrightText: 2026 Wonhyeok Kim (Project_IO)
// SPDX-License-Identifier: GPL-3.0-or-later

package util

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"
)

func openTestDB(t *testing.T) *sql.DB {
	var database *sql.DB

	var err error

	t.Helper()

	database, err = NewDatabase(filepath.Join(t.TempDir(), "pragma.db"))
	if err != nil {
		t.Fatalf("Create Database failed: %v", err)
	}
	t.Cleanup(func() { database.Close() })

	return database
}

func TestDatabasePragmaOnEveryConnection(t *testing.T) {
	var database *sql.DB
	var held []*sql.Conn
	var conn *sql.Conn
	var index int
	var foreignKeys int
	var busyTimeout int
	var journalMode string

	var err error

	database = openTestDB(t)

	for index = range 4 {
		conn, err = database.Conn(context.Background())
		if err != nil {
			t.Fatalf("conn %d failed: %v", index, err)
		}
		held = append(held, conn)

		err = conn.QueryRowContext(context.Background(), "PRAGMA foreign_keys;").Scan(&foreignKeys)
		if err != nil {
			t.Fatalf("conn %d foreign_keys failed: %v", index, err)
		}
		if foreignKeys != 1 {
			t.Fatalf("conn %d has foreign_keys = %d, want 1", index, foreignKeys)
		}

		err = conn.QueryRowContext(context.Background(), "PRAGMA busy_timeout;").Scan(&busyTimeout)
		if err != nil {
			t.Fatalf("conn %d busy_timeout failed: %v", index, err)
		}
		if busyTimeout != 5000 {
			t.Fatalf("conn %d has busy_timeout = %d, want 5000", index, busyTimeout)
		}

		err = conn.QueryRowContext(context.Background(), "PRAGMA journal_mode;").Scan(&journalMode)
		if err != nil {
			t.Fatalf("conn %d journal_mode failed: %v", index, err)
		}
		if journalMode != "wal" {
			t.Fatalf("conn %d has journal_mode = %q, want wal", index, journalMode)
		}
	}

	for _, conn = range held {
		conn.Close()
	}
}

func TestDatabaseCascadeDeleteAcrossConnections(t *testing.T) {
	var database *sql.DB
	var index int
	var sessions int
	var messages int

	var err error

	database = openTestDB(t)

	_, err = database.Exec("INSERT INTO agents (id, name, model) VALUES ('a1', 'naru', 'gpt-4o-mini');")
	if err != nil {
		t.Fatalf("agent insert failed: %v", err)
	}

	_, err = database.Exec("INSERT INTO sessions (id, agent_id, name) VALUES ('s1', 'a1', 'test');")
	if err != nil {
		t.Fatalf("session insert failed: %v", err)
	}

	_, err = database.Exec(`INSERT INTO messages (id, session_id, role, content, status, error)
		VALUES ('m1', 's1', 'user', 'hi', 'completed', '');`)
	if err != nil {
		t.Fatalf("message insert failed: %v", err)
	}

	for index = range 8 {
		_, err = database.Exec("SELECT 1;")
		if err != nil {
			t.Fatalf("warm up %d failed: %v", index, err)
		}
	}

	_, err = database.Exec("DELETE FROM agents WHERE id = 'a1';")
	if err != nil {
		t.Fatalf("agent delete failed: %v", err)
	}

	err = database.QueryRow("SELECT COUNT(*) FROM sessions;").Scan(&sessions)
	if err != nil {
		t.Fatalf("session count failed: %v", err)
	}
	if sessions != 0 {
		t.Fatalf("sessions did not cascade, %d rows remain", sessions)
	}

	err = database.QueryRow("SELECT COUNT(*) FROM messages;").Scan(&messages)
	if err != nil {
		t.Fatalf("message count failed: %v", err)
	}
	if messages != 0 {
		t.Fatalf("messages did not cascade, %d rows remain", messages)
	}
}

func TestDatabaseRejectsOrphanMessage(t *testing.T) {
	var database *sql.DB

	var err error

	database = openTestDB(t)

	_, err = database.Exec(`INSERT INTO messages (id, session_id, role, content, status, error)
		VALUES ('m1', 'missing', 'user', 'hi', 'completed', '');`)
	if err == nil {
		t.Fatal("expected foreign key violation for a message without a session")
	}
}

func TestDatabaseRejectsOrphanSession(t *testing.T) {
	var database *sql.DB

	var err error

	database = openTestDB(t)

	_, err = database.Exec("INSERT INTO sessions (id, agent_id, name) VALUES ('s1', 'missing', 'test');")
	if err == nil {
		t.Fatal("expected foreign key violation for a session without an agent")
	}
}
