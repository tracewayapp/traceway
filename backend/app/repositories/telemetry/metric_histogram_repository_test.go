package telemetry

import (
	"context"
	"fmt"
	"math"
	"testing"
	"time"

	"github.com/google/uuid"
)

func histogramOf(values ...float64) *Histogram {
	h := &Histogram{Buckets: map[int32]float64{}}
	for _, v := range values {
		h.Observe(v, 1)
	}
	return h
}

func histogramCell(projectId uuid.UUID, name string, at time.Time, dims map[string]string, values ...float64) MetricHistogramCell {
	return MetricHistogramCell{ProjectId: projectId, Name: name, RecordedAt: at, Dims: dims, Hist: histogramOf(values...)}
}

func TestMetricHistogramRepository_MergesCellsAcrossFlushes(t *testing.T) {
	setupTestDB(t)
	ctx := context.Background()
	projectId := uuid.New()
	minute := time.Now().UTC().Add(-10 * time.Minute).Truncate(time.Minute)
	dims := map[string]string{"service.version": "4.2", "url.path": "/orders/:id", "rating": "good"}

	// Two flushes, and two instances, write the same cell as separate rows.
	for _, values := range [][]float64{{100, 200}, {300}, {400, 500}} {
		if err := MetricHistogramRepository.InsertCells(ctx, []MetricHistogramCell{histogramCell(projectId, "browser.web_vital.lcp", minute, dims, values...)}); err != nil {
			t.Fatal(err)
		}
	}
	if err := MetricHistogramRepository.InsertCells(ctx, []MetricHistogramCell{histogramCell(uuid.New(), "browser.web_vital.lcp", minute, dims, 9000)}); err != nil {
		t.Fatal(err)
	}

	series, truncated, err := MetricHistogramRepository.Query(ctx, projectId, "browser.web_vital.lcp", minute.Add(-time.Hour), minute.Add(time.Hour), 60, nil, nil, 200)
	if err != nil {
		t.Fatal(err)
	}
	if truncated || len(series["__all__"]) != 1 {
		t.Fatalf("series %+v truncated %v", series, truncated)
	}
	h := series["__all__"][0].Hist
	if h.Count != 5 || h.Sum != 1500 || h.Min != 100 || h.Max != 500 {
		t.Fatalf("merged histogram %+v", h)
	}
	if p50 := h.Quantile(0.5); math.Abs(p50-300)/300 > 0.1 {
		t.Errorf("p50 = %v, want about 300", p50)
	}
}

