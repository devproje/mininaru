// SPDX-License-Identifier: GPL-2.0-or-later
/*
 * MiniNaru
 * Copyright (C) 2022-2026 Project_IO
 *
 * This program is free software; you can redistribute it and/or modify
 * it under the terms of the GNU General Public License as published by
 * the Free Software Foundation; either version 2 of the License, or
 * (at your option) any later version.
 */

package database

import (
	"database/sql"
	"embed"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"git.wh64.net/naru-studio/mininaru/config"
	"git.wh64.net/naru-studio/mininaru/log"
	_ "github.com/mattn/go-sqlite3"
)

type DatabaseModule struct {
	DB *sql.DB
}

//go:embed migrations
var migrations embed.FS

var schema = `CREATE TABLE IF NOT EXISTS migrations (
	version		VARCHAR(255) NOT NULL,
	applied_at	DATETIME DEFAULT CURRENT_TIMESTAMP,
	PRIMARY KEY(version)
);`

func (m *DatabaseModule) initializeMigrate() error {
	log.Debugf("[database]: creating migrations schema table...\n")
	var _, err = m.DB.Exec(schema)
	if err != nil {
		return err
	}

	return nil
}

func (m *DatabaseModule) queryMigrations() ([]string, error) {
	var err error
	var rows *sql.Rows
	var versions []string = make([]string, 0)

	log.Debugf("[database]: get migrations from database file...\n")
	rows, err = m.DB.Query("SELECT version FROM migrations;")
	if err != nil {
		goto err_cleanup
	}

	for rows.Next() {
		var version string
		err = rows.Scan(&version)
		if err != nil {
			goto err_row_cleanup
		}

		log.Debugf("[database]: loaded migration version data: %s\n", version)

		versions = append(versions, version)
	}

	rows.Close()

	return versions, nil

err_row_cleanup:
	rows.Close()
err_cleanup:
	return nil, err
}

func (m *DatabaseModule) applyMigration(tx *sql.Tx, path, version string) error {
	var buf []byte
	var err error

	log.Debugf("[database]: loading embedded migration file: %s\n", path)
	buf, err = fs.ReadFile(migrations, path)
	if err != nil {
		goto err_cleanup
	}

	log.Debugf("[database]: executing migration:\n%s\n", string(buf))
	_, err = tx.Exec(string(buf))
	if err != nil {
		goto err_cleanup
	}

	log.Debugf("[database]: upload version data to migration table: %s\n", version)
	_, err = tx.Exec("INSERT INTO migrations (version) VALUES (?);", version)
	if err != nil {
		goto err_cleanup
	}

	log.Printf("[database]: applied migration: %s\n", version)
	return nil

err_cleanup:
	return err
}

func (m *DatabaseModule) migrations() error {
	var versions []string
	var glob []string
	var err error
	var tx *sql.Tx

	err = m.initializeMigrate()
	if err != nil {
		goto err_cleanup
	}

	versions, err = m.queryMigrations()
	if err != nil {
		goto err_cleanup
	}

	glob, err = fs.Glob(migrations, "migrations/*.sql")
	if err != nil {
		goto err_cleanup
	}
	if len(glob) == 0 {
		return nil
	}

	tx, err = m.DB.Begin()
	if err != nil {
		goto err_cleanup
	}

	for _, path := range glob {
		var version = strings.ReplaceAll(path, "migrations/", "")
		if slices.Contains(versions, version) {
			log.Printf("[database]: already applied migration: %s\n", version)
			continue
		}

		err = m.applyMigration(tx, path, version)
		if err != nil {
			goto err_tx_failed
		}
	}

	err = tx.Commit()
	if err != nil {
		goto err_tx_failed
	}

	return nil

err_tx_failed:
	_ = tx.Rollback()
err_cleanup:
	return err
}

func (m *DatabaseModule) Name() string {
	return "database-module"
}

func (m *DatabaseModule) Load() error {
	var err error
	var cnf = config.Get
	var dbpath = filepath.Join(cnf.DataDir, "data.db")

	_, err = os.Stat(dbpath)
	if err != nil {
		log.Warnf("[database]: database file not exists... create new one...\n")
		_ = os.WriteFile(dbpath, nil, 0700)
		log.Debugf("[database]: created database file: %s\n", dbpath)
	}

	m.DB, err = sql.Open("sqlite3", fmt.Sprintf("%s?_journal_mode=WAL&_busy_timeout=5000", dbpath))
	if err != nil {
		goto err_cleanup
	}

	log.Printf("[database]: connected database to %s\n", dbpath)

	err = m.migrations()
	if err != nil {
		goto err_cleanup
	}

	return nil

err_cleanup:
	return err
}

func (m *DatabaseModule) Unload() error {
	_ = m.DB.Close()
	return nil
}

var Database *DatabaseModule = &DatabaseModule{}
