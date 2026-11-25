//go:build telemetry_ch

package telemetry

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/tracewayapp/traceway/backend/v2/app/chdb"
	"github.com/tracewayapp/traceway/backend/v2/app/models"
	"github.com/tracewayapp/traceway/backend/v2/app/repositories/telemetry/shared"
)

// ClickHouse compares a Map in the order its entries were written, and the rollup
// tables carry tags in their sorting key, so a series written in two key orders is
// stored as two series that never merge (issue #448).

var manyTags = map[string]string{
	"server_name": "node-1", "host.name": "node-1", "host.id": "i-0abc", "os.type": "linux",
	"k8s.cluster.name": "prod", "k8s.node.name": "node-1", "device": "eth0", "direction": "receive",
	"état": "prêt", "Zone": "eu-1",
}

func storedSeriesCount(t *testing.T, table string, projectId uuid.UUID, key string) (stored, distinct uint64) {
	t.Helper()
	query := "SELECT count(), uniqExact(mapSort(tags)) FROM (SELECT tags FROM " + table + " WHERE project_id = ? GROUP BY " + key + ")"
	if err := chdb.Conn.QueryRow(context.Background(), query, projectId).Scan(&stored, &distinct); err != nil {
		t.Fatalf("%s: %v", table, err)
	}
	return stored, distinct
}

func TestMetricPointRepository_WritesOneRollupRowPerSeries(t *testing.T) {
	setupTestDB(t)
	ctx := context.Background()
	projectId := uuid.New()
	base := time.Now().UTC().Truncate(time.Hour).Add(-time.Hour)

	for batch := 0; batch < 6; batch++ {
		var points []models.MetricPoint
		for i := 0; i < 10; i++ {
			points = append(points, makeMetricPoint(projectId, "system.network.io", float64(batch*10+i), manyTags, base.Add(time.Duration(batch*10+i)*time.Second)))
		}
		if err := MetricPointRepository.InsertAsync(ctx, points); err != nil {
			t.Fatalf("InsertAsync failed: %v", err)
		}
	}

	var unsorted uint64
	if err := chdb.Conn.QueryRow(ctx, "SELECT countIf(tags != mapSort(tags)) FROM metric_points WHERE project_id = ?", projectId).Scan(&unsorted); err != nil {
		t.Fatal(err)
	}
	if unsorted != 0 {
		t.Errorf("%d of 60 raw points were written with tags out of mapSort order", unsorted)
	}
	for table, buckets := range map[string]uint64{"metric_points_1m": 1, "metric_points_1h": 1, "metric_points_1d": 1} {
		stored, distinct := storedSeriesCount(t, table, projectId, "name, recorded_at, tags")
		if distinct != buckets || stored != distinct {
			t.Errorf("%s holds %d rows for %d series", table, stored, distinct)
		}
	}

	result, err := MetricPointRepository.QueryTimeSeries(ctx, projectId, "system.network.io", base.Add(-48*time.Hour), base.Add(time.Hour), 60, "sum", map[string]string{"état": "prêt"}, "Zone", 0)
	if err != nil {
		t.Fatalf("QueryTimeSeries failed: %v", err)
	}
	if len(result["eu-1"]) != 1 || result["eu-1"][0].Value != 1770 {
		t.Fatalf("the rollup must still answer by tag, got %+v", result)
	}
}

