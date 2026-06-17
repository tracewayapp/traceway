package repositories

import (
	"context"
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

	stats, total, err := EndpointRepository.FindGroupedByEndpoint(ctx, projectId, now.Add(-time.Hour), now.Add(time.Hour), 1, 10, "count", "desc", "", "")
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

	stats, total, err := EndpointRepository.FindGroupedByEndpoint(ctx, projectId, now.Add(-time.Hour), now.Add(time.Hour), 1, 10, "count", "desc", "users", "")
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
