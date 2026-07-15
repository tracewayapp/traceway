package telemetry

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/tracewayapp/traceway/backend/app/models"
)

func TestMetricPointRepository_InsertAndQueryTimeSeries(t *testing.T) {
	setupTestDB(t)
	ctx := context.Background()
	projectId := uuid.New()
	base := time.Now().UTC().Truncate(time.Hour)

	points := []models.MetricPoint{
		makeMetricPoint(projectId, "cpu.used_pcnt", 50.0, map[string]string{"server_name": "web-1"}, base),
		makeMetricPoint(projectId, "cpu.used_pcnt", 60.0, map[string]string{"server_name": "web-1"}, base.Add(5*time.Minute)),
		makeMetricPoint(projectId, "cpu.used_pcnt", 70.0, map[string]string{"server_name": "web-1"}, base.Add(35*time.Minute)),
	}

	err := MetricPointRepository.InsertAsync(ctx, points)
	if err != nil {
		t.Fatalf("InsertAsync failed: %v", err)
	}

	result, err := MetricPointRepository.QueryTimeSeries(ctx, projectId, "cpu.used_pcnt", base.Add(-time.Minute), base.Add(time.Hour), 30, "avg", nil, "")
	if err != nil {
		t.Fatalf("QueryTimeSeries failed: %v", err)
	}

	series, ok := result["__all__"]
	if !ok {
		t.Fatal("expected '__all__' series key")
	}

	if len(series) != 2 {
		t.Fatalf("expected 2 time series buckets, got %d", len(series))
	}

	// First bucket avg = (50+60)/2 = 55
	assertApproxEqual(t, "first bucket avg", series[0].Value, 55.0, 0.1)
	// Second bucket avg = 70
	assertApproxEqual(t, "second bucket avg", series[1].Value, 70.0, 0.1)
}

func TestMetricPointRepository_QueryTimeSeries_WithGroupBy(t *testing.T) {
	setupTestDB(t)
	ctx := context.Background()
	projectId := uuid.New()
	base := time.Now().UTC().Truncate(time.Hour)

	points := []models.MetricPoint{
		makeMetricPoint(projectId, "cpu.used_pcnt", 50.0, map[string]string{"server_name": "web-1"}, base),
		makeMetricPoint(projectId, "cpu.used_pcnt", 80.0, map[string]string{"server_name": "web-2"}, base),
		makeMetricPoint(projectId, "cpu.used_pcnt", 60.0, map[string]string{"server_name": "web-1"}, base.Add(5*time.Minute)),
	}

	if err := MetricPointRepository.InsertAsync(ctx, points); err != nil {
		t.Fatalf("InsertAsync failed: %v", err)
	}

	result, err := MetricPointRepository.QueryTimeSeries(ctx, projectId, "cpu.used_pcnt", base.Add(-time.Minute), base.Add(time.Hour), 60, "avg", nil, "server_name")
	if err != nil {
		t.Fatalf("QueryTimeSeries with groupBy failed: %v", err)
	}

	web1, ok := result["web-1"]
	if !ok {
		t.Fatal("expected 'web-1' series")
	}
	if len(web1) != 1 {
		t.Fatalf("expected 1 bucket for web-1, got %d", len(web1))
	}
	// avg of 50 and 60 = 55
	assertApproxEqual(t, "web-1 avg", web1[0].Value, 55.0, 0.1)

	web2, ok := result["web-2"]
	if !ok {
		t.Fatal("expected 'web-2' series")
	}
	if len(web2) != 1 {
		t.Fatalf("expected 1 bucket for web-2, got %d", len(web2))
	}
	assertApproxEqual(t, "web-2 avg", web2[0].Value, 80.0, 0.1)
}

func TestMetricPointRepository_QueryTimeSeries_WithTagFilter(t *testing.T) {
	setupTestDB(t)
	ctx := context.Background()
	projectId := uuid.New()
	base := time.Now().UTC().Truncate(time.Hour)

	points := []models.MetricPoint{
		makeMetricPoint(projectId, "cpu.used_pcnt", 50.0, map[string]string{"server_name": "web-1"}, base),
		makeMetricPoint(projectId, "cpu.used_pcnt", 80.0, map[string]string{"server_name": "web-2"}, base),
	}

	if err := MetricPointRepository.InsertAsync(ctx, points); err != nil {
		t.Fatalf("InsertAsync failed: %v", err)
	}

	filters := map[string]string{"server_name": "web-1"}
	result, err := MetricPointRepository.QueryTimeSeries(ctx, projectId, "cpu.used_pcnt", base.Add(-time.Minute), base.Add(time.Hour), 60, "avg", filters, "")
	if err != nil {
		t.Fatalf("QueryTimeSeries with tag filter failed: %v", err)
	}

	series, ok := result["__all__"]
	if !ok {
		t.Fatal("expected '__all__' series key")
	}
	if len(series) != 1 {
		t.Fatalf("expected 1 bucket (filtered to web-1 only), got %d", len(series))
	}
	assertApproxEqual(t, "filtered value", series[0].Value, 50.0, 0.1)
}

