package clientmetrics

import (
	"context"
	"fmt"
	"maps"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
)

type memoryStore struct {
	mu      sync.Mutex
	entries map[GuardEntry]time.Time
	pruned  int64
}

func (s *memoryStore) SaveGuardEntries(_ context.Context, entries []GuardEntry) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.entries == nil {
		s.entries = make(map[GuardEntry]time.Time)
	}
	for _, entry := range entries {
		if _, ok := s.entries[entry]; !ok {
			s.entries[entry] = time.Now()
		}
	}
	return nil
}

func (s *memoryStore) LoadGuardEntries(_ context.Context, day int64, since time.Time) ([]GuardEntry, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []GuardEntry
	for entry, created := range s.entries {
		if (entry.Day == 0 || entry.Day == day) && created.After(since) {
			out = append(out, entry)
		}
	}
	return out, nil
}

func (s *memoryStore) PruneGuardEntries(_ context.Context, beforeDay int64) error {
	s.pruned = beforeDay
	return nil
}

func withResource(extra map[string]any) map[string]any {
	resource := maps.Clone(browserResource)
	maps.Copy(resource, extra)
	return resource
}

func TestResendOfAnArrivedPayloadIsCountedOnce(t *testing.T) {
	p, sink := newTestPipeline(Config{})
	projectId := uuid.New()
	now := time.Now()
	point := dataPoint{metric: "browser.web_vital.lcp", values: []float64{1200, 1300}}

	first := sdkPayload(t, withResource(map[string]any{"traceway.batch_id": "a1b2c3d4e5f60718"}), point)
	if result := p.IngestOTLP(projectId, first, Client{}, now); result.Accepted != 1 {
		t.Fatalf("first upload %+v", result)
	}
	resend := sdkPayload(t, withResource(map[string]any{"traceway.batch_id": "a1b2c3d4e5f60718", "traceway.retry": true}), point)
	if result := p.IngestOTLP(projectId, resend, Client{}, now); result.Accepted != 0 || result.Dropped != 1 {
		t.Fatalf("resend of an arrived payload %+v", result)
	}
	lost := sdkPayload(t, withResource(map[string]any{"traceway.batch_id": "ffffffffffffffff", "traceway.retry": true}), point)
	if result := p.IngestOTLP(projectId, lost, Client{}, now); result.Accepted != 1 {
		t.Fatalf("resend of a lost payload %+v", result)
	}
	// The same id under another project is another payload.
	if result := p.IngestOTLP(uuid.New(), resend, Client{}, now); result.Accepted != 1 {
		t.Fatalf("other project %+v", result)
	}
	_ = p.Flush(context.Background())
	if got := sink.histogram("browser.web_vital.lcp").Count; got != 6 {
		t.Errorf("count %v, want 6", got)
	}
	if stats := p.Snapshot(); stats.DuplicatePayloads != 1 || stats.Dropped[ReasonDuplicate] != 1 {
		t.Errorf("stats %+v", stats)
	}
}

func TestBatchFilterKeepsIdsForADay(t *testing.T) {
	start := time.Now()
	f := newBatchFilter(1, start)
	projectId := uuid.New()
	for i := 0; i < 5000; i++ {
		f.seen(projectId, fmt.Sprintf("id-%d", i), start)
	}
	later := start.Add(dedupGeneration + time.Hour)
	for i := 0; i < 5000; i++ {
		if !f.seen(projectId, fmt.Sprintf("id-%d", i), later) {
			t.Fatalf("id %d forgotten after one rotation", i)
		}
	}
	falsePositives := 0
	for i := 0; i < 5000; i++ {
		if f.seen(projectId, fmt.Sprintf("new-%d", i), later) {
			falsePositives++
		}
	}
	if falsePositives > 2 {
		t.Errorf("%d false positives in 5000", falsePositives)
	}
	if f.seen(projectId, "only-once", start) {
		t.Fatal("unknown id reported as seen")
	}
	if f.seen(projectId, "only-once", start.Add(3*dedupGeneration)); f.seen(uuid.New(), "only-once", start.Add(3*dedupGeneration)) {
		t.Error("another project's id reported as seen")
	}
}

func TestCountryComesFromTheServerOnly(t *testing.T) {
	p, sink := newTestPipeline(Config{})
	projectId := uuid.New()
	now := time.Now()
	req := sdkPayload(t, withResource(map[string]any{"country": "XX"}),
		dataPoint{metric: "browser.web_vital.lcp", attrs: map[string]string{"country": "YY"}, values: []float64{1000}})
	p.IngestOTLP(projectId, req, Client{Country: "DE"}, now)
	p.IngestOTLP(projectId, req, Client{Country: "not a code"}, now.Add(2*time.Minute))
	_ = p.Flush(context.Background())
	seen := map[string]bool{}
	for _, cell := range sink.histograms {
		seen[cell.Dims[DimensionCountry]] = true
	}
	if !seen["DE"] || !seen[""] || len(seen) != 2 {
		t.Errorf("countries %v", seen)
	}
}

