package monitoring

import (
	"testing"
	"time"
)

func TestLogVolumeWindowAndIdleZero(t *testing.T) {
	var v logVolume
	start := time.Unix(100, 0)
	v.add(start, 10, 8, 2, 1, 1)
	v.add(start.Add(29*time.Second), 5, 5, 0, 0, 0)
	got := v.snapshot(start.Add(29 * time.Second))
	if got.received != 15 || got.accepted != 13 || got.dropped != 2 || got.failures != 1 || got.saturated != 1 {
		t.Fatalf("%+v", got)
	}
	got = v.snapshot(start.Add(30 * time.Second))
	if got.received != 5 || got.dropped != 0 {
		t.Fatalf("expired bucket: %+v", got)
	}
	if got = v.snapshot(start.Add(59 * time.Second)); got.received != 0 || got.accepted != 0 {
		t.Fatalf("idle: %+v", got)
	}
	v.add(start.Add(60*time.Second), 1, 1, 0, 0, 0)
	if got = v.snapshot(start.Add(60 * time.Second)); got.received != 1 {
		t.Fatalf("ring reuse: %+v", got)
	}
}