// A writer that does not sort (an older instance during a rolling deploy) must not
// split the rollups either: the views sort on their own.
func TestMetricRollupViewsSortTags(t *testing.T) {
	setupTestDB(t)
	ctx := context.Background()
	projectId := uuid.New()
	at := time.Now().UTC().Truncate(time.Hour).Add(-time.Hour).Format("2006-01-02 15:04:05")
	keys := []string{"server_name", "host.name", "device", "direction", "os.type", "Zone", "état"}

	for order := range keys {
		ordered := rotated(keys, order)
		values := make([]string, len(ordered))
		for i, key := range ordered {
			values[i] = manyTags[key]
		}
		if err := chdb.Conn.Exec(ctx, "INSERT INTO metric_points (project_id, name, value, tags, recorded_at) SELECT toUUID(?), 'system.network.io', 1, mapFromArrays(?, ?), toDateTime64(?, 3)",
			projectId.String(), ordered, values, at); err != nil {
			t.Fatal(err)
		}
		if err := chdb.Conn.Exec(ctx, "INSERT INTO metric_histograms (project_id, name, recorded_at, tags, cnt, total, min_val, max_val, zero_cnt, buckets, weight_squares) "+
			"SELECT toUUID(?), 'http.server.request.duration', toDateTime(?), mapFromArrays(?, ?), 1, 100, 100, 100, 0, ([toInt32(53)], [1.]), 1",
			projectId.String(), at, ordered, values); err != nil {
			t.Fatal(err)
		}
	}

	histogramKey := "name, recorded_at, " + strings.Join(shared.HistogramDimensionColumnOrder, ", ") + ", tags"
	for table, key := range map[string]string{
		"metric_points_1m": "name, recorded_at, tags", "metric_points_1h": "name, recorded_at, tags", "metric_points_1d": "name, recorded_at, tags",
		"metric_histograms_1h": histogramKey, "metric_histograms_1d": histogramKey,
	} {
		stored, distinct := storedSeriesCount(t, table, projectId, key)
		if stored != 1 || distinct != 1 {
			t.Errorf("%s holds %d rows for %d series", table, stored, distinct)
		}
	}
	var count uint64
	var sum float64
	if err := chdb.Conn.QueryRow(ctx, "SELECT countMerge(count_val), sumMerge(sum_val) FROM metric_points_1h WHERE project_id = ?", projectId).Scan(&count, &sum); err != nil {
		t.Fatal(err)
	}
	if count != uint64(len(keys)) || sum != float64(len(keys)) {
		t.Errorf("the hourly rollup holds count %d sum %v, want %d of each", count, sum, len(keys))
	}
}

func insertRollupRow(t *testing.T, table string, projectId uuid.UUID, name string, keys []string, at time.Time, value float64) {
	t.Helper()
	values := make([]string, len(keys))
	for i, key := range keys {
		values[i] = manyTags[key]
	}
	err := chdb.Conn.Exec(context.Background(),
		"INSERT INTO "+table+" SELECT toUUID(?), ?, mapFromArrays(?, ?), toDateTime64(?, 3), minState(v), maxState(v), sumState(v), countState() FROM (SELECT toFloat64(?) AS v)",
		projectId.String(), name, keys, values, at.Format("2006-01-02 15:04:05"), value)
	if err != nil {
		t.Fatalf("insert into %s: %v", table, err)
	}
}

func rotated(keys []string, by int) []string {
	by %= len(keys)
	return append(append([]string{}, keys[by:]...), keys[:by]...)
}

// Rows written before the fix stay in the rollups for up to a year, so a read must
// treat every key order of a series as that series.
func TestMetricPointRepository_RateReadsRollupRowsOfAnyTagOrder(t *testing.T) {
	setupTestDB(t)
	ctx := context.Background()
	projectId := uuid.New()
	base := time.Now().UTC().Truncate(time.Hour).Add(-6 * time.Hour)
	keys := []string{"server_name", "host.name", "device", "direction", "os.type", "Zone"}

	// A counter growing by 600 an hour. Every hour holds the series in six key orders,
	// each with the largest value that order happened to see.
	for hour := 0; hour < 5; hour++ {
		for order := 0; order < 6; order++ {
			value := float64(hour*600 + order*100)
			insertRollupRow(t, "metric_points_1h", projectId, "system.network.io", rotated(keys, order), base.Add(time.Duration(hour)*time.Hour), value)
		}
	}

	result, err := MetricPointRepository.QueryTimeSeries(ctx, projectId, "system.network.io", base.Add(-4*24*time.Hour), base.Add(5*time.Hour), 60, "rate", nil, "server_name", 0)
	if err != nil {
		t.Fatalf("QueryTimeSeries failed: %v", err)
	}
	series := result["node-1"]
	if len(series) != 4 {
		t.Fatalf("expected 4 hourly buckets, got %d: %+v", len(series), series)
	}
	for _, p := range series {
		assertApproxEqual(t, "hourly rate "+p.Timestamp.Format(time.TimeOnly), p.Value, 600.0/3600, 0.0001)
	}
}