func TestMetricPointRepository_DiscoverMetrics(t *testing.T) {
	setupTestDB(t)
	ctx := context.Background()
	projectId := uuid.New()
	base := time.Now().UTC().Truncate(time.Hour)

	points := []models.MetricPoint{
		makeMetricPoint(projectId, "cpu.used_pcnt", 50.0, map[string]string{"server_name": "web-1"}, base),
		makeMetricPoint(projectId, "mem.used", 1024.0, map[string]string{"server_name": "web-1", "region": "us-east"}, base),
	}

	if err := MetricPointRepository.InsertAsync(ctx, points); err != nil {
		t.Fatalf("InsertAsync failed: %v", err)
	}

	metrics, err := MetricPointRepository.DiscoverMetrics(ctx, projectId, base.Add(-time.Minute), base.Add(time.Hour))
	if err != nil {
		t.Fatalf("DiscoverMetrics failed: %v", err)
	}

	if len(metrics) != 2 {
		t.Fatalf("expected 2 discovered metrics, got %d", len(metrics))
	}

	// Ordered by name ASC
	if metrics[0].Name != "cpu.used_pcnt" {
		t.Errorf("expected first metric 'cpu.used_pcnt', got %q", metrics[0].Name)
	}
	if metrics[1].Name != "mem.used" {
		t.Errorf("expected second metric 'mem.used', got %q", metrics[1].Name)
	}

	// mem.used should have 2 tag keys
	if len(metrics[1].TagKeys) != 2 {
		t.Errorf("expected 2 tag keys for mem.used, got %d", len(metrics[1].TagKeys))
	}
}

func TestMetricPointRepository_DiscoverTagValues(t *testing.T) {
	setupTestDB(t)
	ctx := context.Background()
	projectId := uuid.New()
	base := time.Now().UTC().Truncate(time.Hour)

	points := []models.MetricPoint{
		makeMetricPoint(projectId, "cpu.used_pcnt", 50.0, map[string]string{"server_name": "web-1"}, base),
		makeMetricPoint(projectId, "cpu.used_pcnt", 60.0, map[string]string{"server_name": "web-2"}, base.Add(time.Minute)),
		makeMetricPoint(projectId, "cpu.used_pcnt", 70.0, map[string]string{"server_name": "web-1"}, base.Add(2*time.Minute)),
	}

	if err := MetricPointRepository.InsertAsync(ctx, points); err != nil {
		t.Fatalf("InsertAsync failed: %v", err)
	}

	values, err := MetricPointRepository.DiscoverTagValues(ctx, projectId, "cpu.used_pcnt", "server_name", base.Add(-time.Minute), base.Add(time.Hour))
	if err != nil {
		t.Fatalf("DiscoverTagValues failed: %v", err)
	}

	if len(values) != 2 {
		t.Fatalf("expected 2 tag values, got %d", len(values))
	}

	// Ordered ASC
	if values[0] != "web-1" {
		t.Errorf("expected first value 'web-1', got %q", values[0])
	}
	if values[1] != "web-2" {
		t.Errorf("expected second value 'web-2', got %q", values[1])
	}
}

func TestMetricPointRepository_DiscoverTagValues_DottedKey(t *testing.T) {
	setupTestDB(t)
	ctx := context.Background()
	projectId := uuid.New()
	base := time.Now().UTC().Truncate(time.Hour)

	points := []models.MetricPoint{
		makeMetricPoint(projectId, "http.server.duration", 1.0, map[string]string{"http.route": "GET /a"}, base),
		makeMetricPoint(projectId, "http.server.duration", 2.0, map[string]string{"http.route": "GET /b"}, base.Add(time.Minute)),
		makeMetricPoint(projectId, "http.server.duration", 3.0, map[string]string{"http.route": "GET /a"}, base.Add(2*time.Minute)),
	}
	if err := MetricPointRepository.InsertAsync(ctx, points); err != nil {
		t.Fatalf("InsertAsync failed: %v", err)
	}

	values, err := MetricPointRepository.DiscoverTagValues(ctx, projectId, "http.server.duration", "http.route", base.Add(-time.Minute), base.Add(time.Hour))
	if err != nil {
		t.Fatalf("DiscoverTagValues failed: %v", err)
	}
	if len(values) != 2 || values[0] != "GET /a" || values[1] != "GET /b" {
		t.Errorf("dotted tag values = %v, want [GET /a GET /b] (dotted key must round-trip in SQLite mode)", values)
	}
}

