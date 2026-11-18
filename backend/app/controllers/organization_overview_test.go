package controllers

import (
	"math"
	"testing"
	"time"

	"github.com/tracewayapp/traceway/backend/v2/app/models"
	"github.com/tracewayapp/traceway/backend/v2/app/repositories/telemetry/shared"
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

func TestSumDeviceRates(t *testing.T) {
	now := time.Now().UTC()
	series := map[string][]models.TimeSeriesPoint{
		"eth0":  {{Timestamp: now, Value: 1_000}, {Timestamp: now.Add(time.Minute), Value: 7_000}},
		"eth1":  {{Timestamp: now, Value: 500}, {Timestamp: now.Add(time.Minute), Value: 1_100}},
		"reset": {{Timestamp: now, Value: 9_000}, {Timestamp: now.Add(time.Minute), Value: 10}},
		"fresh": {{Timestamp: now.Add(time.Minute), Value: 42}},
	}
	rate, ok := sumDeviceRates(series)
	if !ok || rate != 110 {
		t.Fatalf("rate = %v, %v, want 110 bytes/s (100 + 10 + 0 for the reset device)", rate, ok)
	}

	if _, ok := sumDeviceRates(map[string][]models.TimeSeriesPoint{"fresh": series["fresh"]}); ok {
		t.Fatal("a device with a single bucket reported a rate")
	}
	if _, ok := sumDeviceRates(nil); ok {
		t.Fatal("no devices reported a rate")
	}
}

func TestOtelFractionToPercent(t *testing.T) {
	cases := map[float64]float64{0: 0, 0.42: 42, 1: 100, 1.2: 1.2, 55: 55, 140: 100}
	for in, want := range cases {
		if got := otelFractionToPercent(in); math.Abs(got-want) > 1e-9 {
			t.Errorf("otelFractionToPercent(%v) = %v, want %v", in, got, want)
		}
	}
}

func TestNetworkRatesByServer(t *testing.T) {
	now := time.Now().UTC()
	sep := shared.GroupKeySeparator
	rates := networkRatesByServer(map[string][]models.TimeSeriesPoint{
		"eth0" + sep + "web-1":  {{Timestamp: now, Value: 1_000}, {Timestamp: now.Add(time.Minute), Value: 7_000}},
		"lo" + sep + "web-1":    {{Timestamp: now, Value: 10}, {Timestamp: now.Add(time.Minute), Value: 610}},
		"eth0" + sep + "web-2":  {{Timestamp: now, Value: 500}, {Timestamp: now.Add(time.Minute), Value: 1_100}},
		"fresh" + sep + "web-3": {{Timestamp: now.Add(time.Minute), Value: 42}},
		"__all__":               {{Timestamp: now, Value: 1}, {Timestamp: now.Add(time.Minute), Value: 2}},
	})
	if len(rates) != 2 {
		t.Fatalf("rates = %v, want web-1 and web-2 only", rates)
	}
	if rates["web-1"] != 110 {
		t.Fatalf("web-1 = %v, want 110 bytes/s (100 eth0 + 10 lo)", rates["web-1"])
	}
	if rates["web-2"] != 10 {
		t.Fatalf("web-2 = %v, want 10 bytes/s", rates["web-2"])
	}
}
