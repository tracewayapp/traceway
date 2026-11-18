package clientmetrics

import (
	"context"
	"fmt"
	"math"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/tracewayapp/traceway/backend/v2/app/models"
	"github.com/tracewayapp/traceway/backend/v2/app/repositories/telemetry/shared"
	colmetricspb "go.opentelemetry.io/proto/otlp/collector/metrics/v1"
	"google.golang.org/protobuf/encoding/protojson"
)

type memorySink struct {
	mu         sync.Mutex
	histograms []shared.MetricHistogramCell
	points     []models.MetricPoint
	exemplars  []shared.MetricExemplar
	writes     int
	fail       error
}

func (s *memorySink) WriteHistograms(_ context.Context, cells []shared.MetricHistogramCell) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.writes++
	if s.fail != nil {
		return s.fail
	}
	s.histograms = append(s.histograms, cells...)
	return nil
}

func (s *memorySink) WritePoints(_ context.Context, points []models.MetricPoint) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.points = append(s.points, points...)
	return nil
}

func (s *memorySink) WriteExemplars(_ context.Context, exemplars []shared.MetricExemplar) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.exemplars = append(s.exemplars, exemplars...)
	return nil
}

func (s *memorySink) histogram(name string) *shared.Histogram {
	merged := shared.NewHistogram()
	for _, cell := range s.histograms {
		if cell.Name == name {
			merged.Merge(cell.Hist)
		}
	}
	return merged
}

func (s *memorySink) sum(name string) float64 {
	var total float64
	for _, point := range s.points {
		if point.Name == name {
			total += point.Value
		}
	}
	return total
}

type dataPoint struct {
	metric  string
	attrs   map[string]string
	values  []float64
	count   float64
	session string
	target  string
	timeMs  int64
}

// sdkPayload renders what js-client's encodeMetrics produces, as JSON, so the test
// goes through the same decoder as a real upload.
func sdkPayload(t *testing.T, resource map[string]any, points ...dataPoint) *colmetricspb.ExportMetricsServiceRequest {
	t.Helper()
	kv := func(attrs map[string]string) string {
		parts := make([]string, 0, len(attrs))
		for _, key := range sortedKeys(attrs) {
			parts = append(parts, fmt.Sprintf(`{"key":%q,"value":{"stringValue":%q}}`, key, attrs[key]))
		}
		return "[" + strings.Join(parts, ",") + "]"
	}
	var resourceAttrs []string
	for key, value := range resource {
		switch v := value.(type) {
		case string:
			resourceAttrs = append(resourceAttrs, fmt.Sprintf(`{"key":%q,"value":{"stringValue":%q}}`, key, v))
		case float64:
			resourceAttrs = append(resourceAttrs, fmt.Sprintf(`{"key":%q,"value":{"doubleValue":%v}}`, key, v))
		case bool:
			resourceAttrs = append(resourceAttrs, fmt.Sprintf(`{"key":%q,"value":{"boolValue":%v}}`, key, v))
		}
	}
	var metrics []string
	for _, point := range points {
		at := point.timeMs
		if at == 0 {
			at = time.Now().UnixMilli()
		}
		times := fmt.Sprintf(`"startTimeUnixNano":"%d000000","timeUnixNano":"%d000000"`, at-1000, at)
		if point.values == nil {
			metrics = append(metrics, fmt.Sprintf(`{"name":%q,"sum":{"aggregationTemporality":1,"isMonotonic":true,"dataPoints":[{"attributes":%s,%s,"asDouble":%v}]}}`,
				point.metric, kv(point.attrs), times, point.count))
			continue
		}
		h := shared.NewHistogram()
		for _, v := range point.values {
			h.Observe(v, 1)
		}
		indexes, counts := h.BucketArrays()
		dense := make([]string, indexes[len(indexes)-1]-indexes[0]+1)
		for i := range dense {
			dense[i] = "0"
		}
		for i, index := range indexes {
			dense[index-indexes[0]] = fmt.Sprintf("%v", counts[i])
		}
		exemplars := ""
		if point.session != "" {
			target := ""
			if point.target != "" {
				target = fmt.Sprintf(`,{"key":"traceway.target","value":{"stringValue":%q}}`, point.target)
			}
			exemplars = fmt.Sprintf(`,"exemplars":[{"timeUnixNano":"%d000000","asDouble":%v,"filteredAttributes":[{"key":"session.id","value":{"stringValue":%q}}%s]}]`, at, h.Max, point.session, target)
		}
		metrics = append(metrics, fmt.Sprintf(`{"name":%q,"unit":"ms","exponentialHistogram":{"aggregationTemporality":1,"dataPoints":[{"attributes":%s,%s,"count":%v,"sum":%v,"scale":3,"zeroCount":%v,"min":%v,"max":%v,"positive":{"offset":%d,"bucketCounts":[%s]}%s}]}}`,
			point.metric, kv(point.attrs), times, h.Count, h.Sum, h.Zero, h.Min, h.Max, indexes[0], strings.Join(dense, ","), exemplars))
	}
	body := fmt.Sprintf(`{"resourceMetrics":[{"resource":{"attributes":[%s]},"scopeMetrics":[{"scope":{"name":"@tracewayapp/frontend","version":"2.1.0"},"metrics":[%s]}]}]}`,
		strings.Join(resourceAttrs, ","), strings.Join(metrics, ","))
	req := &colmetricspb.ExportMetricsServiceRequest{}
	if err := (protojson.UnmarshalOptions{DiscardUnknown: true}).Unmarshal([]byte(body), req); err != nil {
		t.Fatalf("payload does not decode as OTLP: %v\n%s", err, body)
	}
	return req
}

