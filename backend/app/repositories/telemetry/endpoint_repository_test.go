package telemetry

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/tracewayapp/traceway/backend/app/models"
)

func TestEndpointRepository_InsertAndCount(t *testing.T) {
	setupTestDB(t)
	ctx := context.Background()
	projectId := uuid.New()
	now := truncateMs(time.Now().UTC())

	endpoints := []models.Endpoint{
		makeEndpoint(projectId, "GET /api/users", 100*time.Millisecond, 200, now),
		makeEndpoint(projectId, "POST /api/users", 200*time.Millisecond, 201, now.Add(time.Minute)),
		makeEndpoint(projectId, "GET /api/users", 150*time.Millisecond, 200, now.Add(2*time.Minute)),
	}

	err := EndpointRepository.InsertAsync(ctx, endpoints)
	if err != nil {
		t.Fatalf("InsertAsync failed: %v", err)
	}

	count, err := EndpointRepository.CountBetween(ctx, projectId, now.Add(-time.Hour), now.Add(time.Hour))
	if err != nil {
		t.Fatalf("CountBetween failed: %v", err)
	}

	if count != 3 {
		t.Errorf("expected count 3, got %d", count)
	}
}

func TestEndpointRepository_CountBetween_TimeFilter(t *testing.T) {
	setupTestDB(t)
	ctx := context.Background()
	projectId := uuid.New()
	now := truncateMs(time.Now().UTC())

	endpoints := []models.Endpoint{
		makeEndpoint(projectId, "GET /old", 100*time.Millisecond, 200, now.Add(-2*time.Hour)),
		makeEndpoint(projectId, "GET /new", 100*time.Millisecond, 200, now),
	}

	if err := EndpointRepository.InsertAsync(ctx, endpoints); err != nil {
		t.Fatalf("InsertAsync failed: %v", err)
	}

	count, err := EndpointRepository.CountBetween(ctx, projectId, now.Add(-time.Hour), now.Add(time.Hour))
	if err != nil {
		t.Fatalf("CountBetween failed: %v", err)
	}

	if count != 1 {
		t.Errorf("expected count 1 (only recent endpoint), got %d", count)
	}
}

func TestEndpointRepository_FindAll(t *testing.T) {
	setupTestDB(t)
	ctx := context.Background()
	projectId := uuid.New()
	now := truncateMs(time.Now().UTC())

	endpoints := []models.Endpoint{
		makeEndpoint(projectId, "GET /api/a", 100*time.Millisecond, 200, now),
		makeEndpoint(projectId, "GET /api/b", 200*time.Millisecond, 200, now.Add(time.Minute)),
		makeEndpoint(projectId, "GET /api/c", 300*time.Millisecond, 200, now.Add(2*time.Minute)),
	}

	if err := EndpointRepository.InsertAsync(ctx, endpoints); err != nil {
		t.Fatalf("InsertAsync failed: %v", err)
	}

	found, total, err := EndpointRepository.FindAll(ctx, projectId, now.Add(-time.Hour), now.Add(time.Hour), 1, 10, "recorded_at")
	if err != nil {
		t.Fatalf("FindAll failed: %v", err)
	}

	if total != 3 {
		t.Errorf("expected total 3, got %d", total)
	}
	if len(found) != 3 {
		t.Errorf("expected 3 results, got %d", len(found))
	}
}

func TestEndpointRepository_FindAll_Pagination(t *testing.T) {
	setupTestDB(t)
	ctx := context.Background()
	projectId := uuid.New()
	now := truncateMs(time.Now().UTC())

	endpoints := make([]models.Endpoint, 5)
	for i := range endpoints {
		endpoints[i] = makeEndpoint(projectId, "GET /api/test", time.Duration(i+1)*100*time.Millisecond, 200, now.Add(time.Duration(i)*time.Minute))
	}

	if err := EndpointRepository.InsertAsync(ctx, endpoints); err != nil {
		t.Fatalf("InsertAsync failed: %v", err)
	}

	page1, total, err := EndpointRepository.FindAll(ctx, projectId, now.Add(-time.Hour), now.Add(time.Hour), 1, 2, "recorded_at")
	if err != nil {
		t.Fatalf("FindAll page 1 failed: %v", err)
	}
	if total != 5 {
		t.Errorf("expected total 5, got %d", total)
	}
	if len(page1) != 2 {
		t.Errorf("expected 2 results on page 1, got %d", len(page1))
	}
}

