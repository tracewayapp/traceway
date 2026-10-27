package slo

import (
	"testing"
	"time"
)

func TestPassDueRunsEverySLOOncePerIntervalSpreadOverTicks(t *testing.T) {
	start := time.Date(2026, 10, 27, 12, 0, 0, 0, time.UTC)
	const slos = 900
	runs := make([]int, slos)
	busiestTick := 0
	for minute := 1; minute <= 60; minute++ {
		since, now := start.Add(time.Duration(minute-1)*time.Minute), start.Add(time.Duration(minute)*time.Minute)
		due := 0
		for id := 1; id <= slos; id++ {
			if passDue(id, lateArrivalInterval, &since, now) {
				runs[id-1]++
				due++
			}
		}
		busiestTick = max(busiestTick, due)
	}
	for id, count := range runs {
		if count != 4 {
			t.Fatalf("SLO %d ran its 15-minute pass %d times in an hour, want 4", id+1, count)
		}
	}
	if busiestTick > slos/5 {
		t.Fatalf("%d of %d SLOs ran their pass on the same tick; the passes are not spread", busiestTick, slos)
	}
}

func TestPassDueAfterARestartDoesNotRunEveryPass(t *testing.T) {
	now := time.Date(2026, 10, 27, 12, 0, 30, 0, time.UTC)
	since := now.Add(-time.Minute)
	due := 0
	for id := 1; id <= 1000; id++ {
		if passDue(id, lateCrashInterval, &since, now) {
			due++
		}
	}
	if due > 5 {
		t.Fatalf("%d of 1000 daily passes ran on one tick after a restart", due)
	}
	if !passDue(1, lateCrashInterval, nil, now) {
		t.Fatal("an SLO that was never computed runs every pass")
	}
	longAgo := now.Add(-25 * time.Hour)
	if !passDue(1, lateCrashInterval, &longAgo, now) {
		t.Fatal("an SLO that was not computed for a whole interval runs its pass")
	}
}