func TestMetricPointRepository_GetAverageBetween(t *testing.T) {
	setupTestDB(t)
	ctx := context.Background()
	projectId := uuid.New()
	base := time.Now().UTC().Truncate(time.Hour)

	points := []models.MetricPoint{
		makeMetricPoint(projectId, "cpu.used_pcnt", 40.0, nil, base),
		makeMetricPoint(projectId, "cpu.used_pcnt", 60.0, nil, base.Add(time.Minute)),
		makeMetricPoint(projectId, "cpu.used_pcnt", 80.0, nil, base.Add(2*time.Minute)),
	}

	if err := MetricPointRepository.InsertAsync(ctx, points); err != nil {
		t.Fatalf("InsertAsync failed: %v", err)
	}

	avg, err := MetricPointRepository.GetAverageBetween(ctx, projectId, "cpu.used_pcnt", base.Add(-time.Minute), base.Add(time.Hour))
	if err != nil {
		t.Fatalf("GetAverageBetween failed: %v", err)
	}

	// avg = (40+60+80)/3 = 60
	assertApproxEqual(t, "average", avg, 60.0, 0.1)
}

func TestMetricPointRepository_GetAverageBetween_NoData(t *testing.T) {
	setupTestDB(t)
	ctx := context.Background()
	projectId := uuid.New()
	base := time.Now().UTC().Truncate(time.Hour)

	avg, err := MetricPointRepository.GetAverageBetween(ctx, projectId, "nonexistent.metric", base.Add(-time.Minute), base.Add(time.Hour))
	if err != nil {
		t.Fatalf("GetAverageBetween failed: %v", err)
	}

	assertApproxEqual(t, "average with no data", avg, 0.0, 0.001)
}

func TestMetricPointRepository_GetDistinctServers(t *testing.T) {
	setupTestDB(t)
	ctx := context.Background()
	projectId := uuid.New()
	base := time.Now().UTC().Truncate(time.Hour)

	points := []models.MetricPoint{
		makeMetricPoint(projectId, "cpu.used_pcnt", 50.0, map[string]string{"server_name": "web-1"}, base),
		makeMetricPoint(projectId, "cpu.used_pcnt", 60.0, map[string]string{"server_name": "web-2"}, base.Add(time.Minute)),
		makeMetricPoint(projectId, "cpu.used_pcnt", 70.0, map[string]string{"server_name": "web-1"}, base.Add(2*time.Minute)),
		makeMetricPoint(projectId, "mem.used", 1024.0, map[string]string{"server_name": "web-3"}, base.Add(3*time.Minute)),
	}

	if err := MetricPointRepository.InsertAsync(ctx, points); err != nil {
		t.Fatalf("InsertAsync failed: %v", err)
	}

	servers, err := MetricPointRepository.GetDistinctServers(ctx, projectId, base.Add(-time.Minute), base.Add(time.Hour))
	if err != nil {
		t.Fatalf("GetDistinctServers failed: %v", err)
	}

	if len(servers) != 3 {
		t.Fatalf("expected 3 distinct servers, got %d: %v", len(servers), servers)
	}
}

func TestMetricPointRepository_InsertEmpty(t *testing.T) {
	setupTestDB(t)
	ctx := context.Background()

	err := MetricPointRepository.InsertAsync(ctx, []models.MetricPoint{})
	if err != nil {
		t.Fatalf("InsertAsync with empty slice should not error: %v", err)
	}
}