func TestEndpointRepository_FindGroupedByEndpoint(t *testing.T) {
	setupTestDB(t)
	ctx := context.Background()
	projectId := uuid.New()
	now := truncateMs(time.Now().UTC())

	endpoints := []models.Endpoint{
		makeEndpoint(projectId, "GET /api/users", 100*time.Millisecond, 200, now),
		makeEndpoint(projectId, "GET /api/users", 200*time.Millisecond, 200, now.Add(time.Minute)),
		makeEndpoint(projectId, "GET /api/users", 300*time.Millisecond, 200, now.Add(2*time.Minute)),
		makeEndpoint(projectId, "POST /api/users", 150*time.Millisecond, 201, now.Add(3*time.Minute)),
	}

	if err := EndpointRepository.InsertAsync(ctx, endpoints); err != nil {
		t.Fatalf("InsertAsync failed: %v", err)
	}

	stats, total, err := EndpointRepository.FindGroupedByEndpoint(ctx, projectId, now.Add(-time.Hour), now.Add(time.Hour), 1, 10, "count", "desc", "", "", "")
	if err != nil {
		t.Fatalf("FindGroupedByEndpoint failed: %v", err)
	}

	if total != 2 {
		t.Errorf("expected 2 distinct endpoints, got %d", total)
	}
	if len(stats) != 2 {
		t.Fatalf("expected 2 grouped stats, got %d", len(stats))
	}

	// Ordered by count DESC: GET /api/users (3) first
	if stats[0].Endpoint != "GET /api/users" {
		t.Errorf("expected first group 'GET /api/users', got %q", stats[0].Endpoint)
	}
	if stats[0].Count != 3 {
		t.Errorf("expected count 3, got %d", stats[0].Count)
	}
}

func TestEndpointRepository_FindGroupedByEndpoint_Search(t *testing.T) {
	setupTestDB(t)
	ctx := context.Background()
	projectId := uuid.New()
	now := truncateMs(time.Now().UTC())

	endpoints := []models.Endpoint{
		makeEndpoint(projectId, "GET /api/users", 100*time.Millisecond, 200, now),
		makeEndpoint(projectId, "GET /api/products", 200*time.Millisecond, 200, now.Add(time.Minute)),
	}

	if err := EndpointRepository.InsertAsync(ctx, endpoints); err != nil {
		t.Fatalf("InsertAsync failed: %v", err)
	}

	stats, total, err := EndpointRepository.FindGroupedByEndpoint(ctx, projectId, now.Add(-time.Hour), now.Add(time.Hour), 1, 10, "count", "desc", "users", "", "")
	if err != nil {
		t.Fatalf("FindGroupedByEndpoint with search failed: %v", err)
	}

	if total != 1 {
		t.Errorf("expected 1 matching endpoint, got %d", total)
	}
	if len(stats) != 1 {
		t.Fatalf("expected 1 grouped stat, got %d", len(stats))
	}
	if stats[0].Endpoint != "GET /api/users" {
		t.Errorf("expected 'GET /api/users', got %q", stats[0].Endpoint)
	}
}

