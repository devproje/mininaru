package database

import "database/sql"

type DatabaseModule struct {
	DB *sql.DB
}

func (m *DatabaseModule) Name() string {
	return "database-module"
}

func (m *DatabaseModule) Load() error {
	return nil
}

func (m *DatabaseModule) Unload() error {
	return nil
}

var Database *DatabaseModule = &DatabaseModule{}
