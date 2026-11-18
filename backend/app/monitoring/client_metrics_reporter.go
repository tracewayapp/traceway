package monitoring

import (
	"context"
	"time"

	"github.com/tracewayapp/traceway/backend/v2/app/clientmetrics"
	"github.com/tracewayapp/traceway/backend/v2/app/traceway"
)

const clientMetricsReportInterval = 10 * time.Second

// StartClientMetricsReporter emits the aggregator's own numbers as
// traceway.client_metrics.*: what arrived, how many cells it holds, how long a flush
// took and what was dropped, by reason. Cells against the cap is the number to watch:
// it is what bounds memory.
func StartClientMetricsReporter(ctx context.Context, snapshot func() clientmetrics.HealthStats) {
	go func() {
		defer traceway.Recover()
		ticker := time.NewTicker(clientMetricsReportInterval)
		defer ticker.Stop()
		previous := snapshot()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				current := snapshot()
				traceway.CaptureMetric("traceway.client_metrics.requests.delta", float64(safeDeltaInt64(previous.Requests, current.Requests)))
				traceway.CaptureMetric("traceway.client_metrics.observations.delta", float64(safeDeltaInt64(previous.Observations, current.Observations)))
				traceway.CaptureMetric("traceway.client_metrics.folded_dimensions.delta", float64(safeDeltaInt64(previous.FoldedDimensions, current.FoldedDimensions)))
				traceway.CaptureMetric("traceway.client_metrics.cells", float64(current.Cells))
				traceway.CaptureMetric("traceway.client_metrics.flush_ms", float64(current.LastFlushMs))
				traceway.CaptureMetric("traceway.client_metrics.flush_rows", float64(current.LastFlushRows))
				for reason, total := range current.Dropped {
					if delta := safeDeltaInt64(previous.Dropped[reason], total); delta > 0 {
						traceway.CaptureMetricWithTags("traceway.client_metrics.dropped.delta", float64(delta), map[string]string{"reason": reason})
					}
				}
				previous = current
			}
		}
	}()
}