func TestEndpointRepository_FindGroupedByEndpoint_MethodFilter(t *testing.T) {
	setupTestDB(t)
	ctx := context.Background()
	projectId := uuid.New()
	now := truncateMs(time.Now().UTC())

	endpoints := []models.Endpoint{
		makeEndpoint(projectId, "GET /api/users", 100*time.Millisecond, 200, now),
		makeEndpoint(projectId, "POST /api/users", 200*time.Millisecond, 201, now.Add(time.Minute)),
		makeEndpoint(projectId, "GETAWAY /api/cars", 100*time.Millisecond, 200, now.Add(2*time.Minute)),
		makeEndpoint(projectId, "get /api/lowercase", 100*time.Millisecond, 200, now.Add(3*time.Minute)),
	}

	if err := EndpointRepository.InsertAsync(ctx, endpoints); err != nil {
		t.Fatalf("InsertAsync failed: %v", err)
	}

	// "GETAWAY /api/cars" must not match: the filter compares a whole
	// space-terminated token, not a prefix of the endpoint string.
	// "get /api/lowercase" must match. getHTTPEndpoint concatenates
	// http.request.method verbatim, so lowercase rows genuinely exist, and the
	// dropdown only ever offers the 7 canonical uppercase methods -- a
	// case-sensitive comparison made those rows unreachable from the UI (#321).
	stats, total, err := EndpointRepository.FindGroupedByEndpoint(ctx, projectId, now.Add(-time.Hour), now.Add(time.Hour), 1, 10, "count", "desc", "", "", "get")
	if err != nil {
		t.Fatalf("FindGroupedByEndpoint with method filter failed: %v", err)
	}

	if total != 2 {
		t.Errorf("expected 2 matching endpoints, got %d", total)
	}
	matched := make(map[string]bool, len(stats))
	for _, s := range stats {
		matched[s.Endpoint] = true
	}
	if len(stats) != 2 || !matched["GET /api/users"] || !matched["get /api/lowercase"] {
		t.Fatalf("expected GET /api/users and get /api/lowercase, got %v", matched)
	}
}

// The filter is applied by upper-casing the column, not the caller's argument,
// so it holds however the request cased its method.
func TestEndpointRepository_FindGroupedByEndpoint_MethodFilterCaseInsensitive(t *testing.T) {
	setupTestDB(t)
	ctx := context.Background()
	projectId := uuid.New()
	now := truncateMs(time.Now().UTC())

	endpoints := []models.Endpoint{
		makeEndpoint(projectId, "GET /api/users", 100*time.Millisecond, 200, now),
		makeEndpoint(projectId, "get /api/lowercase", 100*time.Millisecond, 200, now.Add(time.Minute)),
		makeEndpoint(projectId, "Get /api/mixed", 100*time.Millisecond, 200, now.Add(2*time.Minute)),
		makeEndpoint(projectId, "POST /api/users", 200*time.Millisecond, 201, now.Add(3*time.Minute)),
	}

	if err := EndpointRepository.InsertAsync(ctx, endpoints); err != nil {
		t.Fatalf("InsertAsync failed: %v", err)
	}

	for _, method := range []string{"GET", "get", "GeT"} {
		_, total, err := EndpointRepository.FindGroupedByEndpoint(ctx, projectId, now.Add(-time.Hour), now.Add(time.Hour), 1, 10, "count", "desc", "", "", method)
		if err != nil {
			t.Fatalf("methodFilter %q failed: %v", method, err)
		}
		if total != 3 {
			t.Errorf("methodFilter %q: expected 3 matching endpoints, got %d", method, total)
		}
	}
}

