package telemetry

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/tracewayapp/traceway/backend/app/models"
	"github.com/tracewayapp/traceway/backend/app/repositories/telemetry/shared"
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

	result, err := MetricPointRepository.QueryTimeSeries(ctx, projectId, "cpu.used_pcnt", base.Add(-time.Minute), base.Add(time.Hour), 30, "avg", nil, "", 0)
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

func TestMetricPointRepository_QueryTimeSeries_GroupCap(t *testing.T) {
	setupTestDB(t)
	ctx := context.Background()
	projectId := uuid.New()
	base := time.Now().UTC().Truncate(time.Hour)

	var points []models.MetricPoint
	for _, server := range []string{"web-5", "web-1", "web-3", "web-2", "web-4"} {
		for i := 0; i < 3; i++ {
			points = append(points, makeMetricPoint(projectId, "cpu.used_pcnt", float64(10*i), map[string]string{"server_name": server}, base.Add(time.Duration(i)*10*time.Minute)))
		}
	}
	if err := MetricPointRepository.InsertAsync(ctx, points); err != nil {
		t.Fatalf("InsertAsync failed: %v", err)
	}

	capped, err := MetricPointRepository.QueryTimeSeries(ctx, projectId, "cpu.used_pcnt", base.Add(-time.Minute), base.Add(time.Hour), 10, "avg", nil, "server_name", 2)
	if err != nil {
		t.Fatalf("QueryTimeSeries failed: %v", err)
	}
	if len(capped) != 3 {
		t.Fatalf("expected 2 groups plus the marker, got %d: %v", len(capped), capped)
	}
	for _, server := range []string{"web-1", "web-2"} {
		if len(capped[server]) != 3 {
			t.Fatalf("group %s should be complete with 3 buckets, got %d", server, len(capped[server]))
		}
	}
	if marker, ok := capped["web-3"]; !ok || len(marker) != 0 {
		t.Fatalf("expected an empty marker for the first group past the cap, got %v", capped)
	}

	uncapped, err := MetricPointRepository.QueryTimeSeries(ctx, projectId, "cpu.used_pcnt", base.Add(-time.Minute), base.Add(time.Hour), 10, "avg", nil, "server_name", 0)
	if err != nil {
		t.Fatalf("QueryTimeSeries failed: %v", err)
	}
	if len(uncapped) != 5 {
		t.Fatalf("cap 0 must return every group, got %d", len(uncapped))
	}

	wide, err := MetricPointRepository.QueryTimeSeries(ctx, projectId, "cpu.used_pcnt", base.Add(-time.Minute), base.Add(time.Hour), 10, "avg", nil, "server_name", 5)
	if err != nil {
		t.Fatalf("QueryTimeSeries failed: %v", err)
	}
	if len(wide) != 5 {
		t.Fatalf("a cap equal to the cardinality must not add a marker, got %d", len(wide))
	}
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

	result, err := MetricPointRepository.QueryTimeSeries(ctx, projectId, "cpu.used_pcnt", base.Add(-time.Minute), base.Add(time.Hour), 60, "avg", nil, "server_name", 0)
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
	result, err := MetricPointRepository.QueryTimeSeries(ctx, projectId, "cpu.used_pcnt", base.Add(-time.Minute), base.Add(time.Hour), 60, "avg", filters, "", 0)
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

func TestMetricPointRepository_LatestPerServer(t *testing.T) {
	setupTestDB(t)
	ctx := context.Background()
	projectId := uuid.New()
	base := time.Now().UTC().Truncate(time.Second)

	points := []models.MetricPoint{
		makeMetricPoint(projectId, "cpu.used_pcnt", 10, map[string]string{"server_name": "api-1"}, base),
		makeMetricPoint(projectId, "cpu.used_pcnt", 30, map[string]string{"server_name": "api-1", "os.type": "linux"}, base.Add(2*time.Minute)),
		makeMetricPoint(projectId, "cpu.used_pcnt", 50, map[string]string{"server_name": "worker-1"}, base.Add(3*time.Minute)),
		makeMetricPoint(projectId, "mem.used", 99, map[string]string{"server_name": "api-1"}, base.Add(4*time.Minute)),
		makeMetricPoint(projectId, "cpu.used_pcnt", 70, nil, base.Add(5*time.Minute)),
		makeMetricPoint(uuid.New(), "cpu.used_pcnt", 80, map[string]string{"server_name": "other"}, base.Add(6*time.Minute)),
	}

	if err := MetricPointRepository.InsertAsync(ctx, points); err != nil {
		t.Fatalf("InsertAsync failed: %v", err)
	}

	latest, err := MetricPointRepository.LatestPerServer(ctx, projectId, "cpu.used_pcnt", base.Add(time.Minute))
	if err != nil {
		t.Fatalf("LatestPerServer failed: %v", err)
	}
	if len(latest) != 2 {
		t.Fatalf("LatestPerServer returned %d rows, want 2: %+v", len(latest), latest)
	}
	if latest[0].ServerName != "api-1" || latest[0].Value != 30 {
		t.Errorf("latest[0] = %+v, want api-1 value 30", latest[0])
	}
	if !latest[0].LastReportedAt.Equal(base.Add(2 * time.Minute)) {
		t.Errorf("api-1 timestamp = %s, want %s", latest[0].LastReportedAt, base.Add(2*time.Minute))
	}
	if latest[0].Tags["os.type"] != "linux" {
		t.Errorf("api-1 tags = %v, want os.type=linux", latest[0].Tags)
	}
	if latest[1].ServerName != "worker-1" || latest[1].Value != 50 {
		t.Errorf("latest[1] = %+v, want worker-1 value 50", latest[1])
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
	result, err := MetricPointRepository.QueryTimeSeries(ctx, projectId, "response_time", from, to, 60, "min", nil, "", 0)
	if err != nil {
		t.Fatalf("QueryTimeSeries min failed: %v", err)
	}
	series := result["__all__"]
	if len(series) != 1 {
		t.Fatalf("expected 1 bucket, got %d", len(series))
	}
	assertApproxEqual(t, "min", series[0].Value, 100.0, 0.1)

	// Test max aggregation
	result, err = MetricPointRepository.QueryTimeSeries(ctx, projectId, "response_time", from, to, 60, "max", nil, "", 0)
	if err != nil {
		t.Fatalf("QueryTimeSeries max failed: %v", err)
	}
	series = result["__all__"]
	assertApproxEqual(t, "max", series[0].Value, 300.0, 0.1)

	// Test sum aggregation
	result, err = MetricPointRepository.QueryTimeSeries(ctx, projectId, "response_time", from, to, 60, "sum", nil, "", 0)
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

	result, err := MetricPointRepository.QueryTimeSeries(ctx, projectId, "response_time", from, to, 60, "last", nil, "", 0)
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

	result, err := MetricPointRepository.QueryTimeSeries(ctx, projectId, "disk.used", from, to, 60, "last", nil, "disk", 0)
	if err != nil {
		t.Fatalf("QueryTimeSeries last with groupBy failed: %v", err)
	}
	if len(result["sda"]) != 1 || len(result["sdb"]) != 1 {
		t.Fatalf("expected 1 bucket per group, got sda=%d sdb=%d", len(result["sda"]), len(result["sdb"]))
	}
	assertApproxEqual(t, "last sda", result["sda"][0].Value, 40.0, 0.1)
	assertApproxEqual(t, "last sdb", result["sdb"][0].Value, 70.0, 0.1)
}

func TestMetricPointRepository_QueryTimeSeries_LastPerDeviceWithServerFilter(t *testing.T) {
	setupTestDB(t)
	ctx := context.Background()
	projectId := uuid.New()
	base := time.Now().UTC().Truncate(time.Hour)
	tags := func(server, device string) map[string]string {
		return map[string]string{"server_name": server, "device": device, "direction": "receive"}
	}

	points := []models.MetricPoint{
		makeMetricPoint(projectId, "system.network.io", 1000, tags("web-1", "eth0"), base),
		makeMetricPoint(projectId, "system.network.io", 1600, tags("web-1", "eth0"), base.Add(30*time.Second)),
		makeMetricPoint(projectId, "system.network.io", 2200, tags("web-1", "eth0"), base.Add(time.Minute)),
		makeMetricPoint(projectId, "system.network.io", 2800, tags("web-1", "eth0"), base.Add(90*time.Second)),
		makeMetricPoint(projectId, "system.network.io", 10, tags("web-1", "lo"), base),
		makeMetricPoint(projectId, "system.network.io", 20, tags("web-1", "lo"), base.Add(time.Minute)),
		makeMetricPoint(projectId, "system.network.io", 99999, tags("web-2", "eth0"), base.Add(time.Minute)),
	}
	if err := MetricPointRepository.InsertAsync(ctx, points); err != nil {
		t.Fatalf("InsertAsync failed: %v", err)
	}

	filters := map[string]string{"direction": "receive", "server_name": "web-1"}
	result, err := MetricPointRepository.QueryTimeSeries(ctx, projectId, "system.network.io", base.Add(-time.Minute), base.Add(2*time.Minute), 1, "last", filters, "device", 0)
	if err != nil {
		t.Fatalf("QueryTimeSeries last per device failed: %v", err)
	}
	if len(result) != 2 {
		t.Fatalf("expected eth0 and lo, got %v", result)
	}
	eth0, lo := result["eth0"], result["lo"]
	if len(eth0) != 2 || len(lo) != 2 {
		t.Fatalf("expected 2 buckets per device, got eth0=%d lo=%d", len(eth0), len(lo))
	}
	assertApproxEqual(t, "eth0 bucket 0", eth0[0].Value, 1600, 0.1)
	assertApproxEqual(t, "eth0 bucket 1", eth0[1].Value, 2800, 0.1)
	assertApproxEqual(t, "lo bucket 0", lo[0].Value, 10, 0.1)
	assertApproxEqual(t, "lo bucket 1", lo[1].Value, 20, 0.1)
	if gap := eth0[1].Timestamp.Sub(eth0[0].Timestamp); gap != time.Minute {
		t.Fatalf("bucket gap = %v, want 1m", gap)
	}
}

func TestMetricPointRepository_QueryTimeSeries_CompositeGroupBy(t *testing.T) {
	setupTestDB(t)
	ctx := context.Background()
	projectId := uuid.New()
	base := time.Now().UTC().Truncate(time.Hour)
	tags := func(server, device string) map[string]string {
		return map[string]string{"server_name": server, "device": device, "direction": "receive"}
	}

	points := []models.MetricPoint{
		makeMetricPoint(projectId, "system.network.io", 1000, tags("web-1", "eth0"), base),
		makeMetricPoint(projectId, "system.network.io", 2200, tags("web-1", "eth0"), base.Add(time.Minute)),
		makeMetricPoint(projectId, "system.network.io", 10, tags("web-1", "lo"), base),
		makeMetricPoint(projectId, "system.network.io", 20, tags("web-1", "lo"), base.Add(time.Minute)),
		makeMetricPoint(projectId, "system.network.io", 500, tags("web-2", "eth0"), base),
		makeMetricPoint(projectId, "system.network.io", 900, tags("web-2", "eth0"), base.Add(time.Minute)),
	}
	if err := MetricPointRepository.InsertAsync(ctx, points); err != nil {
		t.Fatalf("InsertAsync failed: %v", err)
	}

	result, err := MetricPointRepository.QueryTimeSeriesByTags(ctx, projectId, "system.network.io", base.Add(-time.Minute), base.Add(2*time.Minute), 1, "last", map[string]string{"direction": "receive"}, []string{"device", "server_name"}, 0)
	if err != nil {
		t.Fatalf("QueryTimeSeriesByTags failed: %v", err)
	}
	sep := shared.GroupKeySeparator
	want := map[string][2]float64{"eth0" + sep + "web-1": {1000, 2200}, "lo" + sep + "web-1": {10, 20}, "eth0" + sep + "web-2": {500, 900}}
	if len(result) != len(want) {
		t.Fatalf("expected one series per device and server, got %v", result)
	}
	for key, values := range want {
		if len(result[key]) != 2 {
			t.Fatalf("group %s: expected 2 buckets, got %d", key, len(result[key]))
		}
		assertApproxEqual(t, key+" bucket 0", result[key][0].Value, values[0], 0.1)
		assertApproxEqual(t, key+" bucket 1", result[key][1].Value, values[1], 0.1)
	}
}

func TestMetricPointRepository_QueryTimeSeries_GroupByKeyWithComma(t *testing.T) {
	setupTestDB(t)
	ctx := context.Background()
	projectId := uuid.New()
	base := time.Now().UTC().Truncate(time.Hour)

	points := []models.MetricPoint{
		makeMetricPoint(projectId, "cpu.used_pcnt", 50.0, map[string]string{"a,b": "x", "a": "wrong"}, base),
		makeMetricPoint(projectId, "cpu.used_pcnt", 80.0, map[string]string{"a,b": "y", "a": "wrong"}, base),
	}
	if err := MetricPointRepository.InsertAsync(ctx, points); err != nil {
		t.Fatalf("InsertAsync failed: %v", err)
	}

	result, err := MetricPointRepository.QueryTimeSeries(ctx, projectId, "cpu.used_pcnt", base.Add(-time.Minute), base.Add(time.Hour), 60, "avg", nil, "a,b", 0)
	if err != nil {
		t.Fatalf("QueryTimeSeries failed: %v", err)
	}
	if len(result) != 2 || len(result["x"]) != 1 || len(result["y"]) != 1 {
		t.Fatalf("a comma in groupBy must name one tag key literally, got %v", result)
	}
}

func TestMetricPointRepository_QueryTimeSeries_Rate(t *testing.T) {
	setupTestDB(t)
	ctx := context.Background()
	projectId := uuid.New()
	base := time.Now().UTC().Truncate(time.Hour)

	sample := func(server, device string, value float64, offset time.Duration) models.MetricPoint {
		return makeMetricPoint(projectId, "system.network.io", value, map[string]string{
			"server_name": server, "device": device, "direction": "receive",
		}, base.Add(offset))
	}
	points := []models.MetricPoint{
		sample("web-1", "eth0", 1000, 0), sample("web-1", "eth0", 1600, 30*time.Second),
		sample("web-1", "eth0", 2200, 60*time.Second), sample("web-1", "eth0", 2800, 90*time.Second),
		sample("web-1", "eth0", 3400, 120*time.Second),
		sample("web-1", "eth1", 500, 0), sample("web-1", "eth1", 800, 30*time.Second),
		sample("web-1", "eth1", 1100, 60*time.Second), sample("web-1", "eth1", 1400, 90*time.Second),
		sample("web-1", "eth1", 1700, 120*time.Second),
		sample("web-2", "eth0", 1000, 0), sample("web-2", "eth0", 1500, 30*time.Second),
		sample("web-2", "eth0", 100, 60*time.Second), sample("web-2", "eth0", 400, 90*time.Second),
	}
	if err := MetricPointRepository.InsertAsync(ctx, points); err != nil {
		t.Fatalf("InsertAsync failed: %v", err)
	}

	result, err := MetricPointRepository.QueryTimeSeries(ctx, projectId, "system.network.io", base, base.Add(3*time.Minute), 1, "rate", nil, "server_name", 0)
	if err != nil {
		t.Fatalf("QueryTimeSeries failed: %v", err)
	}

	web1 := result["web-1"]
	if len(web1) != 3 {
		t.Fatalf("expected 3 buckets for web-1, got %d: %+v", len(web1), web1)
	}
	assertApproxEqual(t, "web-1 first bucket", web1[0].Value, 15.0, 0.01)
	assertApproxEqual(t, "web-1 second bucket", web1[1].Value, 30.0, 0.01)
	assertApproxEqual(t, "web-1 third bucket", web1[2].Value, 15.0, 0.01)

	web2 := result["web-2"]
	if len(web2) != 2 {
		t.Fatalf("expected 2 buckets for web-2, got %d: %+v", len(web2), web2)
	}
	assertApproxEqual(t, "web-2 first bucket", web2[0].Value, 500.0/60, 0.01)
	assertApproxEqual(t, "web-2 reset bucket", web2[1].Value, 300.0/60, 0.01)

	later, err := MetricPointRepository.QueryTimeSeries(ctx, projectId, "system.network.io", base.Add(time.Minute), base.Add(3*time.Minute), 1, "rate", map[string]string{"server_name": "web-1"}, "", 0)
	if err != nil {
		t.Fatalf("QueryTimeSeries with lookback failed: %v", err)
	}
	all := later["__all__"]
	if len(all) != 2 {
		t.Fatalf("expected 2 buckets from the later range, got %d: %+v", len(all), all)
	}
	assertApproxEqual(t, "lookback bucket", all[0].Value, 30.0, 0.01)
}
