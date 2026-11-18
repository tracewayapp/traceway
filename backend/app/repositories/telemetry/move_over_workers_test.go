package telemetry

import (
	"context"
	"errors"
	"strings"
	"testing"
	"testing/synctest"
	"time"

	"github.com/tracewayapp/traceway/backend/v2/app/models"
	"github.com/tracewayapp/traceway/backend/v2/app/repositories/transactional"
)

func TestMoveOverDaysRunConcurrentlyWithoutOverlapping(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		newest := time.Now().UTC().Truncate(moveOverDay)
		started := make(chan time.Time, 6)
		release := make(chan struct{})
		defer close(release)
		done := make(chan error, 1)
		go func() {
			done <- runMoveOverDays(context.Background(), newest, newest.Add(-5*moveOverDay), 2, func(ctx context.Context, day time.Time) error {
				started <- day
				<-release
				return nil
			})
		}()
		synctest.Wait()
		if len(started) != 2 {
			t.Fatalf("want two days running concurrently, got %d", len(started))
		}
		seen := map[time.Time]bool{<-started: true, <-started: true}
		if !seen[newest] || !seen[newest.Add(-moveOverDay)] {
			t.Fatalf("the newest days must start first: %v", seen)
		}
		for range 4 {
			release <- struct{}{}
			synctest.Wait()
			if len(started) != 1 {
				t.Fatalf("one available worker should start exactly one day: %d", len(started))
			}
			day := <-started
			if seen[day] {
				t.Fatalf("day %v scheduled more than once", day)
			}
			seen[day] = true
		}
		release <- struct{}{}
		release <- struct{}{}
		if err := <-done; err != nil || len(seen) != 6 {
			t.Fatalf("want six completed days: %v, %v", seen, err)
		}
	})
}

func TestMoveOverDaysCancelAndJoinWorkers(t *testing.T) {
	for _, failure := range []string{"error", "panic", "cancel"} {
		t.Run(failure, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				ctx, cancel := context.WithCancel(context.Background())
				defer cancel()
				newest := time.Now().UTC().Truncate(moveOverDay)
				peerStarted := make(chan struct{})
				peerFinished := make(chan struct{})
				injected := errors.New("copy failed")
				err := runMoveOverDays(ctx, newest, newest.Add(-5*moveOverDay), 2, func(ctx context.Context, day time.Time) error {
					if day.Equal(newest) {
						<-peerStarted
						switch failure {
						case "panic":
							panic("copy panicked")
						case "cancel":
							cancel()
							return ctx.Err()
						default:
							return injected
						}
					}
					if !day.Equal(newest.Add(-moveOverDay)) {
						t.Errorf("started another day after failure: %v", day)
						return errors.New("unexpected day")
					}
					close(peerStarted)
					<-ctx.Done()
					time.Sleep(time.Second)
					close(peerFinished)
					return ctx.Err()
				})
				select {
				case <-peerFinished:
				default:
					t.Fatal("returned before the cancelled worker finished")
				}
				switch failure {
				case "error":
					if !errors.Is(err, injected) {
						t.Fatalf("lost the original failure: %v", err)
					}
				case "panic":
					if err == nil || !strings.Contains(err.Error(), "copy panicked") {
						t.Fatalf("lost the worker panic: %v", err)
					}
				case "cancel":
					if !errors.Is(err, context.Canceled) {
						t.Fatalf("lost external cancellation: %v", err)
					}
				}
			})
		})
	}
}

type slowMoveOverLegacy struct {
	moveOverLegacy
}

func (slowMoveOverLegacy) FindEndpoints(context.Context, time.Time, time.Time, int, *int) ([]models.Endpoint, error) {
	time.Sleep(11 * time.Second)
	return nil, nil
}

type recordingMoveOverProgress struct {
	moveOverProgress
	days chan transactional.MoveOverDay
	err  error
}

func (p *recordingMoveOverProgress) SaveProgress(ctx context.Context, day transactional.MoveOverDay) error {
	p.days <- day
	return p.err
}

func TestMoveOverReportsProgressBeforeDayCompletes(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		start := time.Now().UTC().Truncate(moveOverDay)
		progress := &recordingMoveOverProgress{days: make(chan transactional.MoveOverDay, 16)}
		m := &mover{
			MoveOverOptions: MoveOverOptions{PageSize: 2, Log: func(string, ...any) {}},
			legacy:          slowMoveOverLegacy{}, progress: progress,
		}
		done := make(chan error, 1)
		go func() {
			_, err := m.moveDay(context.Background(), "endpoints", start, false)
			done <- err
		}()
		time.Sleep(12 * time.Second)
		synctest.Wait()
		if len(progress.days) != 1 {
			t.Fatalf("want one intermediate progress update: %d", len(progress.days))
		}
		if day := <-progress.days; day.State != transactional.MoveOverStarted || day.Day != start.Format(time.DateOnly) {
			t.Fatalf("want intermediate progress for the current day: %+v", day)
		}
		select {
		case <-done:
			t.Fatal("the day should still be copying")
		default:
		}
		if err := <-done; err != nil {
			t.Fatal(err)
		}
		progress.err = errProgressUnavailable
		if _, err := m.moveDay(context.Background(), "endpoints", start, true); !errors.Is(err, progress.err) {
			t.Fatalf("a failed progress update must stop the copy: %v", err)
		}
	})
}
