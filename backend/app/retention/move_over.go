package retention

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"

	"github.com/tracewayapp/traceway/backend/app/config"
	"github.com/tracewayapp/traceway/backend/app/repositories/telemetry"
	traceway "go.tracewayapp.com"
)

const moveOverInterval = time.Hour

// Opt in on one instance: copying history competes with live ingest for telemetry I/O.
func startMoveOver(ctx context.Context, telemetryRetentionDays int) {
	cfg := config.Config
	enabled, err := strconv.ParseBool(strings.TrimSpace(cfg.V2MoveOver))
	if err != nil || !enabled {
		return
	}
	options := telemetry.MoveOverOptions{}
	if oldest := strings.TrimSpace(cfg.V2MoveOverOldest); oldest != "" {
		if options.Oldest, err = time.Parse(time.DateOnly, oldest); err != nil {
			log.Printf("[tracewaybackend] move-over: V2_MOVE_OVER_OLDEST %q is not a YYYY-MM-DD date, not starting", oldest)
			return
		}
	}
	go func() {
		defer traceway.Recover()
		runMoveOverSchedule(ctx, moveOverInterval, func() {
			current := moveOverOptionsForRun(options, telemetryRetentionDays, time.Now())
			if err := telemetry.RunMoveOver(ctx, current); err != nil && !errors.Is(err, context.Canceled) {
				log.Printf("[tracewaybackend] move-over: stopped, retrying in %s: %v", moveOverInterval, err)
				traceway.CaptureException(fmt.Errorf("V2 move-over stopped: %w", err))
			}
		})
	}()
}

func moveOverOptionsForRun(options telemetry.MoveOverOptions, retentionDays int, now time.Time) telemetry.MoveOverOptions {
	// Recompute for every pass: a failed migration can outlive the retention window it started with.
	if retentionDays > 0 {
		if kept := now.UTC().AddDate(0, 0, -retentionDays); kept.After(options.Oldest) {
			options.Oldest = kept
		}
	}
	return options
}

func runMoveOverSchedule(ctx context.Context, interval time.Duration, run func()) {
	for ctx.Err() == nil {
		run()
		timer := time.NewTimer(interval)
		select {
		case <-ctx.Done():
			timer.Stop()
			return
		case <-timer.C:
		}
	}
}
