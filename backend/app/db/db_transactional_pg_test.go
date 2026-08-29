//go:build transactional_pg

package db

import (
	"database/sql"
	"testing"

	"github.com/google/uuid"
)

type recordingExecer struct {
	query string
	args  []any
	err   error
}

func (e *recordingExecer) Exec(query string, args ...any) (sql.Result, error) {
	e.query = query
	e.args = args
	return nil, e.err
}

func TestNotifyProjectCacheChangedIncludesProjectId(t *testing.T) {
	projectId := uuid.New()
	execer := &recordingExecer{}

	if err := notifyProjectCacheChanged(execer, projectId); err != nil {
		t.Fatalf("notify project cache changed: %v", err)
	}
	if execer.query != "SELECT pg_notify($1, $2)" {
		t.Fatalf("query = %q", execer.query)
	}
	if len(execer.args) != 2 {
		t.Fatalf("args = %#v", execer.args)
	}
	if execer.args[0] != ProjectCacheNotificationChannel {
		t.Errorf("channel = %v", execer.args[0])
	}
	if execer.args[1] != projectId.String() {
		t.Errorf("payload = %v", execer.args[1])
	}
}