var browserResource = map[string]any{
	"service.name": "shop", "service.version": "4.2.0", "os.name": "Android", "browser.name": "Chrome",
	"device.form_factor": "phone", "traceway.metrics": "aggregated", "traceway.sample_rate": 1.0,
}

func newTestPipeline(cfg Config) (*Pipeline, *memorySink) {
	sink := &memorySink{}
	return New(sink, cfg), sink
}

func TestManyClientsBecomeOneCell(t *testing.T) {
	p, sink := newTestPipeline(Config{})
	projectId := uuid.New()
	now := time.Now()
	for client := 0; client < 500; client++ {
		req := sdkPayload(t, browserResource,
			dataPoint{metric: "browser.web_vital.lcp", attrs: map[string]string{"url.path": "/orders/:id", "rating": "good", "navigation_type": "navigate"}, values: []float64{1000 + float64(client)}},
			dataPoint{metric: "browser.page_view", attrs: map[string]string{"url.path": "/orders/:id"}, count: 3},
		)
		if result := p.IngestOTLP(projectId, req, Client{IP: fmt.Sprintf("10.0.%d.%d", client/250, client%250)}, now); result.Accepted != 2 || result.Dropped != 0 {
			t.Fatalf("client %d: %+v", client, result)
		}
	}
	if sink.writes != 0 {
		t.Fatal("a request wrote to storage")
	}
	if cells := p.Snapshot().Cells; cells > 4 {
		t.Fatalf("500 clients hold %d cells; cells must not grow with clients", cells)
	}
	if err := p.Flush(context.Background()); err != nil {
		t.Fatal(err)
	}
	h := sink.histogram("browser.web_vital.lcp")
	if h.Count != 500 || len(sink.histograms) > 2 {
		t.Fatalf("count %v in %d rows", h.Count, len(sink.histograms))
	}
	if p75 := h.Quantile(0.75); math.Abs(p75-1375)/1375 > 0.05 {
		t.Errorf("p75 = %v, want about 1375", p75)
	}
	if got := sink.sum("browser.page_view"); got != 1500 {
		t.Errorf("page views = %v, want 1500", got)
	}
	dims := sink.histograms[0].Dims
	if dims["service.version"] != "4.2.0" || dims["server_name"] != "shop" || dims["rating"] != "good" || dims["url.path"] != "/orders/:id" {
		t.Errorf("dims %v", dims)
	}
	if _, leaked := dims["traceway.sample_rate"]; leaked {
		t.Errorf("a control attribute became a dimension: %v", dims)
	}
}

