//go:build !transactional_pg

package db

import (
	"database/sql"

	"github.com/google/uuid"
	"github.com/tracewayapp/lit/v2"
	"github.com/tracewayapp/traceway/backend/app/config"
)

func initMainDB() error {
	if config.Config.DBType != "sqlite" {
		return initPostgres()
	}

	path := config.Config.SQLitePath
	if path == "" {
		path = "./traceway.db"
	}

	mainDB, err := openSQLite(path, false)
	if err != nil {
		return err
	}
	DB = mainDB
	Driver = lit.SQLite
	config.Logf("SQLite database opened at %s", path)

	return nil
}

func NotifyProjectCacheChanged(*sql.Tx, uuid.UUID) error {
	return nil
}
