package controllers

import (
	"github.com/tracewayapp/traceway/backend/v2/app/models"
	"testing"
	"time"
)

func TestWholeRangePointsAcrossAnEpochBucketBoundary(t *testing.T) {
	from := time.Date(2026, 11, 16, 12, 4, 0, 0, time.UTC)
	a, b := from.Add(-4*time.Minute), from.Add(time.Minute)
	counts := map[string][]models.TimeSeriesPoint{"__all__": {{Timestamp: a, Value: 9}, {Timestamp: b, Value: 3}}}
	for agg, want := range map[string]float64{"sum": 40, "count": 40, "min": 10, "max": 30, "last": 30, "avg": 15, "rate": 50} {
		series := map[string][]models.TimeSeriesPoint{"__all__": {{Timestamp: a, Value: 10}, {Timestamp: b, Value: 30}}}
		result := wholeRangePoints(series, counts, agg, from, from.Add(4*time.Minute), 5)
		points := result["__all__"]
		if len(points) != 1 || points[0].Value != want || !points[0].Timestamp.Equal(from) {
			t.Errorf("%s: %+v, want %v at %v", agg, points, want, from)
		}
	}
}
