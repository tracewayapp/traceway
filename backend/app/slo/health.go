package slo

import (
	"sync/atomic"
	"time"
)

var (
	computedTotal uint64
	errorsTotal   uint64
	lastTickUnix  atomic.Int64
)

type HealthStats struct {
	ComputedTotal uint64     `json:"computedTotal"`
	ErrorsTotal   uint64     `json:"errorsTotal"`
	LastTickAt    *time.Time `json:"lastTickAt"`
}

func HealthSnapshot() *HealthStats {
	stats := &HealthStats{
		ComputedTotal: atomic.LoadUint64(&computedTotal),
		ErrorsTotal:   atomic.LoadUint64(&errorsTotal),
	}
	if unix := lastTickUnix.Load(); unix > 0 {
		at := time.Unix(unix, 0).UTC()
		stats.LastTickAt = &at
	}
	return stats
}
