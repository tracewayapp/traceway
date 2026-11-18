package controllers

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/tracewayapp/traceway/backend/v2/app/cache"
	"github.com/tracewayapp/traceway/backend/v2/app/models"
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

func TestHasDataWindowsStartWithTheLastMonthAndStopAtTheProjectsCreation(t *testing.T) {
	now := time.Date(2026, 11, 13, 12, 0, 0, 0, time.UTC)

	old := &models.Project{Id: uuid.New(), Token: uuid.NewString(), CreatedAt: now.Add(-400 * 24 * time.Hour)}
	fresh := &models.Project{Id: uuid.New(), Token: uuid.NewString(), CreatedAt: now.Add(-2 * time.Hour)}
	for _, p := range []*models.Project{old, fresh} {
		cache.ProjectCache.AddProject(p)
		t.Cleanup(func() { cache.ProjectCache.RemoveProject(p.Id) })
	}

	windows := hasDataWindows(old.Id, now)
	if len(windows) != 2 {
		t.Fatalf("an old project gets %d windows, want the last month and then the rest", len(windows))
	}
	if windows[0].name != "recent" || !windows[0].start.Equal(now.Add(-hasDataRecentWindow)) || !windows[0].end.Equal(now) {
		t.Fatalf("first window %+v is not the last month", windows[0])
	}
	if !windows[1].end.Equal(windows[0].start) || !windows[1].start.Equal(old.CreatedAt.Add(-hasDataClockSlack)) {
		t.Fatalf("second window %+v does not run from the project's creation up to the first", windows[1])
	}

	windows = hasDataWindows(fresh.Id, now)
	if len(windows) != 1 || !windows[0].start.Equal(fresh.CreatedAt.Add(-hasDataClockSlack)) || !windows[0].end.Equal(now) {
		t.Fatalf("a new project must be counted from its creation only, got %+v", windows)
	}
}

func TestProjectHasDataRemembersAYesButNotANo(t *testing.T) {
	calls := 0
	answer := false
	stubProjectDataCount(t, time.Second, func(context.Context, uuid.UUID, time.Time) (bool, error) {
		calls++
		return answer, nil
	})
	projectId := uuid.New()
	t.Cleanup(func() { projectsWithData.Delete(projectId) })

	if projectHasData(context.Background(), projectId, time.Now()) {
		t.Fatal("an empty project was reported as having data")
	}
	answer = true
	for range 3 {
		if !projectHasData(context.Background(), projectId, time.Now()) {
			t.Fatal("a project with data was reported empty")
		}
	}
	if calls != 2 {
		t.Fatalf("counted %d times, want 2: the no is asked again, the yes is remembered", calls)
	}
}