func TestGuardRefusesWhatAForgedClientCouldSend(t *testing.T) {
	p, sink := newTestPipeline(Config{})
	projectId := uuid.New()
	now := time.Now()
	req := sdkPayload(t, browserResource,
		dataPoint{metric: "browser.web_vital.lcp", attrs: map[string]string{
			"url.path": "/orders/8841/items/3f2b8c1e-9a4d-4c7e-8b1a-2d3e4f5a6b7c", "rating": "excellent", "session.id": "abc", "user.id": "42"}, values: []float64{10_800_000}},
		dataPoint{metric: "browser.made_up", values: []float64{1}},
		dataPoint{metric: "browser.page_view", attrs: map[string]string{"url.path": "/"}, values: []float64{1}},
	)
	result := p.IngestOTLP(projectId, req, Client{IP: "10.0.0.1"}, now)
	if result.Accepted != 1 || result.Dropped != 2 {
		t.Fatalf("%+v", result)
	}
	_ = p.Flush(context.Background())
	cell := sink.histograms[0]
	if cell.Dims["url.path"] != "/orders/:id/items/:id" || cell.Dims["rating"] != Other {
		t.Errorf("dims %v", cell.Dims)
	}
	for _, key := range []string{"session.id", "user.id"} {
		if _, ok := cell.Dims[key]; ok {
			t.Errorf("%s became a dimension", key)
		}
	}
	if cell.Hist.Max != 2*minute {
		t.Errorf("a three hour LCP was kept as %v", cell.Hist.Max)
	}
	stats := p.Snapshot()
	if stats.Dropped[ReasonUnknownMetric] != 1 || stats.Dropped[ReasonUnsupported] != 1 {
		t.Errorf("dropped %v", stats.Dropped)
	}
}

func TestDimensionCapFoldsToOther(t *testing.T) {
	p, sink := newTestPipeline(Config{})
	projectId := uuid.New()
	now := time.Now()
	for i := 0; i < 260; i++ {
		req := sdkPayload(t, browserResource, dataPoint{metric: "browser.route_timing.duration", attrs: map[string]string{"url.path": fmt.Sprintf("/page-%c%c/x", 'a'+i/26, 'a'+i%26)}, values: []float64{100}})
		p.IngestOTLP(projectId, req, Client{}, now)
	}
	_ = p.Flush(context.Background())
	paths := map[string]float64{}
	for _, cell := range sink.histograms {
		paths[cell.Dims["url.path"]] += cell.Hist.Count
	}
	if len(paths) != 201 || paths[Other] != 60 {
		t.Fatalf("%d distinct paths, %v in other", len(paths), paths[Other])
	}
	if p.Snapshot().FoldedDimensions != 60 {
		t.Errorf("folded %d", p.Snapshot().FoldedDimensions)
	}
}

func TestSeriesBudgetFoldsTheViewFirst(t *testing.T) {
	p, sink := newTestPipeline(Config{SeriesBudget: 10})
	projectId := uuid.New()
	now := time.Now()
	for i := 0; i < 40; i++ {
		req := sdkPayload(t, map[string]any{"service.version": fmt.Sprintf("1.%d", i%4)},
			dataPoint{metric: "browser.route_timing.duration", attrs: map[string]string{"url.path": fmt.Sprintf("/view-%d-x", i)}, values: []float64{100}})
		p.IngestOTLP(projectId, req, Client{}, now)
	}
	_ = p.Flush(context.Background())
	series := map[string]struct{}{}
	var total float64
	for _, cell := range sink.histograms {
		series[canonicalDims(cell.Dims)] = struct{}{}
		total += cell.Hist.Count
	}
	if len(series) > 14 || total != 40 {
		t.Fatalf("%d series holding %v observations", len(series), total)
	}

	// A forged client varying every dimension it can is stopped at twice the budget.
	for i := 0; i < 200; i++ {
		req := sdkPayload(t, map[string]any{"service.version": fmt.Sprintf("9.%d", i%50), "os.major": fmt.Sprintf("%d", i%30), "browser.major": fmt.Sprintf("%d", i%60)},
			dataPoint{metric: "browser.route_timing.duration", attrs: map[string]string{"url.path": fmt.Sprintf("/z-%d-x", i)}, values: []float64{100}})
		p.IngestOTLP(projectId, req, Client{}, now)
	}
	sink.histograms = nil
	_ = p.Flush(context.Background())
	if p.Snapshot().Dropped[ReasonCardinality] == 0 {
		t.Error("nothing was refused past the series budget")
	}
	for _, cell := range sink.histograms {
		series[canonicalDims(cell.Dims)] = struct{}{}
	}
	if len(series) > 20 {
		t.Errorf("%d series for a budget of 10", len(series))
	}
}

