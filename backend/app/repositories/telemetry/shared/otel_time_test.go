package shared

import (
	"math"
	"testing"
	"time"
)

func TestOtelStorageTimesPolicy(t *testing.T) {
	ingested := time.Date(2026, 9, 18, 12, 0, 0, 0, time.UTC)
	nanos := func(at time.Time) uint64 { return uint64(at.Unix())*uint64(time.Second) + uint64(at.Nanosecond()) }
	historical := time.Date(2019, 3, 4, 5, 6, 7, 123456789, time.UTC)
	afterPartitionRange := time.Date(2150, 1, 1, 0, 0, 0, 0, time.UTC)
	afterDisplayRange := time.Date(2400, 1, 1, 0, 0, 0, 0, time.UTC)
	for name, tc := range map[string]struct {
		start              uint64
		partition, display time.Time
		fallback           bool
	}{
		"missing start uses the ingest time":                     {0, ingested, ingested, true},
		"historical data keeps its own partition":                {nanos(historical), historical, historical, false},
		"the last representable partition second is kept":        {nanos(otelPartitionTimeMax), otelPartitionTimeMax, otelPartitionTimeMax, false},
		"past the partition range only the partition falls back": {nanos(afterPartitionRange), ingested, afterPartitionRange, true},
		"past the ordering range both fall back":                 {nanos(afterDisplayRange), ingested, ingested, true},
		"beyond signed nanoseconds does not wrap negative":       {math.MaxInt64 + 1000, ingested, OtelNanosToTime(math.MaxInt64 + 1000), true},
		"the largest unsigned value falls back":                  {math.MaxUint64, ingested, ingested, true},
	} {
		partition, display, fallback := OtelStorageTimes(tc.start, ingested)
		if !partition.Equal(tc.partition) || !display.Equal(tc.display) || fallback != tc.fallback {
			t.Errorf("%s: partition %s display %s fallback %v, want %s %s %v", name, partition, display, fallback, tc.partition, tc.display, tc.fallback)
		}
		if partition.Before(time.Unix(0, 0)) || partition.After(otelPartitionTimeMax) {
			t.Errorf("%s: partition time %s cannot be stored in a 32-bit DateTime", name, partition)
		}
	}
	if got := OtelNanosToTime(math.MaxInt64 + 1000); got.Year() != 2262 {
		t.Errorf("values beyond signed nanoseconds must convert forward in time, got %s", got)
	}
}

func TestOtelDurationPolicy(t *testing.T) {
	for name, tc := range map[string]struct {
		start, end uint64
		want       time.Duration
	}{
		"normal interval":   {1_000, 4_500, 3_500},
		"reversed interval": {4_500, 1_000, 0},
		"missing end":       {4_500, 0, 0},
		"zero length":       {7, 7, 0},
		"saturates":         {0, math.MaxUint64, time.Duration(math.MaxInt64)},
	} {
		if got := OtelDuration(tc.start, tc.end); got != tc.want {
			t.Errorf("%s: %d, want %d", name, got, tc.want)
		}
	}
}