func TestGlobalDimensionsAreTwoKeysWithCappedValues(t *testing.T) {
	p, sink := newTestPipeline(Config{})
	projectId := uuid.New()
	now := time.Now()
	for i := 0; i < customDimensionCap+10; i++ {
		req := sdkPayload(t, withResource(map[string]any{
			"custom.plan": fmt.Sprintf("plan-%d", i), "custom.arm": "b", "custom.zzz": "ignored", "custom.Bad Key": "x"}),
			dataPoint{metric: "browser.web_vital.lcp", values: []float64{1000}})
		p.IngestOTLP(projectId, req, Client{}, now)
	}
	_ = p.Flush(context.Background())
	plans := map[string]bool{}
	for _, cell := range sink.histograms {
		plans[cell.Dims["custom.plan"]] = true
		if cell.Dims["custom.arm"] != "b" {
			t.Errorf("custom.arm = %q", cell.Dims["custom.arm"])
		}
		for key := range cell.Dims {
			if key == "custom.zzz" || strings.Contains(key, "Bad") {
				t.Errorf("%s became a dimension", key)
			}
		}
	}
	if len(plans) != customDimensionCap+1 || !plans[Other] {
		t.Errorf("%d plan values, want %d with other", len(plans), customDimensionCap+1)
	}
}

func TestUserMetricsTakeNoPageDimension(t *testing.T) {
	p, sink := newTestPipeline(Config{})
	projectId := uuid.New()
	now := time.Now()
	req := sdkPayload(t, browserResource,
		dataPoint{metric: "browser.user", attrs: map[string]string{"url.path": "/checkout", "network.type": "4g"}, count: 1},
		dataPoint{metric: "browser.user.affected", attrs: map[string]string{"issue": "js_error", "url.path": "/checkout"}, count: 1},
		dataPoint{metric: "browser.user.affected", attrs: map[string]string{"issue": "made_up"}, count: 1},
		dataPoint{metric: "browser.web_vital.attribution", attrs: map[string]string{"vital": "lcp", "rating": "poor", "target": "img.hero", "url.path": "/checkout"}, count: 1},
		dataPoint{metric: "browser.web_vital.lcp", attrs: map[string]string{"target": "img.hero", "script": "x.js"}, values: []float64{1}},
	)
	if result := p.IngestOTLP(projectId, req, Client{}, now); result.Accepted != 5 {
		t.Fatalf("%+v", result)
	}
	_ = p.Flush(context.Background())
	issues := map[string]bool{}
	for _, point := range sink.points {
		switch point.Name {
		case "browser.user", "browser.user.affected":
			if _, ok := point.Tags["url.path"]; ok {
				t.Errorf("%s has a page dimension: %v", point.Name, point.Tags)
			}
			if _, ok := point.Tags["network.type"]; ok {
				t.Errorf("%s has a network dimension: %v", point.Name, point.Tags)
			}
			if point.Name == "browser.user.affected" {
				issues[point.Tags["issue"]] = true
			}
		case "browser.web_vital.attribution":
			if point.Tags["target"] != "img.hero" || point.Tags["url.path"] != "/checkout" {
				t.Errorf("attribution tags %v", point.Tags)
			}
		}
	}
	if !issues["js_error"] || !issues[Other] {
		t.Errorf("issues %v", issues)
	}
	for _, key := range []string{"target", "script"} {
		if _, ok := sink.histograms[0].Dims[key]; ok {
			t.Errorf("%s is accepted on a metric that does not list it", key)
		}
	}
}

func TestExemplarCarriesItsTarget(t *testing.T) {
	p, sink := newTestPipeline(Config{})
	req := sdkPayload(t, browserResource, dataPoint{metric: "browser.view.inp", attrs: map[string]string{"rating": "poor", "view_type": "soft"},
		values: []float64{900}, session: uuid.NewString(), target: "button#pay" + strings.Repeat("x", 200)})
	p.IngestOTLP(uuid.New(), req, Client{}, time.Now())
	_ = p.Flush(context.Background())
	if len(sink.exemplars) != 1 || !strings.HasPrefix(sink.exemplars[0].Target, "button#pay") || len(sink.exemplars[0].Target) != maxTargetLength {
		t.Fatalf("exemplars %+v", sink.exemplars)
	}
}

