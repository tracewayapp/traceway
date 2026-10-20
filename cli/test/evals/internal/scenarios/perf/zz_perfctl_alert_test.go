package perf

import (
	"fmt"
	"math"
	"reflect"
	"unsafe"
	"strings"
	"testing"
	"time"

	"github.com/tracewayapp/traceway/cli/test/evals/internal/scenarios"
	"github.com/tracewayapp/traceway/cli/test/evals/internal/seed"
)

func TestZZPerfctlAlert(t *testing.T) {
	base := time.Date(2026, 10, 20, 0, 3, 0, 0, time.UTC)
	hist := map[int]int{}
	minAP := 1e9
	minC03, minPrev := 1e9, 1e9
	for i := 0; i < 1440; i++ {
		now := base.Add(time.Duration(i) * time.Minute)
		s := seed.NewScenario("perf-ctl-alert-handoff", now, seed.NewDrySink())
		if err := scenarios.Run("perf/ctl-alert-handoff", s); err != nil {
			t.Fatal(err)
		}
		_ = s.Flush()
		var listing *seed.Endpoint
		for _, svc := range zzpctField[[]*seed.Service](s, "services") {
			for _, e := range zzpctField[[]*seed.Endpoint](svc, "endpoints") {
				if e.Name() == "GET /api/categories/{slug}/products" {
					listing = e
				}
			}
		}
		deploy := s.Ago(48 * time.Minute)
		first := -1
		var row []string
		for m := 1; m <= 15; m++ {
			at := deploy.Add(time.Duration(m) * time.Minute)
			p := math.Round(listing.PercentileBetween(0.95, at.Add(-10*time.Minute), at).Seconds() * 1000)
			row = append(row, fmt.Sprint(p))
			if first < 0 && p >= 1000 {
				first = m
			}
		}
		hist[first]++
		bucketMax := func(from, to time.Time, step time.Duration) float64 {
			mx := 0.0
			for b := from.Truncate(step); b.Before(to); b = b.Add(step) {
				mx = math.Max(mx, listing.PercentileBetween(0.95, b, b.Add(step)).Seconds()*1000)
			}
			return mx
		}
		c03 := bucketMax(deploy, deploy.Add(15*time.Minute), 5*time.Minute)
		prev := s.Ago(30 * time.Hour)
		c09prev := bucketMax(prev, prev.Add(15*time.Minute), 30*time.Minute)
		c09 := bucketMax(deploy, deploy.Add(15*time.Minute), 30*time.Minute)
		if c03 < 1000 || c09prev < 200 || c09 < 200 {
			t.Logf("%s c03=%.0f c09prev=%.0f c09=%.0f", now.Format("15:04"), c03, c09prev, c09)
		}
		if c03 < minC03 {
			minC03 = c03
		}
		if c09prev < minPrev {
			minPrev = c09prev
		}
		ap := s.Exposed()["alertP95"].(float64)
		if ap < minAP {
			minAP = ap
		}
		if sr, ok := s.Exposed()["slowRequest"].(map[string]any); !ok || sr == nil {
			t.Logf("no slowRequest at %s", now)
		}
		if ap < 1000 || first < 0 || first > 5 {
			t.Logf("%s first=%d %s", now.Format("15:04"), first, strings.Join(row, " "))
		}
	}
	t.Logf("hist %v minAlertP95 %.0f minC03 %.0f minPrev30m %.0f", hist, minAP, minC03, minPrev)
}

func zzpctField[T any](v any, name string) T {
	rv := reflect.ValueOf(v).Elem().FieldByName(name)
	return *(*T)(unsafe.Pointer(rv.UnsafeAddr()))
}