func TestEndpointRepository_GetEndpointStackedChart_Filters(t *testing.T) {
	setupTestDB(t)
	ctx := context.Background()
	projectId := uuid.New()
	now := truncateMs(time.Now().UTC())

	var endpoints []models.Endpoint
	for i := range 6 {
		name := fmt.Sprintf("GET /api/g%d", i)
		endpoints = append(endpoints, makeEndpoint(projectId, name, time.Duration(i+1)*100*time.Millisecond, 200, now))
	}
	endpoints = append(endpoints, makeEndpoint(projectId, "POST /api/p", time.Second, 201, now))

	if err := EndpointRepository.InsertAsync(ctx, endpoints); err != nil {
		t.Fatalf("InsertAsync failed: %v", err)
	}

	for _, metricType := range []string{"total_time", "p95"} {
		chart, err := EndpointRepository.GetEndpointStackedChart(ctx, projectId, now.Add(-time.Hour), now.Add(time.Hour), 5, metricType, "", "", "get")
		if err != nil {
			t.Fatalf("%s: GetEndpointStackedChart failed: %v", metricType, err)
		}
		if len(chart.Endpoints) != 6 || chart.Endpoints[5] != "Other" {
			t.Errorf("%s: expected five GET endpoints plus Other, got %v", metricType, chart.Endpoints)
		}
		for _, name := range chart.Endpoints {
			if name == "POST /api/p" {
				t.Errorf("%s: POST endpoint leaked into the filtered chart", metricType)
			}
		}
		if len(chart.Series) == 0 {
			t.Errorf("%s: expected series points", metricType)
		}
		for _, p := range chart.Series {
			if p.Endpoint == "POST /api/p" {
				t.Errorf("%s: POST endpoint leaked into the series", metricType)
			}
		}
	}

	chart, err := EndpointRepository.GetEndpointStackedChart(ctx, projectId, now.Add(-time.Hour), now.Add(time.Hour), 5, "total_time", "api/p", "", "")
	if err != nil {
		t.Fatalf("GetEndpointStackedChart with search failed: %v", err)
	}
	if len(chart.Endpoints) != 1 || chart.Endpoints[0] != "POST /api/p" {
		t.Errorf("expected the search to leave only POST /api/p, got %v", chart.Endpoints)
	}
}

func TestEndpointRepository_FindByEndpoint(t *testing.T) {
	setupTestDB(t)
	ctx := context.Background()
	projectId := uuid.New()
	now := truncateMs(time.Now().UTC())

	endpoints := []models.Endpoint{
		makeEndpoint(projectId, "GET /api/users", 100*time.Millisecond, 200, now),
		makeEndpoint(projectId, "GET /api/users", 200*time.Millisecond, 200, now.Add(time.Minute)),
		makeEndpoint(projectId, "POST /api/users", 300*time.Millisecond, 201, now.Add(2*time.Minute)),
	}

	if err := EndpointRepository.InsertAsync(ctx, endpoints); err != nil {
		t.Fatalf("InsertAsync failed: %v", err)
	}

	found, total, err := EndpointRepository.FindByEndpoint(ctx, projectId, "GET /api/users", now.Add(-time.Hour), now.Add(time.Hour), 1, 10, "recorded_at", "desc")
	if err != nil {
		t.Fatalf("FindByEndpoint failed: %v", err)
	}

	if total != 2 {
		t.Errorf("expected total 2, got %d", total)
	}
	if len(found) != 2 {
		t.Errorf("expected 2 results, got %d", len(found))
	}
	for _, f := range found {
		if f.Endpoint != "GET /api/users" {
			t.Errorf("expected endpoint 'GET /api/users', got %q", f.Endpoint)
		}
	}
}

func TestEndpointRepository_FindById(t *testing.T) {
	setupTestDB(t)
	ctx := context.Background()
	projectId := uuid.New()
	now := truncateMs(time.Now().UTC())

	ep := makeEndpoint(projectId, "GET /api/specific", 250*time.Millisecond, 200, now)

	if err := EndpointRepository.InsertAsync(ctx, []models.Endpoint{ep}); err != nil {
		t.Fatalf("InsertAsync failed: %v", err)
	}

	found, err := EndpointRepository.FindById(ctx, projectId, ep.Id, nil)
	if err != nil {
		t.Fatalf("FindById failed: %v", err)
	}
	if found == nil {
		t.Fatal("expected to find endpoint, got nil")
	}
	if found.Endpoint != "GET /api/specific" {
		t.Errorf("expected endpoint 'GET /api/specific', got %q", found.Endpoint)
	}
	if found.Duration != 250*time.Millisecond {
		t.Errorf("expected duration 250ms, got %v", found.Duration)
	}
}

func TestEndpointRepository_FindById_NotFound(t *testing.T) {
	setupTestDB(t)
	ctx := context.Background()

	found, err := EndpointRepository.FindById(ctx, uuid.New(), uuid.New(), nil)
	if err != nil {
		t.Fatalf("FindById failed: %v", err)
	}
	if found != nil {
		t.Errorf("expected nil for unknown endpoint, got %+v", found)
	}
}

