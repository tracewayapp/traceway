package controllers

import (
	"testing"
	"time"

	"github.com/tracewayapp/traceway/backend/app/models"
)

func TestCpuUsageTrendConvertsIdleFraction(t *testing.T) {
	now := time.Now().UTC()
	trend := cpuUsageTrend([]models.TimeSeriesPoint{
		{Timestamp: now, Value: 0.82},
		{Timestamp: now.Add(time.Minute), Value: 0.35},
	})
	if len(trend) != 2 || trend[0].Value != 18 || trend[1].Value != 65 {
		t.Fatalf("cpu trend = %+v, want 18%% then 65%%", trend)
	}
}

func TestLatestCounterRate(t *testing.T) {
	now := time.Now().UTC()
	rate, ok := latestCounterRate([]models.TimeSeriesPoint{
		{Timestamp: now, Value: 1_000},
		{Timestamp: now.Add(time.Minute), Value: 7_000},
	})
	if !ok || rate != 100 {
		t.Fatalf("rate = %v, %v, want 100 bytes/s", rate, ok)
	}

	rate, ok = latestCounterRate([]models.TimeSeriesPoint{
		{Timestamp: now, Value: 7_000},
		{Timestamp: now.Add(time.Minute), Value: 50},
	})
	if !ok || rate != 0 {
		t.Fatalf("reset rate = %v, %v, want 0 bytes/s", rate, ok)
	}
}