func TestMetricPointRepository_QueryTimeSeries_Aggregations(t *testing.T) {
	setupTestDB(t)
	ctx := context.Background()
	projectId := uuid.New()
	base := time.Now().UTC().Truncate(time.Hour)

	points := []models.MetricPoint{
		makeMetricPoint(projectId, "response_time", 100.0, nil, base),
		makeMetricPoint(projectId, "response_time", 200.0, nil, base.Add(time.Minute)),
		makeMetricPoint(projectId, "response_time", 300.0, nil, base.Add(2*time.Minute)),
	}

	if err := MetricPointRepository.InsertAsync(ctx, points); err != nil {
		t.Fatalf("InsertAsync failed: %v", err)
	}

	from := base.Add(-time.Minute)
	to := base.Add(time.Hour)

	// Test min aggregation
	result, err := MetricPointRepository.QueryTimeSeries(ctx, projectId, "response_time", from, to, 60, "min", nil, "")
	if err != nil {
		t.Fatalf("QueryTimeSeries min failed: %v", err)
	}
	series := result["__all__"]
	if len(series) != 1 {
		t.Fatalf("expected 1 bucket, got %d", len(series))
	}
	assertApproxEqual(t, "min", series[0].Value, 100.0, 0.1)

	// Test max aggregation
	result, err = MetricPointRepository.QueryTimeSeries(ctx, projectId, "response_time", from, to, 60, "max", nil, "")
	if err != nil {
		t.Fatalf("QueryTimeSeries max failed: %v", err)
	}
	series = result["__all__"]
	assertApproxEqual(t, "max", series[0].Value, 300.0, 0.1)

	// Test sum aggregation
	result, err = MetricPointRepository.QueryTimeSeries(ctx, projectId, "response_time", from, to, 60, "sum", nil, "")
	if err != nil {
		t.Fatalf("QueryTimeSeries sum failed: %v", err)
	}
	series = result["__all__"]
	assertApproxEqual(t, "sum", series[0].Value, 600.0, 0.1)
}

func TestMetricPointRepository_QueryTimeSeries_LastAggregation(t *testing.T) {
	setupTestDB(t)
	ctx := context.Background()
	projectId := uuid.New()
	base := time.Now().UTC().Truncate(time.Hour)

	points := []models.MetricPoint{
		makeMetricPoint(projectId, "response_time", 100.0, nil, base),
		makeMetricPoint(projectId, "response_time", 300.0, nil, base.Add(time.Minute)),
		makeMetricPoint(projectId, "response_time", 250.0, nil, base.Add(2*time.Minute)),
	}

	if err := MetricPointRepository.InsertAsync(ctx, points); err != nil {
		t.Fatalf("InsertAsync failed: %v", err)
	}

	from := base.Add(-time.Minute)
	to := base.Add(time.Hour)

	result, err := MetricPointRepository.QueryTimeSeries(ctx, projectId, "response_time", from, to, 60, "last", nil, "")
	if err != nil {
		t.Fatalf("QueryTimeSeries last failed: %v", err)
	}
	series := result["__all__"]
	if len(series) != 1 {
		t.Fatalf("expected 1 bucket, got %d", len(series))
	}
	assertApproxEqual(t, "last", series[0].Value, 250.0, 0.1)
}

func TestMetricPointRepository_QueryTimeSeries_LastAggregationWithGroupBy(t *testing.T) {
	setupTestDB(t)
	ctx := context.Background()
	projectId := uuid.New()
	base := time.Now().UTC().Truncate(time.Hour)

	points := []models.MetricPoint{
		makeMetricPoint(projectId, "disk.used", 10.0, map[string]string{"disk": "sda"}, base),
		makeMetricPoint(projectId, "disk.used", 50.0, map[string]string{"disk": "sda"}, base.Add(time.Minute)),
		makeMetricPoint(projectId, "disk.used", 40.0, map[string]string{"disk": "sda"}, base.Add(2*time.Minute)),
		makeMetricPoint(projectId, "disk.used", 90.0, map[string]string{"disk": "sdb"}, base.Add(time.Minute)),
		makeMetricPoint(projectId, "disk.used", 70.0, map[string]string{"disk": "sdb"}, base.Add(3*time.Minute)),
	}

	if err := MetricPointRepository.InsertAsync(ctx, points); err != nil {
		t.Fatalf("InsertAsync failed: %v", err)
	}

	from := base.Add(-time.Minute)
	to := base.Add(time.Hour)

	result, err := MetricPointRepository.QueryTimeSeries(ctx, projectId, "disk.used", from, to, 60, "last", nil, "disk")
	if err != nil {
		t.Fatalf("QueryTimeSeries last with groupBy failed: %v", err)
	}
	if len(result["sda"]) != 1 || len(result["sdb"]) != 1 {
		t.Fatalf("expected 1 bucket per group, got sda=%d sdb=%d", len(result["sda"]), len(result["sdb"]))
	}
	assertApproxEqual(t, "last sda", result["sda"][0].Value, 40.0, 0.1)
	assertApproxEqual(t, "last sdb", result["sdb"][0].Value, 70.0, 0.1)
}
