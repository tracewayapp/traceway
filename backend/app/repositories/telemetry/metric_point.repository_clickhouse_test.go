//go:build telemetry_ch

package telemetry

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/tracewayapp/traceway/backend/app/models"
)

func TestMetricPointRepository_QueryTimeSeries_RateClampsBucketToRollupStep(t *testing.T) {
	setupTestDB(t)
	ctx := context.Background()
	projectId := uuid.New()
	base := time.Now().UTC().Truncate(time.Hour).Add(-4 * time.Hour)

	tags := map[string]string{"server_name": "web-1"}
	var points []models.MetricPoint
	for hour, value := range []float64{1000, 1600, 2200, 2800} {
		points = append(points, makeMetricPoint(projectId, "system.network.io", value, tags, base.Add(time.Duration(hour)*time.Hour)))
	}
	if err := MetricPointRepository.InsertAsync(ctx, points); err != nil {
		t.Fatalf("InsertAsync failed: %v", err)
	}

	result, err := MetricPointRepository.QueryTimeSeries(ctx, projectId, "system.network.io", base.Add(-4*24*time.Hour), base.Add(4*time.Hour), 15, "rate", nil, "", 0)
	if err != nil {
		t.Fatalf("QueryTimeSeries failed: %v", err)
	}
	all := result["__all__"]
	if len(all) != 3 {
		t.Fatalf("expected 3 hourly buckets, got %d: %+v", len(all), all)
	}
	for i, p := range all {
		if !p.Timestamp.Equal(base.Add(time.Duration(i+1) * time.Hour)) {
			t.Fatalf("expected the 15-minute request to widen to hourly buckets, got %+v", all)
		}
		assertApproxEqual(t, "hourly bucket", p.Value, 600.0/3600, 0.001)
	}
}
