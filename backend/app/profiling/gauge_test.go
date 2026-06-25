package profiling

import "testing"

func TestGaugeFromName(t *testing.T) {
	gauges := []string{"inuse_space", "inuse_objects", "goroutine", "goroutines", "threads", "heap_live", "in_use"}
	for _, n := range gauges {
		if !gaugeFromName(n) {
			t.Errorf("gaugeFromName(%q) = false, want true", n)
		}
	}
	counters := []string{"cpu", "alloc_space", "alloc_objects", "mutex", "block", "lock", "wall", "samples"}
	for _, n := range counters {
		if gaugeFromName(n) {
			t.Errorf("gaugeFromName(%q) = true, want false", n)
		}
	}
}