func TestEndpointRepository_CountByHour(t *testing.T) {
	setupTestDB(t)
	ctx := context.Background()
	projectId := uuid.New()
	now := truncateMs(time.Now().UTC()).Truncate(time.Hour)

	endpoints := []models.Endpoint{
		makeEndpoint(projectId, "GET /a", 100*time.Millisecond, 200, now),
		makeEndpoint(projectId, "GET /b", 100*time.Millisecond, 200, now.Add(30*time.Minute)),
		makeEndpoint(projectId, "GET /c", 100*time.Millisecond, 200, now.Add(time.Hour+10*time.Minute)),
	}

	if err := EndpointRepository.InsertAsync(ctx, endpoints); err != nil {
		t.Fatalf("InsertAsync failed: %v", err)
	}

	points, err := EndpointRepository.CountByHour(ctx, projectId, now.Add(-time.Minute), now.Add(2*time.Hour))
	if err != nil {
		t.Fatalf("CountByHour failed: %v", err)
	}

	if len(points) != 2 {
		t.Fatalf("expected 2 hourly buckets, got %d", len(points))
	}

	if points[0].Value != 2 {
		t.Errorf("expected 2 endpoints in first hour, got %v", points[0].Value)
	}
	if points[1].Value != 1 {
		t.Errorf("expected 1 endpoint in second hour, got %v", points[1].Value)
	}
}

func TestEndpointRepository_ErrorRateByHour(t *testing.T) {
	setupTestDB(t)
	ctx := context.Background()
	projectId := uuid.New()
	now := truncateMs(time.Now().UTC()).Truncate(time.Hour)

	endpoints := []models.Endpoint{
		makeEndpoint(projectId, "GET /api/ok", 100*time.Millisecond, 200, now),
		makeEndpoint(projectId, "GET /api/ok", 100*time.Millisecond, 200, now.Add(time.Minute)),
		makeEndpoint(projectId, "GET /api/err", 100*time.Millisecond, 500, now.Add(2*time.Minute)),
		makeEndpoint(projectId, "GET /api/err", 100*time.Millisecond, 503, now.Add(3*time.Minute)),
	}

	if err := EndpointRepository.InsertAsync(ctx, endpoints); err != nil {
		t.Fatalf("InsertAsync failed: %v", err)
	}

	points, err := EndpointRepository.ErrorRateByHour(ctx, projectId, now.Add(-time.Minute), now.Add(time.Hour))
	if err != nil {
		t.Fatalf("ErrorRateByHour failed: %v", err)
	}

	if len(points) != 1 {
		t.Fatalf("expected 1 hourly bucket, got %d", len(points))
	}

	// 2 out of 4 are errors = 50%
	assertApproxEqual(t, "error rate", points[0].Value, 50.0, 0.1)
}

func TestEndpointRepository_UpsertAndGetSlowEndpoint(t *testing.T) {
	setupTestDB(t)
	ctx := context.Background()
	projectId := uuid.New()

	err := EndpointRepository.UpsertSlowEndpoint(ctx, projectId, "GET /api/slow", 500, "Known slow endpoint")
	if err != nil {
		t.Fatalf("UpsertSlowEndpoint failed: %v", err)
	}

	offsetMs, reason, err := EndpointRepository.GetSlowEndpoint(ctx, projectId, "GET /api/slow")
	if err != nil {
		t.Fatalf("GetSlowEndpoint failed: %v", err)
	}

	if offsetMs != 500 {
		t.Errorf("expected offset 500ms, got %d", offsetMs)
	}
	if reason != "Known slow endpoint" {
		t.Errorf("expected reason 'Known slow endpoint', got %q", reason)
	}

	// Upsert should update existing
	err = EndpointRepository.UpsertSlowEndpoint(ctx, projectId, "GET /api/slow", 1000, "Updated reason")
	if err != nil {
		t.Fatalf("UpsertSlowEndpoint update failed: %v", err)
	}

	offsetMs, reason, err = EndpointRepository.GetSlowEndpoint(ctx, projectId, "GET /api/slow")
	if err != nil {
		t.Fatalf("GetSlowEndpoint after update failed: %v", err)
	}

	if offsetMs != 1000 {
		t.Errorf("expected updated offset 1000ms, got %d", offsetMs)
	}
	if reason != "Updated reason" {
		t.Errorf("expected updated reason, got %q", reason)
	}
}

