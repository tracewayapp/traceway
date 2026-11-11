//go:build transactional_pg

package db

import (
	"database/sql"
	"time"

	"github.com/google/uuid"
	"github.com/lib/pq"
)

const (
	ProjectCacheNotificationChannel = "project_cache_changed"
	SdkConfigNotificationChannel    = "sdk_config_changed"
)

func initMainDB() error {
	return initPostgres()
}

func NewPostgresListener() *pq.Listener {
	return pq.NewListener(postgresConnectionString(), 10*time.Second, time.Minute, nil)
}

func NotifyProjectCacheChanged(execer interface {
	Exec(query string, args ...any) (sql.Result, error)
}, projectId uuid.UUID) error {
	_, err := execer.Exec("SELECT pg_notify($1, $2)", ProjectCacheNotificationChannel, projectId.String())
	return err
}

// NotifySdkConfigChanged tells every replica to drop its cached SDK config for the project. Postgres delivers the
// notification only when the transaction commits.
func NotifySdkConfigChanged(execer interface {
	Exec(query string, args ...any) (sql.Result, error)
}, projectId uuid.UUID) error {
	_, err := execer.Exec("SELECT pg_notify($1, $2)", SdkConfigNotificationChannel, projectId.String())
	return err
}

func LockSdkConfigProject(tx *sql.Tx, projectId uuid.UUID) error {
	var id uuid.UUID
	return tx.QueryRow("SELECT id FROM projects WHERE id = $1 FOR UPDATE", projectId).Scan(&id)
}