func TestCustomMetricLimits(t *testing.T) {
	p, sink := newTestPipeline(Config{})
	projectId := uuid.New()
	now := time.Now()
	for i := 0; i < MaxCustomMetrics+5; i++ {
		req := sdkPayload(t, browserResource, dataPoint{metric: fmt.Sprintf("checkout.step_%d", i), attrs: map[string]string{"a": "1", "b": "2", "c": "3"}, values: []float64{5}})
		p.IngestOTLP(projectId, req, Client{}, now)
	}
	p.IngestOTLP(projectId, sdkPayload(t, browserResource, dataPoint{metric: "Bad Name", values: []float64{5}}), Client{}, now)
	_ = p.Flush(context.Background())
	names := map[string]struct{}{}
	for _, cell := range sink.histograms {
		names[cell.Name] = struct{}{}
		if _, third := cell.Dims["c"]; third || cell.Dims["a"] != "1" || cell.Dims["b"] != "2" {
			t.Fatalf("custom dims %v", cell.Dims)
		}
	}
	if len(names) != MaxCustomMetrics {
		t.Errorf("%d custom metrics, want %d", len(names), MaxCustomMetrics)
	}
	if got := p.Snapshot().Dropped[ReasonUnknownMetric]; got != 6 {
		t.Errorf("dropped %d as unknown, want 6", got)
	}
}

func TestSampleRateWeightsCounts(t *testing.T) {
	p, sink := newTestPipeline(Config{})
	projectId := uuid.New()
	resource := map[string]any{"traceway.sample_rate": 0.1}
	req := sdkPayload(t, resource,
		dataPoint{metric: "browser.web_vital.inp", values: []float64{100, 200}},
		dataPoint{metric: "browser.js_error", count: 4})
	p.IngestOTLP(projectId, req, Client{}, time.Now())
	_ = p.Flush(context.Background())
	if h := sink.histogram("browser.web_vital.inp"); math.Abs(h.Count-20) > 1e-9 || math.Abs(h.Sum-3000) > 1e-6 {
		t.Errorf("weighted histogram count %v sum %v", h.Count, h.Sum)
	}
	if got := sink.sum("browser.js_error"); math.Abs(got-40) > 1e-9 {
		t.Errorf("weighted counter = %v, want 40", got)
	}
}

func TestClockCorrectionAndLateSamples(t *testing.T) {
	p, sink := newTestPipeline(Config{})
	projectId := uuid.New()
	now := time.Now()
	// The device clock is two days ahead; it sends data it measured an hour ago.
	deviceNow := now.Add(48 * time.Hour)
	skewed := map[string]any{"traceway.sent_at": float64(deviceNow.UnixMilli())}
	req := sdkPayload(t, skewed, dataPoint{metric: "app.start.duration", attrs: map[string]string{"start_type": "cold"}, values: []float64{900}, timeMs: deviceNow.Add(-time.Hour).UnixMilli()})
	if result := p.IngestOTLP(projectId, req, Client{}, now); result.Accepted != 1 {
		t.Fatalf("%+v", result)
	}
	old := sdkPayload(t, map[string]any{}, dataPoint{metric: "app.start.duration", values: []float64{900}, timeMs: now.Add(-100 * time.Hour).UnixMilli()})
	future := sdkPayload(t, map[string]any{}, dataPoint{metric: "app.start.duration", values: []float64{900}, timeMs: now.Add(time.Hour).UnixMilli()})
	p.IngestOTLP(projectId, old, Client{}, now)
	p.IngestOTLP(projectId, future, Client{}, now)
	_ = p.Flush(context.Background())
	if len(sink.histograms) != 1 {
		t.Fatalf("%d cells", len(sink.histograms))
	}
	want := now.Add(-time.Hour).Truncate(time.Minute)
	if got := sink.histograms[0].RecordedAt; got.Sub(want).Abs() > time.Minute {
		t.Errorf("recorded at %v, want about %v", got, want)
	}
	if got := p.Snapshot().Dropped[ReasonLate]; got != 2 {
		t.Errorf("late = %d, want 2", got)
	}
}

func TestOneAddressCannotMoveAPercentile(t *testing.T) {
	p, sink := newTestPipeline(Config{IPObservationsPerMinute: 100})
	projectId := uuid.New()
	now := time.Now()
	flood := make([]float64, 90)
	for i := range flood {
		flood[i] = 60_000
	}
	for i := 0; i < 20; i++ {
		p.IngestOTLP(projectId, sdkPayload(t, browserResource, dataPoint{metric: "browser.web_vital.lcp", values: flood}), Client{IP: "6.6.6.6"}, now)
	}
	for i := 0; i < 50; i++ {
		p.IngestOTLP(projectId, sdkPayload(t, browserResource, dataPoint{metric: "browser.web_vital.lcp", values: []float64{1000}}), Client{IP: fmt.Sprintf("10.1.1.%d", i)}, now)
	}
	_ = p.Flush(context.Background())
	if h := sink.histogram("browser.web_vital.lcp"); h.Count != 140 {
		t.Errorf("count %v, want 90 from the flood and 50 from everyone else", h.Count)
	}
	if got := p.Snapshot().Dropped[ReasonRateLimit]; got != 19 {
		t.Errorf("rate limited %d, want 19", got)
	}
}

