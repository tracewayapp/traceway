package retention

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/tracewayapp/traceway/backend/v2/app/db"
	"github.com/tracewayapp/traceway/backend/v2/app/traceway"
	"go.opentelemetry.io/otel/codes"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	_ "modernc.org/sqlite"
)

func recordTasks(t *testing.T) *tracetest.SpanRecorder {
	t.Helper()
	recorder := tracetest.NewSpanRecorder()
	t.Cleanup(traceway.Install(traceway.Providers{
		TracerProvider: sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(recorder)),
	}))
	return recorder
}

func isIdleTask(span sdktrace.ReadOnlySpan) bool {
	for _, kv := range span.Attributes() {
		if kv.Key == "traceway.task.idle" {
			return kv.Value.AsBool()
		}
	}
	return false
}

func TestDirCleanupTicksThatRemoveNothingAreSkipped(t *testing.T) {
	recorder := recordTasks(t)
	dir := t.TempDir()
	fresh := filepath.Join(dir, "project-1", "fresh.json")
	old := filepath.Join(dir, "project-1", "old.json")
	mustWriteFile(t, fresh, "fresh")

	runDirCleanupTask(context.Background(), "recordings", dir, 30)

	mustWriteFile(t, old, "old")
	mustChtime(t, old, time.Now().Add(-40*24*time.Hour))
	runDirCleanupTask(context.Background(), "recordings", dir, 30)

	spans := recorder.Ended()
	if len(spans) != 2 {
		t.Fatalf("got %d spans", len(spans))
	}
	if !isIdleTask(spans[0]) {
		t.Fatal("a tick that removed nothing must be skipped")
	}
	if isIdleTask(spans[1]) {
		t.Fatal("a tick that removed a file must be reported")
	}
}

func TestDirCleanupFailuresMarkTheTask(t *testing.T) {
	recorder := recordTasks(t)
	file := filepath.Join(t.TempDir(), "not-a-dir")
	mustWriteFile(t, file, "x")

	runDirCleanupTask(context.Background(), "recordings", file, 30)

	spans := recorder.Ended()
	if len(spans) != 1 || spans[0].Name() != "retention.recordings" {
		t.Fatalf("the failure must be recorded on the task, got %d spans", len(spans))
	}
	if spans[0].Status().Code != codes.Error || len(spans[0].Events()) != 1 {
		t.Fatalf("the task must carry the failure, status %v", spans[0].Status())
	}
}

func TestDBPruneTicksThatPruneNothingAreSkipped(t *testing.T) {
	conn, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	conn.SetMaxOpenConns(1)
	prev := db.DB
	db.DB = conn
	t.Cleanup(func() {
		conn.Close()
		db.DB = prev
	})
	recorder := recordTasks(t)

	for _, pruned := range []int64{0, 3} {
		runDBPrune(context.Background(), "auth_tokens", func(*sql.Tx) (int64, error) { return pruned, nil })
	}
	runDBPrune(context.Background(), "auth_tokens", func(*sql.Tx) (int64, error) { return 0, errors.New("database is locked") })

	spans := recorder.Ended()
	if len(spans) != 3 {
		t.Fatalf("got %d spans", len(spans))
	}
	if !isIdleTask(spans[0]) {
		t.Fatal("a prune that removed nothing must be skipped")
	}
	if isIdleTask(spans[1]) {
		t.Fatal("a prune that removed rows must be reported")
	}
	if spans[2].Status().Code != codes.Error || isIdleTask(spans[2]) {
		t.Fatal("a failed prune must mark its task")
	}
}
