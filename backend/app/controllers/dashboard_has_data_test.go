package controllers

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
)

func stubProjectDataCount(t *testing.T, timeout time.Duration, count func(ctx context.Context, projectId uuid.UUID, now time.Time) (bool, error)) {
	t.Helper()
	prevTimeout, prevCount := hasDataTimeout, countProjectData
	hasDataTimeout, countProjectData = timeout, count
	t.Cleanup(func() { hasDataTimeout, countProjectData = prevTimeout, prevCount })
}

// SQLite and DuckDB reads ignore the context, so the deadline must hold even
// when the count does not return, and the count must still finish on its own.
func TestProjectHasDataKeepsItsDeadlineOverAnUncancellableCount(t *testing.T) {
	release := make(chan struct{})
	finished := make(chan struct{})
	stubProjectDataCount(t, 20*time.Millisecond, func(context.Context, uuid.UUID, time.Time) (bool, error) {
		defer close(finished)
		<-release
		return false, nil
	})

	answered := make(chan bool, 1)
	go func() { answered <- projectHasData(context.Background(), uuid.New(), time.Now()) }()
	select {
	case hasData := <-answered:
		if !hasData {
			t.Fatal("a count that timed out must not swap the dashboard for the setup guide")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("projectHasData waited for a count past its deadline")
	}

	close(release)
	select {
	case <-finished:
	case <-time.After(5 * time.Second):
		t.Fatal("the abandoned count never finished")
	}
}

func TestProjectHasDataAnswersFromTheCount(t *testing.T) {
	stubProjectDataCount(t, time.Second, func(context.Context, uuid.UUID, time.Time) (bool, error) {
		return false, nil
	})
	if projectHasData(context.Background(), uuid.New(), time.Now()) {
		t.Fatal("a project without data must get the setup guide")
	}

	stubProjectDataCount(t, time.Second, func(context.Context, uuid.UUID, time.Time) (bool, error) {
		return false, errors.New("telemetry database is locked")
	})
	if !projectHasData(context.Background(), uuid.New(), time.Now()) {
		t.Fatal("a failed count must not swap the dashboard for the setup guide")
	}

	stubProjectDataCount(t, time.Second, func(context.Context, uuid.UUID, time.Time) (bool, error) {
		panic("boom")
	})
	started := time.Now()
	if !projectHasData(context.Background(), uuid.New(), time.Now()) || time.Since(started) > 500*time.Millisecond {
		t.Fatal("a panicking count must answer at once, without hiding the dashboard")
	}
}