func TestMemoryCapDropsNewCellsAndKeepsCounting(t *testing.T) {
	p, sink := newTestPipeline(Config{MaxCells: 5})
	now := time.Now()
	for i := 0; i < 20; i++ {
		p.IngestOTLP(uuid.New(), sdkPayload(t, browserResource, dataPoint{metric: "browser.web_vital.lcp", values: []float64{1000}}), Client{}, now)
	}
	stats := p.Snapshot()
	if stats.Cells != 5 || stats.Dropped[ReasonMemory] != 15 {
		t.Fatalf("cells %d dropped %v", stats.Cells, stats.Dropped)
	}
	_ = p.Flush(context.Background())
	if len(sink.histograms) != 5 || p.Snapshot().Cells != 0 {
		t.Errorf("flushed %d cells, %d left", len(sink.histograms), p.Snapshot().Cells)
	}
}

func TestEventsOfOlderSDKsFeedTheSameCells(t *testing.T) {
	p, sink := newTestPipeline(Config{})
	projectId := uuid.New()
	now := time.Now()
	resource := map[string]string{"service.name": "shop", "service.version": "4.1.0", "browser.name": "Firefox"}
	for i := 0; i < 10; i++ {
		if !p.ObserveEvent(projectId, "browser.web_vital.lcp", 2000, now, resource, map[string]string{"rating": "good", "url.path": "/orders/77"}, Client{IP: "10.0.0.1"}, now) {
			t.Fatal("event refused")
		}
	}
	if p.ObserveEvent(projectId, "browser.page_view", 1, now, resource, nil, Client{}, now) {
		t.Error("a counter was accepted as an event observation")
	}
	if _, mutated := resource["server_name"]; mutated {
		t.Error("the caller's resource attributes were changed")
	}
	_ = p.Flush(context.Background())
	if len(sink.histograms) != 1 || sink.histograms[0].Hist.Count != 10 || sink.histograms[0].Dims["url.path"] != "/orders/:id" {
		t.Fatalf("cells %+v", sink.histograms)
	}
}

func TestExemplarsAreCappedPerCell(t *testing.T) {
	p, sink := newTestPipeline(Config{})
	projectId := uuid.New()
	now := time.Now()
	for i := 0; i < 10; i++ {
		req := sdkPayload(t, browserResource, dataPoint{metric: "browser.web_vital.lcp", attrs: map[string]string{"rating": "poor"}, values: []float64{9000}, session: uuid.NewString()})
		p.IngestOTLP(projectId, req, Client{}, now)
	}
	p.IngestOTLP(projectId, sdkPayload(t, browserResource, dataPoint{metric: "browser.web_vital.lcp", attrs: map[string]string{"rating": "good"}, values: []float64{900}, session: "not-a-uuid"}), Client{}, now)
	_ = p.Flush(context.Background())
	if len(sink.exemplars) != exemplarsPerHour {
		t.Fatalf("%d exemplars, want %d", len(sink.exemplars), exemplarsPerHour)
	}
}

func TestCumulativeBackendSeriesAreDifferenced(t *testing.T) {
	p, sink := newTestPipeline(Config{})
	projectId := uuid.New()
	now := time.Now()
	tags := map[string]string{"http.route": "/api/users"}
	cumulative := shared.NewHistogram()
	for step, values := range [][]float64{{10, 20}, {30}, {40, 50}} {
		for _, v := range values {
			cumulative.Observe(v, 1)
		}
		snapshot := shared.NewHistogram()
		snapshot.Merge(cumulative)
		p.AddSeriesHistogram(projectId, "http.server.request.duration", now.Add(time.Duration(step)*time.Second), tags, snapshot, true, now)
	}
	_ = p.Flush(context.Background())
	// The first sample is the baseline.
	if h := sink.histogram("http.server.request.duration"); h.Count != 3 || h.Sum != 120 {
		t.Errorf("count %v sum %v, want 3 and 120", h.Count, h.Sum)
	}
}

