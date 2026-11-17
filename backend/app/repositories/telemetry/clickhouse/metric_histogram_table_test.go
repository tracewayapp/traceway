//go:build telemetry_ch

package clickhouse

import (
	"testing"
	"time"
)

// The table is chosen by how far back a read starts as well as by its length: an SLO
// backfills a 90 day window in 24 hour chunks, and the minute table only keeps 7 days.
func TestHistogramTableStaysInsideEachTableTTL(t *testing.T) {
	now := time.Now().UTC()
	day := 24 * time.Hour
	for _, tc := range []struct {
		name      string
		from, to  time.Time
		wantTable string
		wantGrain int
	}{
		{"the last day", now.Add(-day), now, "metric_histograms", 60},
		{"three days", now.Add(-3 * day), now, "metric_histograms", 60},
		{"a day three weeks back", now.Add(-21 * day), now.Add(-20 * day), "metric_histograms_1h", 3600},
		{"a day at the edge of the minute TTL", now.Add(-7*day - time.Minute), now.Add(-6 * day), "metric_histograms_1h", 3600},
		{"a month", now.Add(-30 * day), now, "metric_histograms_1h", 3600},
		{"a day four months back", now.Add(-120 * day), now.Add(-119 * day), "metric_histograms_1d", 86400},
		{"a year", now.Add(-365 * day), now, "metric_histograms_1d", 86400},
	} {
		t.Run(tc.name, func(t *testing.T) {
			table, grain := histogramTable(tc.from, tc.to)
			if table != tc.wantTable || grain != tc.wantGrain {
				t.Errorf("got %s at %ds, want %s at %ds", table, grain, tc.wantTable, tc.wantGrain)
			}
		})
	}
}