func TestCapsAreSharedBetweenInstancesAndSurviveARestart(t *testing.T) {
	store := &memoryStore{}
	projectId := uuid.New()
	now := time.Now()
	ctx := context.Background()
	send := func(p *Pipeline, from, to int) {
		for i := from; i < to; i++ {
			req := sdkPayload(t, withResource(map[string]any{"os.name": fmt.Sprintf("os-%d", i)}),
				dataPoint{metric: "browser.web_vital.lcp", values: []float64{1000}},
				dataPoint{metric: fmt.Sprintf("checkout.step_%d", i), attrs: map[string]string{"plan": "pro"}, values: []float64{1}})
			p.IngestOTLP(projectId, req, Client{}, now)
		}
	}
	a, _ := newTestPipeline(Config{Store: store})
	b, sinkB := newTestPipeline(Config{Store: store})
	// os.name allows 10 values a day. Instance A uses 6 of them.
	send(a, 0, 6)
	a.SyncState(ctx, now)
	b.SyncState(ctx, now)
	send(b, 6, 16)
	_ = b.Flush(ctx)
	values := map[string]bool{}
	for _, cell := range sinkB.histograms {
		if cell.Name == "browser.web_vital.lcp" {
			values[cell.Dims["os.name"]] = true
		}
	}
	if len(values) != 5 || !values[Other] {
		t.Errorf("instance B admitted %v: want 4 new values and other", values)
	}

	b.SyncState(ctx, now)
	restarted, sink := newTestPipeline(Config{Store: store})
	restarted.SyncState(ctx, now)
	send(restarted, 0, 3)
	send(restarted, 40, 42)
	_ = restarted.Flush(ctx)
	values = map[string]bool{}
	custom := map[string]bool{}
	for _, cell := range sink.histograms {
		if cell.Name == "browser.web_vital.lcp" {
			values[cell.Dims["os.name"]] = true
		} else {
			custom[cell.Name] = true
		}
	}
	if !values["os-0"] || !values["os-2"] || !values[Other] || len(values) != 4 {
		t.Errorf("after a restart %v: known values must stay, new ones fold", values)
	}
	if !custom["checkout.step_0"] || !custom["checkout.step_40"] {
		t.Errorf("custom metrics %v", custom)
	}

	// The next day starts every dimension again and prunes the store.
	tomorrow := now.Add(24 * time.Hour)
	restarted.SyncState(ctx, tomorrow)
	if store.pruned == 0 {
		t.Errorf("old days are not pruned")
	}
}

func TestStoredSeriesCountAgainstTheBudget(t *testing.T) {
	projectId := uuid.New()
	now := time.Now()
	stored := make([]map[string]string, 0, 8)
	for i := 0; i < 8; i++ {
		stored = append(stored, map[string]string{"url.path": fmt.Sprintf("/stored-%d", i), "service.version": "4.2.0", "os.name": "Android",
			"browser.name": "Chrome", "device.form_factor": "phone", "server_name": "shop"})
	}
	p, sink := newTestPipeline(Config{SeriesBudget: 8, HotSeries: func(context.Context, time.Time, int, int) ([]SeriesSet, error) {
		return []SeriesSet{{ProjectId: projectId, Name: "browser.web_vital.lcp", Dims: stored}}, nil
	}})
	p.SyncState(context.Background(), now)
	for _, path := range []string{"/stored-3", "/brand-new"} {
		req := sdkPayload(t, browserResource, dataPoint{metric: "browser.web_vital.lcp", attrs: map[string]string{"url.path": path}, values: []float64{1}})
		p.IngestOTLP(projectId, req, Client{}, now)
	}
	_ = p.Flush(context.Background())
	paths := map[string]bool{}
	for _, cell := range sink.histograms {
		paths[cell.Dims["url.path"]] = true
	}
	if !paths["/stored-3"] || !paths[Other] || paths["/brand-new"] {
		t.Errorf("paths %v: a stored series stays, a new one folds", paths)
	}
}

func TestAddressCapIsDividedByInstances(t *testing.T) {
	instances := 1
	p, _ := newTestPipeline(Config{IPObservationsPerMinute: 100, Instances: func() int { return instances }})
	projectId := uuid.New()
	now := time.Now()
	send := func(ip string) int {
		accepted := 0
		for i := 0; i < 100; i++ {
			req := sdkPayload(t, browserResource, dataPoint{metric: "browser.page_view", count: 1})
			accepted += p.IngestOTLP(projectId, req, Client{IP: ip}, now).Accepted
		}
		return accepted
	}
	if got := send("1.1.1.1"); got != 100 {
		t.Errorf("one instance let %d through, want 100", got)
	}
	instances = 4
	if got := send("2.2.2.2"); got != 25 {
		t.Errorf("four instances let %d through here, want 25", got)
	}
}
