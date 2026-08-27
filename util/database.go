// SPDX-FileCopyrightText: 2026 Wonhyeok Kim (Project_IO)
// SPDX-License-Identifier: GPL-3.0-or-later

package util

import (
	"database/sql"
	"embed"
	"io/fs"
	"path/filepath"
	"slices"
	"sort"
	"strings"

	_ "modernc.org/sqlite"
)

const migrationSchema = `CREATE TABLE IF NOT EXISTS migrations(
	version VARCHAR(255) PRIMARY KEY,
	created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);`

var (
	DB *sql.DB
	//go:embed migrations/*.sql
	files embed.FS
)

func migration(tx *sql.Tx, version, buf string) error {
	var err error

	_, err = tx.Exec("INSERT INTO migrations (version) VALUES (?);", version)
	if err != nil {
		return err
	}

	_, err = tx.Exec(buf)
	if err != nil {
		return err
	}

	return nil
}

func migrations(db *sql.DB) error {
	var migs []fs.DirEntry
	var row *sql.Rows
	var heap string
	var applied []string
	var tx *sql.Tx
	var mig fs.DirEntry
	var version, path string
	var buf []byte
	var rollbackErr error

	var err error

	migs, err = files.ReadDir("migrations")
	if err != nil {
		return err
	}

	sort.Slice(migs, func(i, j int) bool {
		return migs[i].Name() < migs[j].Name()
	})

	_, err = db.Exec(migrationSchema)
	if err != nil {
		return err
	}

	row, err = db.Query("SELECT version FROM migrations;")
	if err != nil {
		return err
	}
	defer row.Close()

	for row.Next() {
		err = row.Scan(&heap)
		if err != nil {
			return err
		}

		applied = append(applied, heap)
	}

	err = row.Err()
	if err != nil {
		return err
	}

	err = row.Close()
	if err != nil {
		return err
	}

	tx, err = db.Begin()
	if err != nil {
		return err
	}

	for _, mig = range migs {
		if mig.IsDir() {
			continue
		}

		version, _ = strings.CutSuffix(mig.Name(), ".sql")
		path = filepath.Join("migrations", mig.Name())
		buf, err = files.ReadFile(path)
		if err != nil {
			return err
		}

		if slices.Contains(applied, version) {
			continue
		}

		err = migration(tx, version, string(buf))
		if err != nil {
			rollbackErr = tx.Rollback()
			if rollbackErr != nil {
				Log.Error("rollback after a failed migration also failed", "version", version, "error", rollbackErr)
			}
			return err
		}
	}

	err = tx.Commit()
	if err != nil {
		rollbackErr = tx.Rollback()
		if rollbackErr != nil {
			Log.Error("rollback after a failed migration commit also failed", "error", rollbackErr)
		}
		return err
	}

	return nil
}

func databaseDSN(dbPath string) string {
	var query string

	query = strings.Join([]string{
		"_pragma=journal_mode(WAL)",
		"_pragma=foreign_keys(1)",
		"_pragma=busy_timeout(5000)",
	}, "&")

	return "file:" + dbPath + "?" + query
}

func NewDatabase(dbPath string) (*sql.DB, error) {
	var db *sql.DB
	var err error

	db, err = sql.Open("sqlite", databaseDSN(dbPath))
	if err != nil {
		return nil, err
	}

	err = migrations(db)
	if err != nil {
		return nil, err
	}

	return db, nil
}
