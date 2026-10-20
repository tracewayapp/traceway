package perf

import (
	"math"
	"reflect"
	"testing"
	"time"
	"unsafe"

	"github.com/tracewayapp/traceway/cli/test/evals/internal/scenarios"
	"github.com/tracewayapp/traceway/cli/test/evals/internal/seed"
)

func zzpctField2[T any](v any, name string) T {
	rv := reflect.ValueOf(v).Elem().FieldByName(name)
	return *(*T)(unsafe.Pointer(rv.UnsafeAddr()))
}

func TestZZPerfctlBucket(t *testing.T) {
	base := time.Date(2026, 10, 20, 0, 0, 0, 0, time.UTC)
	for _, step := range []time.Duration{15 * time.Minute, 20 * time.Minute} {
		bad, minPrev, minCur := 0, 1e9, 1e9
		for i := 0; i < 1440; i += 2 {
			now := base.Add(time.Duration(i) * time.Minute)
			s := seed.NewScenario("perf-ctl-alert-handoff", now, seed.NewDrySink())
			if err := scenarios.Run("perf/ctl-alert-handoff", s); err != nil {
				t.Fatal(err)
			}
			_ = s.Flush()
			var listing *seed.Endpoint
			for _, svc := range zzpctField2[[]*seed.Service](s, "services") {
				for _, e := range zzpctField2[[]*seed.Endpoint](svc, "endpoints") {
					if e.Name() == "GET /api/categories/{slug}/products" {
						listing = e
					}
				}
			}
			bucketMax := func(from, to time.Time) float64 {
				mx := 0.0
				for b := from.Truncate(step); b.Before(to); b = b.Add(step) {
					mx = math.Max(mx, listing.PercentileBetween(0.95, b, b.Add(step)).Seconds()*1000)
				}
				return mx
			}
			deploy, prev := s.Ago(48*time.Minute), s.Ago(30*time.Hour)
			p, c := bucketMax(prev, prev.Add(15*time.Minute)), bucketMax(deploy, deploy.Add(15*time.Minute))
			minPrev, minCur = math.Min(minPrev, p), math.Min(minCur, c)
			if p < 200 || c < 200 {
				bad++
			}
		}
		t.Logf("step %v bad=%d minPrev=%.0f minCur=%.0f", step, bad, minPrev, minCur)
	}
}
