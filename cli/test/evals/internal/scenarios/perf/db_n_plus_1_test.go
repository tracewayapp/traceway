package perf

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/tracewayapp/traceway/cli/test/evals/internal/env"
	"github.com/tracewayapp/traceway/cli/test/evals/internal/scenarios"
	"github.com/tracewayapp/traceway/cli/test/evals/internal/seed"
	client "github.com/tracewayapp/traceway/cli/v2/pkg/client"
)

// TestSeedNPlusOne is the Phase 2 acceptance check: the slow request's
// waterfall has at least 40 order_items spans, every parent covers its
// children, and the listed percentiles match what the DSL generated.
func TestSeedNPlusOne(t *testing.T) {
	bins := env.BinariesForTest(t)
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Minute)
	defer cancel()
	inst, err := env.NewInstance(ctx, bins, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer inst.Close()

	sink := seed.NewLiveSink(ctx, inst.Backend, inst.Admin)
	s := seed.NewScenario("perf-db-n-plus-1", time.Now(), sink)
	if err := scenarios.Run("perf/db-n-plus-1", s); err != nil {
		t.Fatal(err)
	}
	if err := s.Verify(time.Minute); err != nil {
		t.Fatal(err)
	}
	t.Logf("seed stats: %+v", s.Stats())
	ex := s.Exposed()
	slow := ex["slowRequest"].(map[string]any)
	pid := s.Projects()[0].ID
	api := inst.Backend.API().WithBearer(inst.Admin.JWT)

	at, _ := time.Parse(time.RFC3339Nano, slow["recordedAt"].(string))
	c := client.New(inst.Backend.URL, client.WithJWT(inst.Admin.JWT))
	detail, err := c.GetEndpoint(ctx, pid, slow["id"].(string), at)
	if err != nil {
		t.Fatal(err)
	}
	items := 0
	children := map[string]int64{}
	byID := map[string]int64{}
	for _, sp := range detail.Spans {
		byID[sp.SpanId] = int64(sp.Duration)
		if sp.ParentSpanId != "" {
			children[sp.ParentSpanId] += int64(sp.Duration)
		}
		if sp.Name == "SELECT order_items" {
			items++
		}
	}
	if items < 40 {
		t.Errorf("slowest request has %d order_items spans, want >= 40 (spans: %d)", items, len(detail.Spans))
	}
	for parent, sum := range children {
		if d, ok := byID[parent]; ok && d < sum {
			t.Errorf("span %s lasts %d but its children sum to %d", parent, d, sum)
		}
	}

	from, to := s.Now.Add(-24*time.Hour), s.Now.Add(time.Minute)
	lres, err := api.MustOK(ctx, http.MethodPost, "/api/endpoints/grouped", env.ProjectQuery(pid), map[string]any{
		"fromDate": from.Format(time.RFC3339), "toDate": to.Format(time.RFC3339), "orderBy": "p95",
		"search": "GET /api/orders", "pagination": map[string]int{"page": 1, "pageSize": 20},
	})
	if err != nil {
		t.Fatal(err)
	}
	var list struct {
		Data []struct {
			Endpoint    string `json:"endpoint"`
			Count       int    `json:"count"`
			P50Duration int64  `json:"p50Duration"`
			P95Duration int64  `json:"p95Duration"`
		} `json:"data"`
	}
	if err := json.Unmarshal(lres.Body, &list); err != nil {
		t.Fatal(err)
	}
	var found bool
	for _, d := range list.Data {
		if d.Endpoint != "GET /api/orders" {
			continue
		}
		found = true
		p95 := float64(d.P95Duration) / 1e6
		want := ex["p95Ms24h"].(float64)
		if p95 < want*0.85 || p95 > want*1.15 {
			t.Errorf("listed p95 %.0fms, generated %.0fms", p95, want)
		}
		if p95 < 500 || p95 > 2500 {
			t.Errorf("p95 %.0fms is outside the designed 0.5-2.5s range", p95)
		}
		t.Logf("GET /api/orders 24h: count=%d p50=%.0fms p95=%.0fms (first week p95 %.0fms)", d.Count, float64(d.P50Duration)/1e6, p95, ex["p95MsFirstWeek"].(float64))
	}
	if !found {
		t.Fatalf("GET /api/orders not listed: %s", lres.Body)
	}
	for _, sp := range detail.Spans {
		if strings.Contains(strings.ToLower(sp.Name), "n+1") {
			t.Error("seeded data names the answer")
		}
	}
}
