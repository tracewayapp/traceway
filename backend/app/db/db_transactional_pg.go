//go:build transactional_pg

package db

import (
	"database/sql"
	"time"

	"github.com/google/uuid"
	"github.com/lib/pq"
)

const ProjectCacheNotificationChannel = "project_cache_changed"

func initMainDB() error {
	return initPostgres()
}

func NewPostgresListener() *pq.Listener {
	return pq.NewListener(postgresConnectionString(), 10*time.Second, time.Minute, nil)
}

func NotifyProjectCacheChanged(tx *sql.Tx, projectId uuid.UUID) error {
	return notifyProjectCacheChanged(tx, projectId)
}

func notifyProjectCacheChanged(execer interface {
	Exec(query string, args ...any) (sql.Result, error)
}, projectId uuid.UUID) error {
	_, err := execer.Exec("SELECT pg_notify($1, $2)", ProjectCacheNotificationChannel, projectId.String())
	return err
}