func TestEndpointRepository_FindWorstEndpoints(t *testing.T) {
	setupTestDB(t)
	ctx := context.Background()
	projectId := uuid.New()
	now := truncateMs(time.Now().UTC())

	endpoints := []models.Endpoint{
		// Healthy endpoint
		makeEndpoint(projectId, "GET /healthy", 100*time.Millisecond, 200, now),
		makeEndpoint(projectId, "GET /healthy", 150*time.Millisecond, 200, now.Add(time.Minute)),
		// Unhealthy endpoint (5xx errors)
		makeEndpoint(projectId, "GET /broken", 100*time.Millisecond, 500, now),
		makeEndpoint(projectId, "GET /broken", 200*time.Millisecond, 500, now.Add(time.Minute)),
	}

	if err := EndpointRepository.InsertAsync(ctx, endpoints); err != nil {
		t.Fatalf("InsertAsync failed: %v", err)
	}

	worst, err := EndpointRepository.FindWorstEndpoints(ctx, projectId, now.Add(-time.Hour), now.Add(time.Hour), 5)
	if err != nil {
		t.Fatalf("FindWorstEndpoints failed: %v", err)
	}

	if len(worst) != 2 {
		t.Fatalf("expected 2 endpoint groups, got %d", len(worst))
	}

	// /broken should have higher impact (100% error rate)
	if worst[0].Endpoint != "GET /broken" {
		t.Errorf("expected worst endpoint to be 'GET /broken', got %q", worst[0].Endpoint)
	}
}

func TestEndpointRepository_GetEndpointStats(t *testing.T) {
	setupTestDB(t)
	ctx := context.Background()
	projectId := uuid.New()
	now := truncateMs(time.Now().UTC())

	endpoints := []models.Endpoint{
		makeEndpoint(projectId, "GET /api/measured", 100*time.Millisecond, 200, now),
		makeEndpoint(projectId, "GET /api/measured", 200*time.Millisecond, 200, now.Add(time.Minute)),
		makeEndpoint(projectId, "GET /api/measured", 300*time.Millisecond, 200, now.Add(2*time.Minute)),
		makeEndpoint(projectId, "GET /api/measured", 400*time.Millisecond, 500, now.Add(3*time.Minute)),
	}

	if err := EndpointRepository.InsertAsync(ctx, endpoints); err != nil {
		t.Fatalf("InsertAsync failed: %v", err)
	}

	start := now.Add(-time.Hour)
	end := now.Add(time.Hour)
	stats, err := EndpointRepository.GetEndpointStats(ctx, projectId, "GET /api/measured", start, end)
	if err != nil {
		t.Fatalf("GetEndpointStats failed: %v", err)
	}

	if stats.Count != 4 {
		t.Errorf("expected count 4, got %d", stats.Count)
	}

	// avg duration = (100+200+300+400)/4 = 250ms
	assertApproxEqual(t, "AvgDuration", stats.AvgDuration, 250.0, 1.0)

	// 1 out of 4 is error = 25%
	assertApproxEqual(t, "ErrorRate", stats.ErrorRate, 25.0, 0.1)

	if stats.MedianDuration < 100 || stats.MedianDuration > 400 {
		t.Errorf("MedianDuration %v out of expected range", stats.MedianDuration)
	}

	if stats.Throughput <= 0 {
		t.Errorf("expected positive throughput, got %v", stats.Throughput)
	}
}

func TestEndpointRepository_InsertEmpty(t *testing.T) {
	setupTestDB(t)
	ctx := context.Background()

	err := EndpointRepository.InsertAsync(ctx, []models.Endpoint{})
	if err != nil {
		t.Fatalf("InsertAsync with empty slice should not error: %v", err)
	}
}

