package shared

import (
	"math"
	"time"
)

var (
	// ClickHouse partitions spans on a 32-bit DateTime and orders them on DateTime64(6).
	otelPartitionTimeMax = time.Date(2106, 2, 7, 6, 28, 15, 0, time.UTC)
	otelDisplayTimeMax   = time.Date(2299, 12, 31, 23, 59, 59, 999999000, time.UTC)
)

func OtelNanosToTime(nanos uint64) time.Time {
	return time.Unix(int64(nanos/uint64(time.Second)), int64(nanos%uint64(time.Second))).UTC()
}

// OtelStorageTimes derives the partition and ordering times from the exact source start, which is
// stored separately. A missing or unrepresentable start falls back to the ingest time and reports it.
func OtelStorageTimes(startUnixNano uint64, ingestedAt time.Time) (partition, display time.Time, fallback bool) {
	start := OtelNanosToTime(startUnixNano)
	if startUnixNano == 0 || start.After(otelPartitionTimeMax) {
		partition, fallback = ingestedAt.UTC(), true
	} else {
		partition = start
	}
	if startUnixNano == 0 || start.After(otelDisplayTimeMax) {
		return partition, partition, fallback
	}
	return partition, start, fallback
}

// OtelDuration is zero for a missing or reversed interval and saturates instead of overflowing.
func OtelDuration(startUnixNano, endUnixNano uint64) time.Duration {
	if endUnixNano <= startUnixNano {
		return 0
	}
	if difference := endUnixNano - startUnixNano; difference < math.MaxInt64 {
		return time.Duration(difference)
	}
	return time.Duration(math.MaxInt64)
}