func TestMetricHistogramRepository_GroupsFiltersAndBuckets(t *testing.T) {
	setupTestDB(t)
	ctx := context.Background()
	projectId := uuid.New()
	base := time.Now().UTC().Add(-2 * time.Hour).Truncate(time.Hour)
	cells := []MetricHistogramCell{
		histogramCell(projectId, "browser.route_timing.duration", base, map[string]string{"service.version": "4.1", "url.path": "/a", "outcome": "settled"}, 100, 110),
		histogramCell(projectId, "browser.route_timing.duration", base.Add(2*time.Minute), map[string]string{"service.version": "4.1", "url.path": "/b", "outcome": "settled"}, 120),
		histogramCell(projectId, "browser.route_timing.duration", base.Add(11*time.Minute), map[string]string{"service.version": "4.2", "url.path": "/a", "outcome": "timeout"}, 30000),
		histogramCell(projectId, "browser.route_timing.duration", base.Add(12*time.Minute), map[string]string{"url.path": "/a", "outcome": "settled"}, 90),
	}
	if err := MetricHistogramRepository.InsertCells(ctx, cells); err != nil {
		t.Fatal(err)
	}
	from, to := base.Add(-time.Minute), base.Add(time.Hour)

	byRelease, _, err := MetricHistogramRepository.Query(ctx, projectId, "browser.route_timing.duration", from, to, 10, nil, []string{"service.version"}, 200)
	if err != nil {
		t.Fatal(err)
	}
	if len(byRelease) != 3 || byRelease["4.1"][0].Hist.Count != 3 || byRelease["4.2"][0].Hist.Count != 1 || byRelease["(empty)"][0].Hist.Count != 1 {
		t.Fatalf("by release: %+v", byRelease)
	}
	if !byRelease["4.1"][0].Timestamp.Equal(base) || !byRelease["4.2"][0].Timestamp.Equal(base.Add(10*time.Minute)) {
		t.Errorf("buckets not floored to the interval: %v %v", byRelease["4.1"][0].Timestamp, byRelease["4.2"][0].Timestamp)
	}

	// A column dimension and a tag dimension filtered together.
	filtered, _, err := MetricHistogramRepository.Query(ctx, projectId, "browser.route_timing.duration", from, to, 60, map[string]string{"url.path": "/a", "outcome": "settled"}, nil, 200)
	if err != nil {
		t.Fatal(err)
	}
	if got := filtered["__all__"][0].Hist.Count; got != 3 {
		t.Errorf("filtered count = %v, want 3", got)
	}

	capped, truncated, err := MetricHistogramRepository.Query(ctx, projectId, "browser.route_timing.duration", from, to, 60, nil, []string{"service.version", "outcome"}, 2)
	if err != nil {
		t.Fatal(err)
	}
	if len(capped) != 2 || !truncated {
		t.Errorf("group cap: %d groups, truncated %v", len(capped), truncated)
	}
	// The cap keeps the groups with the most observations, whatever their keys sort like.
	busiest := 0.0
	for _, points := range capped {
		for _, point := range points {
			busiest = max(busiest, point.Hist.Count)
		}
	}
	if busiest != 3 {
		t.Errorf("the busiest group (3 observations) was cut; kept %+v", capped)
	}
}

func TestMetricHistogramRepository_Discover(t *testing.T) {
	setupTestDB(t)
	ctx := context.Background()
	projectId := uuid.New()
	at := time.Now().UTC().Add(-30 * time.Minute).Truncate(time.Minute)
	if err := MetricHistogramRepository.InsertCells(ctx, []MetricHistogramCell{
		histogramCell(projectId, "app.start.duration", at, map[string]string{"service.version": "1.0", "start_type": "cold"}, 800),
		histogramCell(projectId, "app.start.duration", at, map[string]string{"service.version": "1.1", "start_type": "warm"}, 200),
		histogramCell(projectId, "app.screen.ttid", at, map[string]string{"url.path": "Home"}, 120),
	}); err != nil {
		t.Fatal(err)
	}
	from, to := at.Add(-2*time.Hour), at.Add(time.Hour)

	metrics, err := MetricHistogramRepository.Discover(ctx, projectId, from, to)
	if err != nil {
		t.Fatal(err)
	}
	if len(metrics) != 2 || metrics[0].Name != "app.screen.ttid" || metrics[1].Name != "app.start.duration" {
		t.Fatalf("metrics %+v", metrics)
	}
	if got := metrics[1].TagKeys; len(got) != 2 || got[0] != "service.version" || got[1] != "start_type" || metrics[1].MetricType != "histogram" {
		t.Errorf("dimension keys %+v", metrics[1])
	}

	for key, want := range map[string][]string{"service.version": {"1.0", "1.1"}, "start_type": {"cold", "warm"}} {
		values, err := MetricHistogramRepository.DiscoverTagValues(ctx, projectId, "app.start.duration", key, from, to)
		if err != nil {
			t.Fatal(err)
		}
		if len(values) != 2 || values[0] != want[0] || values[1] != want[1] {
			t.Errorf("%s values = %v, want %v", key, values, want)
		}
	}
}