func TestEndpointRepository_GetEndpointStats_EmptyWindow(t *testing.T) {
	setupTestDB(t)
	ctx := context.Background()
	now := time.Now().UTC()

	stats, err := EndpointRepository.GetEndpointStats(ctx, uuid.New(), "GET /api/none", now.Add(-time.Hour), now)
	if err != nil {
		t.Fatalf("GetEndpointStats failed: %v", err)
	}
	if stats.Count != 0 {
		t.Errorf("expected count 0, got %d", stats.Count)
	}
	if stats.IsStream {
		t.Errorf("expected IsStream false for empty window")
	}
}

func TestEndpointRepository_FindGroupedByEndpoint_RootFilterPercentiles(t *testing.T) {
	setupTestDB(t)
	ctx := context.Background()
	projectId := uuid.New()
	now := truncateMs(time.Now().UTC())

	var endpoints []models.Endpoint
	for i := 0; i < 4; i++ {
		at := now.Add(time.Duration(i) * time.Second)
		root := makeEndpoint(projectId, "GET /api/orders", 100*time.Millisecond, 200, at)
		root.IsRoot = true
		endpoints = append(endpoints, root, makeEndpoint(projectId, "GET /api/orders", 900*time.Millisecond, 200, at.Add(500*time.Millisecond)))
	}
	if err := EndpointRepository.InsertAsync(ctx, endpoints); err != nil {
		t.Fatalf("InsertAsync failed: %v", err)
	}

	stats, total, err := EndpointRepository.FindGroupedByEndpoint(ctx, projectId, now.Add(-time.Hour), now.Add(time.Hour), 1, 10, "count", "desc", "", "root", "")
	if err != nil {
		t.Fatalf("FindGroupedByEndpoint with root filter failed: %v", err)
	}
	if total != 1 || len(stats) != 1 {
		t.Fatalf("expected 1 grouped endpoint, got total=%d len=%d", total, len(stats))
	}
	if stats[0].Count != 4 {
		t.Errorf("count = %d, want the 4 root calls", stats[0].Count)
	}
	if stats[0].P95Duration > 100*time.Millisecond {
		t.Errorf("p95 = %v, want the root-only 100ms; non-root durations leaked into the percentile", stats[0].P95Duration)
	}
}

// The chart ranks the top 5 on percentiles and then plots only the filtered
// rows, so ranking on the unfiltered population puts endpoints on the chart that
// are not slow under the filter. These two rank one way on all rows and the
// other way on root rows alone.
func TestEndpointRepository_GetEndpointStackedChart_RanksOnFilteredRows(t *testing.T) {
	setupTestDB(t)
	ctx := context.Background()
	projectId := uuid.New()
	now := truncateMs(time.Now().UTC())

	var endpoints []models.Endpoint
	for range 3 {
		// Slow as a root request, which is what "root" asks to see.
		slowRoot := makeEndpoint(projectId, "GET /slow-root", 500*time.Millisecond, 200, now)
		slowRoot.IsRoot = true
		// Fast as a root request, slow only in the rows the filter drops.
		fastRoot := makeEndpoint(projectId, "GET /fast-root", 10*time.Millisecond, 200, now)
		fastRoot.IsRoot = true
		endpoints = append(endpoints, slowRoot, fastRoot,
			makeEndpoint(projectId, "GET /fast-root", 5*time.Second, 200, now))
	}

	if err := EndpointRepository.InsertAsync(ctx, endpoints); err != nil {
		t.Fatalf("InsertAsync failed: %v", err)
	}

	chart, err := EndpointRepository.GetEndpointStackedChart(ctx, projectId, now.Add(-time.Hour), now.Add(time.Hour), 5, "p95", "", "root", "")
	if err != nil {
		t.Fatalf("GetEndpointStackedChart failed: %v", err)
	}
	if len(chart.Endpoints) < 2 {
		t.Fatalf("expected both endpoints in the chart, got %v", chart.Endpoints)
	}
	if chart.Endpoints[0] != "GET /slow-root" {
		t.Errorf("top endpoint ranked on unfiltered rows: got %v, want 'GET /slow-root' first", chart.Endpoints)
	}
}
