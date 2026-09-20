package retention

import (
	"context"
	"testing"
	"testing/synctest"
	"time"

	"github.com/tracewayapp/traceway/backend/app/repositories/telemetry"
)

func TestMoveOverScheduleWaitsAfterPassAndStopsOnCancellation(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		starts := make(chan time.Time, 8)
		start := time.Now()
		done := make(chan struct{})
		go func() {
			defer close(done)
			runMoveOverSchedule(ctx, time.Hour, func() {
				starts <- time.Now()
				time.Sleep(2 * time.Hour)
			})
		}()
		synctest.Wait()
		if len(starts) != 1 {
			t.Fatalf("first pass must start immediately: %d passes", len(starts))
		}
		if got := <-starts; !got.Equal(start) {
			t.Fatalf("first pass started at %v, want %v", got, start)
		}
		time.Sleep(3 * time.Hour)
		synctest.Wait()
		if len(starts) != 1 {
			t.Fatalf("passes must not overlap or queue while the previous pass runs: %d new passes", len(starts))
		}
		if got := <-starts; !got.Equal(start.Add(3 * time.Hour)) {
			t.Fatalf("second pass started at %v, want %v", got, start.Add(3*time.Hour))
		}
		cancel()
		<-done
		time.Sleep(2 * time.Hour)
		if len(starts) != 0 {
			t.Fatalf("cancelled worker ran again: %d passes", len(starts))
		}
	})
}

func TestMoveOverScheduleDoesNotStartWhenCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	runMoveOverSchedule(ctx, time.Hour, func() { t.Fatal("cancelled worker started") })
}

func TestMoveOverRecomputesRetentionForEachPass(t *testing.T) {
	now := time.Date(2026, 9, 20, 1, 0, 0, 0, time.FixedZone("UTC+2", 2*60*60))
	options := telemetry.MoveOverOptions{Oldest: now.AddDate(0, 0, -90), PageSize: 25}
	for _, elapsed := range []int{0, 10} {
		current := now.AddDate(0, 0, elapsed)
		got := moveOverOptionsForRun(options, 30, current)
		if !got.Oldest.Equal(current.UTC().AddDate(0, 0, -30)) || got.Oldest.Location() != time.UTC || got.PageSize != 25 {
			t.Fatalf("retention window at day %d: %+v", elapsed, got)
		}
	}
	if got := moveOverOptionsForRun(options, 0, now); !got.Oldest.Equal(options.Oldest) {
		t.Fatal("disabled retention must preserve the operator's lower bound")
	}
	options.Oldest = now.AddDate(0, 0, -2)
	if got := moveOverOptionsForRun(options, 30, now); !got.Oldest.Equal(options.Oldest) {
		t.Fatal("a newer operator lower bound must win")
	}
}
