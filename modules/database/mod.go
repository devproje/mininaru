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

	rows, err = m.DB.Query("SELECT version FROM migrations;")
	if err != nil {
		goto handle_err
	}
	defer rows.Close()

	for rows.Next() {
		var version string
		err = rows.Scan(&version)
		if err != nil {
			goto handle_err
		}

		versions = append(versions, version)
	}

	if err = rows.Err(); err != nil {
		goto handle_err
	}

	return versions, nil

handle_err:
	return nil, err
}

func (m *DatabaseModule) applyMigration(tx *sql.Tx, path, version string) error {
	var buf []byte
	var err error

	buf, err = fs.ReadFile(migrations, path)
	if err != nil {
		goto handle_err
	}

	_, err = tx.Exec(string(buf))
	if err != nil {
		goto handle_err
	}

	_, err = tx.Exec("INSERT INTO migrations (version) VALUES (?);", version)
	if err != nil {
		goto handle_err
	}

	fmt.Printf("[Database] applied migration: %s\n", version)
	return nil

handle_err:
	return err
}

func (m *DatabaseModule) migrations() error {
	var versions []string
	var glob []string
	var err error
	var tx *sql.Tx

	err = m.initializeMigrate()
	if err != nil {
		goto handle_err
	}

	versions, err = m.queryMigrations()
	if err != nil {
		goto handle_err
	}

	glob, err = fs.Glob(migrations, "migrations/*.sql")
	if err != nil {
		goto handle_err
	}

	if len(glob) == 0 {
		return nil
	}

	tx, err = m.DB.Begin()
	if err != nil {
		goto handle_err
	}

	for _, path := range glob {
		var version = strings.ReplaceAll(path, "migrations/", "")
		if slices.Contains(versions, version) {
			fmt.Printf("[Database] Already applied migration: %s\n", version)
			continue
		}

		err = m.applyMigration(tx, path, version)
		if err != nil {
			_ = tx.Rollback()
			goto handle_err
		}
	}
	err = tx.Commit()
	if err != nil {
		goto handle_err
	}

	return nil

handle_err:
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
		fmt.Printf("[Database] WARN: DATABASE FILE IS NOT EXISTS... CREATE NEW ONE...")
		_ = os.WriteFile(dbpath, nil, 0700)
	}

	m.DB, err = sql.Open("sqlite3", dbpath)
	if err != nil {
		goto handle_err
	}

	fmt.Printf("[Database] Connected database to %s\n", dbpath)

	err = m.migrations()
	if err != nil {
		goto handle_err
	}

	return nil

handle_err:
	return err
}

func (m *DatabaseModule) Unload() error {
	_ = m.DB.Close()
	return nil
}

var Database *DatabaseModule = &DatabaseModule{}