func TestConcurrentIngestLosesNothing(t *testing.T) {
	p, sink := newTestPipeline(Config{FlushInterval: time.Millisecond, IPObservationsPerMinute: 1 << 30})
	ctx, cancel := context.WithCancel(context.Background())
	p.Start(ctx)
	projects := []uuid.UUID{uuid.New(), uuid.New(), uuid.New()}
	now := time.Now()
	const workers, perWorker = 16, 200
	var wg sync.WaitGroup
	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func(w int) {
			defer wg.Done()
			for i := 0; i < perWorker; i++ {
				req := sdkPayload(t, browserResource,
					dataPoint{metric: "browser.web_vital.lcp", attrs: map[string]string{"url.path": fmt.Sprintf("/p%d", i%7)}, values: []float64{float64(500 + i)}},
					dataPoint{metric: "browser.page_view", count: 1})
				p.IngestOTLP(projects[(w+i)%len(projects)], req, Client{}, now)
			}
		}(w)
	}
	wg.Wait()
	cancel()
	_ = p.Flush(context.Background())
	sink.mu.Lock()
	defer sink.mu.Unlock()
	if h := sink.histogram("browser.web_vital.lcp"); h.Count != workers*perWorker {
		t.Errorf("count %v, want %d", h.Count, workers*perWorker)
	}
	if got := sink.sum("browser.page_view"); got != workers*perWorker {
		t.Errorf("page views %v, want %d", got, workers*perWorker)
	}
}

func TestFailedFlushIsCountedAndDropped(t *testing.T) {
	p, sink := newTestPipeline(Config{})
	sink.fail = fmt.Errorf("storage down")
	p.IngestOTLP(uuid.New(), sdkPayload(t, browserResource, dataPoint{metric: "browser.web_vital.lcp", values: []float64{1}}), Client{}, time.Now())
	if err := p.Flush(context.Background()); err == nil {
		t.Fatal("no error")
	}
	stats := p.Snapshot()
	if stats.Dropped[ReasonFlushFailed] != 1 || stats.Cells != 0 {
		t.Errorf("%+v", stats)
	}
}

// One request of a realistic session payload: eight histograms and four counters.
func BenchmarkIngestOTLP(b *testing.B) {
	p, _ := newTestPipeline(Config{IPObservationsPerMinute: 1 << 30})
	t := &testing.T{}
	paths := []string{"/", "/orders/:id", "/cart", "/checkout", "/products/:id"}
	requests := make([]*colmetricspb.ExportMetricsServiceRequest, 64)
	for i := range requests {
		path := map[string]string{"url.path": paths[i%len(paths)], "network.type": "4g"}
		vital := map[string]string{"url.path": paths[i%len(paths)], "rating": "good", "navigation_type": "navigate"}
		requests[i] = sdkPayload(t, browserResource,
			dataPoint{metric: "browser.web_vital.lcp", attrs: vital, values: []float64{1800}},
			dataPoint{metric: "browser.web_vital.inp", attrs: vital, values: []float64{120}},
			dataPoint{metric: "browser.web_vital.cls", attrs: vital, values: []float64{0.02}},
			dataPoint{metric: "browser.web_vital.fcp", attrs: vital, values: []float64{900}},
			dataPoint{metric: "browser.web_vital.ttfb", attrs: vital, values: []float64{200}},
			dataPoint{metric: "browser.long_task.duration", attrs: path, values: []float64{80, 120, 300}},
			dataPoint{metric: "browser.route_timing.duration", attrs: path, values: []float64{400, 700}},
			dataPoint{metric: "browser.page_load.duration", attrs: path, values: []float64{2100}},
			dataPoint{metric: "browser.page_view", attrs: path, count: 3},
			dataPoint{metric: "browser.session", attrs: path, count: 1},
			dataPoint{metric: "browser.js_error", attrs: path, count: 1},
			dataPoint{metric: "browser.request.failed.total", attrs: map[string]string{"host": "api.shop.test"}, count: 12},
		)
	}
	projects := make([]uuid.UUID, 100)
	for i := range projects {
		projects[i] = uuid.New()
	}
	now := time.Now()
	b.ReportAllocs()
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			p.IngestOTLP(projects[i%len(projects)], requests[i%len(requests)], Client{}, now)
			i++
		}
	})
	b.StopTimer()
	b.ReportMetric(float64(p.Snapshot().Cells), "cells")
}
