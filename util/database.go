package util

import (
	"database/sql"
	"embed"
	"io/fs"
	"log"
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
	var mig fs.DirEntry
	var row *sql.Rows

	var heap string
	var applied []string

	var tx *sql.Tx
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
				log.Printf("[db] rollback after migration %s failure also failed: %v", version, rollbackErr)
			}
			return err
		}
	}

	err = tx.Commit()
	if err != nil {
		rollbackErr = tx.Rollback()
		if rollbackErr != nil {
			log.Printf("[db] rollback after commit failure also failed: %v", rollbackErr)
		}
		return err
	}

	return nil
}

func InitDatabase(dbPath string) (*sql.DB, error) {
	var database *sql.DB
	var err error

	database, err = sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, err
	}

	_, err = database.Exec("PRAGMA journal_mode = WAL")
	if err != nil {
		return nil, err
	}

	_, err = database.Exec("PRAGMA foreign_keys = ON")
	if err != nil {
		return nil, err
	}

	_, err = database.Exec("PRAGMA busy_timeout = 5000")
	if err != nil {
		return nil, err
	}

	err = migrations(database)
	if err != nil {
		return nil, err
	}

	return database, nil
}