func TestMetricHistogramRepository_WritesOneRowPerSeries(t *testing.T) {
	setupTestDB(t)
	ctx := context.Background()
	projectId := uuid.New()
	minute := time.Now().UTC().Add(-10 * time.Minute).Truncate(time.Minute)
	dims := map[string]string{"service.version": "4.2", "url.path": "/orders/:id"}
	for key, value := range manyTags {
		dims[key] = value
	}

	for flush := 0; flush < 30; flush++ {
		if err := MetricHistogramRepository.InsertCells(ctx, []MetricHistogramCell{histogramCell(projectId, "http.server.request.duration", minute, dims, 100)}); err != nil {
			t.Fatal(err)
		}
	}

	key := "name, recorded_at, " + strings.Join(shared.HistogramDimensionColumnOrder, ", ") + ", tags"
	for _, table := range []string{"metric_histograms", "metric_histograms_1h", "metric_histograms_1d"} {
		stored, distinct := storedSeriesCount(t, table, projectId, key)
		if stored != 1 || distinct != 1 {
			t.Errorf("%s holds %d rows for %d series", table, stored, distinct)
		}
	}
	series, _, err := MetricHistogramRepository.Query(ctx, projectId, "http.server.request.duration", minute.Add(-time.Hour), minute.Add(time.Hour), 60, map[string]string{"état": "prêt"}, []string{"Zone"}, 200)
	if err != nil {
		t.Fatal(err)
	}
	if len(series["eu-1"]) != 1 || series["eu-1"][0].Hist.Count != 30 {
		t.Fatalf("series %+v", series)
	}
}

func TestMetricHistogramRepository_HotSeriesCountsASeriesOnceWhateverItsTagOrder(t *testing.T) {
	setupTestDB(t)
	ctx := context.Background()
	projectId := uuid.New()
	hour := time.Now().UTC().Truncate(time.Hour)
	keys := []string{"server_name", "host.name", "device", "direction", "os.type", "Zone"}

	insert := func(order int, path string) {
		t.Helper()
		ordered := rotated(keys, order)
		values := make([]string, len(ordered))
		for i, key := range ordered {
			values[i] = manyTags[key]
		}
		err := chdb.Conn.Exec(ctx,
			"INSERT INTO metric_histograms_1h (project_id, name, recorded_at, view, tags, cnt, total, min_val, max_val, zero_cnt, buckets, weight_squares) "+
				"SELECT toUUID(?), 'http.server.request.duration', toDateTime(?), ?, mapFromArrays(?, ?), 1, 100, 100, 100, 0, ([toInt32(53)], [1.]), 1",
			projectId.String(), hour.Format("2006-01-02 15:04:05"), path, ordered, values)
		if err != nil {
			t.Fatal(err)
		}
	}
	for path := 0; path < 3; path++ {
		for order := 0; order < 6; order++ {
			insert(order, fmt.Sprintf("/page-%d", path))
		}
	}

	if sets, err := MetricHistogramRepository.HotSeries(ctx, hour.Truncate(24*time.Hour), 4, 100); err != nil || len(sets) != 0 {
		t.Fatalf("3 series in 6 key orders each were counted as 4 or more: %+v %v", sets, err)
	}
	sets, err := MetricHistogramRepository.HotSeries(ctx, hour.Truncate(24*time.Hour), 3, 3)
	if err != nil || len(sets) != 1 {
		t.Fatalf("hot series %+v %v", sets, err)
	}
	paths := map[string]bool{}
	for _, dims := range sets[0].Dims {
		paths[dims["url.path"]] = true
	}
	if len(paths) != 3 {
		t.Errorf("a limit of 3 must return the 3 series, got %v", paths)
	}
}