func TestMetricHistogramRepository_Exemplars(t *testing.T) {
	setupTestDB(t)
	ctx := context.Background()
	projectId := uuid.New()
	at := time.Now().UTC().Add(-5 * time.Minute).Truncate(time.Second)
	exemplar := func(value float64, session, path string) MetricExemplar {
		return MetricExemplar{ProjectId: projectId, Name: "browser.web_vital.lcp", RecordedAt: at, Dims: map[string]string{"url.path": path}, Value: value, SessionId: session, Target: "img." + session}
	}
	if err := MetricHistogramRepository.InsertExemplars(ctx, []MetricExemplar{exemplar(4100, "s1", "/a"), exemplar(9000, "s2", "/a"), exemplar(5000, "s3", "/b")}); err != nil {
		t.Fatal(err)
	}
	found, err := MetricHistogramRepository.FindExemplars(ctx, projectId, "browser.web_vital.lcp", at.Add(-time.Hour), at.Add(time.Hour), map[string]string{"url.path": "/a"}, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(found) != 2 || found[0].SessionId != "s2" || found[1].SessionId != "s1" || found[0].Dims["url.path"] != "/a" || found[0].Target != "img.s2" {
		t.Fatalf("exemplars %+v", found)
	}
}

func TestMetricHistogramRepository_HotSeries(t *testing.T) {
	setupTestDB(t)
	ctx := context.Background()
	busy, quiet := uuid.New(), uuid.New()
	hour := time.Now().UTC().Truncate(time.Hour)
	var cells []MetricHistogramCell
	for i := 0; i < 6; i++ {
		dims := map[string]string{"service.version": "1.0.0", "url.path": fmt.Sprintf("/page-%d", i), "rating": "good"}
		// The same series in two minutes is one series.
		cells = append(cells, histogramCell(busy, "browser.web_vital.lcp", hour, dims, 100), histogramCell(busy, "browser.web_vital.lcp", hour.Add(time.Minute), dims, 200))
	}
	cells = append(cells, histogramCell(busy, "browser.web_vital.inp", hour, map[string]string{"url.path": "/a"}, 50))
	cells = append(cells, histogramCell(quiet, "browser.web_vital.lcp", hour, map[string]string{"url.path": "/a"}, 50))
	if err := MetricHistogramRepository.InsertCells(ctx, cells); err != nil {
		t.Fatal(err)
	}
	sets, err := MetricHistogramRepository.HotSeries(ctx, hour.Truncate(24*time.Hour), 5, 100)
	if err != nil {
		t.Fatal(err)
	}
	if len(sets) != 1 || sets[0].ProjectId != busy || sets[0].Name != "browser.web_vital.lcp" || len(sets[0].Dims) != 6 {
		t.Fatalf("hot series %+v", sets)
	}
	paths := map[string]bool{}
	for _, dims := range sets[0].Dims {
		paths[dims["url.path"]] = true
		if dims["service.version"] != "1.0.0" || dims["rating"] != "good" || len(dims) != 3 {
			t.Errorf("dims %v", dims)
		}
	}
	if len(paths) != 6 {
		t.Errorf("paths %v", paths)
	}
	capped, err := MetricHistogramRepository.HotSeries(ctx, hour.Truncate(24*time.Hour), 5, 4)
	if err != nil || len(capped) != 1 || len(capped[0].Dims) != 4 {
		t.Fatalf("capped %+v %v", capped, err)
	}
}

func TestMetricHistogramRepository_CompactKeepsTheNumbers(t *testing.T) {
	setupTestDB(t)
	ctx := context.Background()
	projectId := uuid.New()
	hour := time.Now().UTC().Add(-12 * time.Hour).Truncate(time.Hour)
	dims := map[string]string{"service.version": "4.2"}
	var cells []MetricHistogramCell
	for minute := 0; minute < 60; minute++ {
		cells = append(cells, histogramCell(projectId, "browser.web_vital.inp", hour.Add(time.Duration(minute)*time.Minute), dims, float64(100+minute)))
	}
	recent := time.Now().UTC().Add(-time.Minute).Truncate(time.Minute)
	cells = append(cells, histogramCell(projectId, "browser.web_vital.inp", recent, dims, 999))
	if err := MetricHistogramRepository.InsertCells(ctx, cells); err != nil {
		t.Fatal(err)
	}
	read := func() *Histogram {
		series, _, err := MetricHistogramRepository.Query(ctx, projectId, "browser.web_vital.inp", hour.Add(-time.Hour), time.Now().UTC().Add(time.Hour), 60*24*30, nil, nil, 200)
		if err != nil {
			t.Fatal(err)
		}
		merged := &Histogram{Buckets: map[int32]float64{}}
		for _, point := range series["__all__"] {
			merged.Merge(point.Hist)
		}
		return merged
	}
	before := read()

	replaced, err := MetricHistogramRepository.Compact(ctx, time.Now().UTC().Add(-6*time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	after := read()
	if after.Count != before.Count || after.Sum != before.Sum || after.Quantile(0.75) != before.Quantile(0.75) {
		t.Fatalf("compaction changed the data: before %+v after %+v", before, after)
	}
	if telemetryBackendCompacts && replaced != 60 {
		t.Errorf("replaced %d minute rows, want 60", replaced)
	}
	again, err := MetricHistogramRepository.Compact(ctx, time.Now().UTC().Add(-6*time.Hour))
	if err != nil || again != 0 {
		t.Errorf("a second pass replaced %d rows (err %v), want 0", again, err)
	}
}

func TestMetricHistogramRepositoryPreservesEffectiveCount(t *testing.T) {
	setupTestDB(t)
	ctx := context.Background()
	project := uuid.New()
	at := time.Now().UTC().Add(-time.Hour).Truncate(time.Minute)
	h := histogramOf()
	h.Observe(5000, 1000)
	if err := MetricHistogramRepository.InsertCells(ctx, []MetricHistogramCell{{ProjectId: project, Name: "browser.web_vital.lcp", RecordedAt: at, Dims: map[string]string{}, Hist: h}}); err != nil {
		t.Fatal(err)
	}
	for _, window := range []time.Duration{time.Minute, 4 * 24 * time.Hour, 91 * 24 * time.Hour} {
		series, _, err := MetricHistogramRepository.Query(ctx, project, "browser.web_vital.lcp", at.Add(-window), at.Add(time.Minute), 24*60, nil, nil, 1)
		if err != nil {
			t.Fatal(err)
		}
		got := series["__all__"][0].Hist
		if got.Count != 1000 || got.EffectiveCount() != 1 {
			t.Fatalf("stored weighted sample: %+v", got)
		}
	}
}

// One busy hour used to stop compaction for good: it took the oldest hour across all
// projects and gave up at 20,000 rows.
func TestMetricHistogramRepository_CompactsABusyHourOfSeveralProjects(t *testing.T) {
	if !telemetryBackendCompacts {
		t.Skip("this backend rolls up by itself")
	}
	setupTestDB(t)
	ctx := context.Background()
	hour := time.Now().UTC().Add(-12 * time.Hour).Truncate(time.Hour)
	projects := []uuid.UUID{uuid.New(), uuid.New()}
	total := 0
	for _, projectId := range projects {
		var cells []MetricHistogramCell
		for series := 0; series < 180; series++ {
			dims := map[string]string{"url.path": fmt.Sprintf("/p/%d", series)}
			for minute := 0; minute < 60; minute++ {
				cells = append(cells, histogramCell(projectId, "browser.web_vital.inp", hour.Add(time.Duration(minute)*time.Minute), dims, float64(100+minute)))
			}
		}
		if err := MetricHistogramRepository.InsertCells(ctx, cells); err != nil {
			t.Fatal(err)
		}
		total += len(cells)
	}
	if total <= 20000 {
		t.Fatalf("the test needs more than 20000 rows, has %d", total)
	}
	replaced, err := MetricHistogramRepository.Compact(ctx, time.Now().UTC().Add(-6*time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if replaced != total {
		t.Errorf("replaced %d minute rows, want %d", replaced, total)
	}
	for _, projectId := range projects {
		series, _, err := MetricHistogramRepository.Query(ctx, projectId, "browser.web_vital.inp", hour, hour.Add(time.Hour), 60, nil, []string{"url.path"}, 200)
		if err != nil {
			t.Fatal(err)
		}
		if len(series) != 180 {
			t.Fatalf("%d series after compaction, want 180", len(series))
		}
		point := series["/p/7"][0]
		if point.Hist.Count != 60 || point.GrainSeconds != 3600 {
			t.Errorf("series /p/7: count %v grain %d, want 60 observations in one hour row", point.Hist.Count, point.GrainSeconds)
		}
	}
}
