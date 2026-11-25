//go:build telemetry_ch

package telemetry

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/tracewayapp/traceway/backend/v2/app/chdb"
)

var repairedRollups = []string{"metric_points_1h", "metric_points_1d", "metric_histograms_1h", "metric_histograms_1d"}

func repairLeftovers(t *testing.T) []string {
	t.Helper()
	rows, err := chdb.Conn.Query(context.Background(), "SELECT name FROM system.tables WHERE database = currentDatabase() AND (name LIKE '%\\_tagrepair\\_%' OR name LIKE '%\\_tagbuild') ORDER BY name")
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var names []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			t.Fatal(err)
		}
		names = append(names, name)
	}
	return names
}

func emptyRollups(t *testing.T) {
	t.Helper()
	ctx := context.Background()
	for _, name := range repairLeftovers(t) {
		if err := chdb.Conn.Exec(ctx, "DROP TABLE IF EXISTS "+name+" SYNC"); err != nil {
			t.Fatal(err)
		}
	}
	for _, table := range repairedRollups {
		if err := chdb.Conn.Exec(ctx, "TRUNCATE TABLE "+table); err != nil {
			t.Fatal(err)
		}
	}
}

// seedSplitRollups fills the four repaired tables with series split over six key
// orders, next to rows that are already sorted, in this month's and last month's
// partition, and returns how many (table, partition) pairs hold unsorted rows.
func seedSplitRollups(t *testing.T, projectId uuid.UUID) int {
	t.Helper()
	ctx := context.Background()
	emptyRollups(t)

	keys := []string{"server_name", "host.name", "device", "direction", "os.type", "Zone"}
	thisMonth := time.Now().UTC().Truncate(24 * time.Hour)
	lastMonth := time.Date(thisMonth.Year(), thisMonth.Month(), 1, 0, 0, 0, 0, time.UTC).AddDate(0, -1, 5)
	for _, day := range []time.Time{lastMonth, lastMonth.AddDate(0, 0, 1), thisMonth} {
		at := day.Format("2006-01-02 15:04:05")
		for series := 0; series < 3; series++ {
			// Order 6 is the sorted one: such rows stay where they are and absorb the merged copies.
			for order := 0; order <= 6; order++ {
				ordered := rotated(keys, order)
				if order == 6 {
					ordered = []string{"Zone", "device", "direction", "host.name", "os.type", "server_name"}
				}
				values := make([]string, len(ordered))
				for i, key := range ordered {
					values[i] = manyTags[key]
					if key == "server_name" {
						values[i] = fmt.Sprintf("node-%d", series)
					}
				}
				value := float64(series*100 + order)
				for _, table := range []string{"metric_points_1h", "metric_points_1d"} {
					if err := chdb.Conn.Exec(ctx, "INSERT INTO "+table+" SELECT toUUID(?), 'system.network.io', mapFromArrays(?, ?), toDateTime64(?, 3), minState(v), maxState(v), sumState(v), countState() FROM (SELECT toFloat64(?) AS v)",
						projectId.String(), ordered, values, at, value); err != nil {
						t.Fatal(err)
					}
				}
				// Half of the unsorted rows carry no weight and count as cnt * 1000. The sorted
				// row has one: a stored 0 next to a merged copy would be lost in the next merge,
				// and no real row has it, since the column came with the tables and defaults to
				// cnt * 1000.
				weight := float64((order + 1) % 2)
				for _, table := range []string{"metric_histograms_1h", "metric_histograms_1d"} {
					if err := chdb.Conn.Exec(ctx, "INSERT INTO "+table+" (project_id, name, recorded_at, view, tags, cnt, total, min_val, max_val, zero_cnt, buckets, weight_squares) "+
						"SELECT toUUID(?), 'http.server.request.duration', toDateTime(?), '/orders', mapFromArrays(?, ?), 2, ?, ?, ?, 1, ([toInt32(?), toInt32(60)], [1., 1.]), ?",
						projectId.String(), at, ordered, values, value*2, value, value+1, 50+order, weight); err != nil {
						t.Fatal(err)
					}
				}
			}
		}
	}

	var pairs uint64
	for _, table := range repairedRollups {
		var partitions uint64
		if err := chdb.Conn.QueryRow(ctx, "SELECT uniqExact(_partition_id) FROM "+table+" WHERE tags != mapSort(tags)").Scan(&partitions); err != nil {
			t.Fatal(err)
		}
		pairs += partitions
	}
	return int(pairs)
}

// rollupFingerprint is what a reader sees per series and bucket, whatever the rows behind it.
func rollupFingerprint(t *testing.T, projectId uuid.UUID) map[string]uint64 {
	t.Helper()
	points := "SELECT cityHash64(groupArray((k, c, s, mx, mn))) FROM (SELECT (recorded_at, mapSort(tags)) AS k, countMerge(count_val) AS c, sumMerge(sum_val) AS s, maxMerge(max_val) AS mx, minMerge(min_val) AS mn FROM %s WHERE project_id = ? GROUP BY k ORDER BY k)"
	histograms := "SELECT cityHash64(groupArray((k, c, s, mn, mx, z, b, w))) FROM (SELECT (recorded_at, view, mapSort(tags)) AS k, sum(cnt) AS c, sum(total) AS s, min(min_val) AS mn, max(max_val) AS mx, sum(zero_cnt) AS z, " +
		"sumMap(buckets.1, buckets.2) AS b, sum(if(weight_squares > 0, weight_squares, cnt * 1000)) AS w FROM %s WHERE project_id = ? GROUP BY k ORDER BY k)"
	prints := map[string]uint64{}
	for table, query := range map[string]string{"metric_points_1h": points, "metric_points_1d": points, "metric_histograms_1h": histograms, "metric_histograms_1d": histograms} {
		var print uint64
		if err := chdb.Conn.QueryRow(context.Background(), fmt.Sprintf(query, table), projectId).Scan(&print); err != nil {
			t.Fatalf("%s: %v", table, err)
		}
		prints[table] = print
	}
	return prints
}

func assertRollupsRepaired(t *testing.T, projectId uuid.UUID, want map[string]uint64) {
	t.Helper()
	for table, print := range rollupFingerprint(t, projectId) {
		if print != want[table] {
			t.Errorf("%s reads differently after the repair", table)
		}
	}
	// And once ClickHouse has merged the copies with the rows that were already sorted,
	// which it does in the background at a time of its own choosing.
	for _, table := range repairedRollups {
		if err := chdb.Conn.Exec(context.Background(), "OPTIMIZE TABLE "+table+" FINAL"); err != nil {
			t.Fatal(err)
		}
	}
	for table, print := range rollupFingerprint(t, projectId) {
		if print != want[table] {
			t.Errorf("%s reads differently once the repaired rows are merged", table)
		}
	}
	for _, table := range repairedRollups {
		var rows, series, unsorted uint64
		// Three days of three series; FINAL because the merged copies and the rows that were already sorted merge in the background.
		if err := chdb.Conn.QueryRow(context.Background(), "SELECT count(), uniqExact(recorded_at, mapSort(tags)), countIf(tags != mapSort(tags)) FROM "+table+" FINAL WHERE project_id = ?", projectId).Scan(&rows, &series, &unsorted); err != nil {
			t.Fatal(err)
		}
		if rows != 9 || series != 9 || unsorted != 0 {
			t.Errorf("%s holds %d rows for %d series buckets, %d of them unsorted", table, rows, series, unsorted)
		}
	}
	if left := repairLeftovers(t); len(left) != 0 {
		t.Errorf("staging tables left behind: %v", left)
	}
}

func TestRepairMetricRollups_MergesSplitRowsAndKeepsTheNumbers(t *testing.T) {
	setupTestDB(t)
	ctx := context.Background()
	projectId := uuid.New()
	pairs := seedSplitRollups(t, projectId)
	before := rollupFingerprint(t, projectId)

	report, err := RepairMetricRollups(ctx, func(string) error { return nil })
	if err != nil {
		t.Fatal(err)
	}
	// 3 days of 3 series in 4 tables: 6 unsorted rows each, merged into one.
	if report.Partitions != pairs || report.RowsBefore != 216 || report.RowsAfter != 36 || len(report.Skipped) != 0 {
		t.Errorf("report %+v, want %d partitions, 216 rows into 36", report, pairs)
	}
	assertRollupsRepaired(t, projectId, before)

	again, err := RepairMetricRollups(ctx, func(string) error { return nil })
	if err != nil || again.Partitions != 0 || again.RowsBefore != 0 {
		t.Errorf("a second pass must find nothing, got %+v %v", again, err)
	}
	assertRollupsRepaired(t, projectId, before)
}

func TestRepairMetricRollups_ResumesAfterStoppingAtAnyStep(t *testing.T) {
	setupTestDB(t)
	ctx := context.Background()
	stopped := errors.New("stopped")
	for _, step := range []string{"stage", "publish", "move", "delete", "drop"} {
		t.Run(step, func(t *testing.T) {
			projectId := uuid.New()
			seedSplitRollups(t, projectId)
			before := rollupFingerprint(t, projectId)

			// Stop at the step's second visit, so one partition is complete and the next is cut short.
			visits := 0
			_, err := RepairMetricRollups(ctx, func(asked string) error {
				if asked == step {
					if visits++; visits == 2 {
						return stopped
					}
				}
				return nil
			})
			if !errors.Is(err, stopped) {
				t.Fatalf("the pass must stop with the guard's error, got %v", err)
			}

			if _, err := RepairMetricRollups(ctx, func(string) error { return nil }); err != nil {
				t.Fatal(err)
			}
			assertRollupsRepaired(t, projectId, before)
		})
	}
}

func TestRepairMetricRollups_RefusesWhileAViewDoesNotSort(t *testing.T) {
	setupTestDB(t)
	ctx := context.Background()
	projectId := uuid.New()
	seedSplitRollups(t, projectId)
	before := rollupFingerprint(t, projectId)

	unsorted := "ALTER TABLE metric_points_1d_mv MODIFY QUERY SELECT project_id, name, tags, toStartOfDay(recorded_at) AS recorded_at, minState(value) AS min_val, maxState(value) AS max_val, sumState(value) AS sum_val, countState() AS count_val FROM metric_points GROUP BY project_id, name, tags, recorded_at"
	sorted := "ALTER TABLE metric_points_1d_mv MODIFY QUERY SELECT project_id, name, mapSort(source.tags) AS tags, toStartOfDay(source.recorded_at) AS recorded_at, minState(value) AS min_val, maxState(value) AS max_val, sumState(value) AS sum_val, countState() AS count_val FROM metric_points AS source GROUP BY project_id, name, mapSort(source.tags), toStartOfDay(source.recorded_at)"
	if err := chdb.Conn.Exec(ctx, unsorted); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := chdb.Conn.Exec(ctx, sorted); err != nil {
			t.Errorf("restoring the view: %v", err)
		}
	})

	report, err := RepairMetricRollups(ctx, func(string) error { return nil })
	if err == nil || report.Partitions != 0 {
		t.Fatalf("a view that writes unsorted rows must stop the repair before it changes anything, got %+v %v", report, err)
	}
	for table, print := range rollupFingerprint(t, projectId) {
		if print != before[table] {
			t.Errorf("%s was changed", table)
		}
	}
	if left := repairLeftovers(t); len(left) != 0 {
		t.Errorf("staging tables left behind: %v", left)
	}
}

// The repair reads the last year and nothing before it. A row a day older keeps its key
// order even where a repaired day shares its partition: it has no merged copy, so the
// delete must not take it.
func TestRepairMetricRollups_ReadsExactlyTheLastYear(t *testing.T) {
	setupTestDB(t)
	ctx := context.Background()
	projectId := uuid.New()
	emptyRollups(t)

	// Days as the server counts them, which is how the repair bounds its window.
	var today string
	if err := chdb.Conn.QueryRow(ctx, "SELECT toString(today())").Scan(&today); err != nil {
		t.Fatal(err)
	}
	end, err := time.Parse("2006-01-02", today)
	if err != nil {
		t.Fatal(err)
	}
	firstDay := end.AddDate(-1, 0, 0).Format("2006-01-02")
	dayBefore := end.AddDate(-1, 0, -1).Format("2006-01-02")
	// The hourly rollups expire within a year, so only the daily ones can hold such rows.
	tables := []string{"metric_points_1d", "metric_histograms_1d"}
	values := []string{"1", "2", "3"}
	for _, day := range []string{firstDay, dayBefore} {
		for order, keys := range [][]string{{"b", "a", "c"}, {"c", "b", "a"}} {
			ordered := make([]string, len(keys))
			for i, key := range keys {
				ordered[i] = values[key[0]-'a']
			}
			if err := chdb.Conn.Exec(ctx, "INSERT INTO metric_points_1d SELECT toUUID(?), 'system.network.io', mapFromArrays(?, ?), toDateTime64(?, 3), minState(v), maxState(v), sumState(v), countState() FROM (SELECT toFloat64(?) AS v)",
				projectId.String(), keys, ordered, day+" 00:00:00", float64(order+1)); err != nil {
				t.Fatal(err)
			}
			if err := chdb.Conn.Exec(ctx, "INSERT INTO metric_histograms_1d (project_id, name, recorded_at, view, tags, cnt, total, min_val, max_val, zero_cnt, buckets, weight_squares) "+
				"SELECT toUUID(?), 'http.server.request.duration', toDateTime(?), '/orders', mapFromArrays(?, ?), 2, 4, 1, 3, 1, ([toInt32(?), toInt32(60)], [1., 1.]), 1",
				projectId.String(), day+" 00:00:00", keys, ordered, 50+order); err != nil {
				t.Fatal(err)
			}
		}
	}
	before := rollupFingerprint(t, projectId)

	report, err := RepairMetricRollups(ctx, func(string) error { return nil })
	if err != nil {
		t.Fatal(err)
	}
	if report.Partitions != len(tables) || report.RowsBefore != 4 || report.RowsAfter != 2 || len(report.Skipped) != 0 {
		t.Errorf("report %+v, want %d partitions, 4 rows into 2", report, len(tables))
	}
	for table, print := range rollupFingerprint(t, projectId) {
		if print != before[table] {
			t.Errorf("%s reads differently after the repair", table)
		}
	}
	for _, table := range tables {
		for day, want := range map[string][2]uint64{firstDay: {1, 0}, dayBefore: {2, 2}} {
			var rows, unsorted uint64
			if err := chdb.Conn.QueryRow(ctx, "SELECT count(), countIf(tags != mapSort(tags)) FROM "+table+" FINAL WHERE project_id = ? AND toDate(recorded_at) = toDate(?)", projectId, day).Scan(&rows, &unsorted); err != nil {
				t.Fatal(err)
			}
			if rows != want[0] || unsorted != want[1] {
				t.Errorf("%s on %s holds %d rows, %d of them unsorted, want %d and %d", table, day, rows, unsorted, want[0], want[1])
			}
		}
	}
	if left := repairLeftovers(t); len(left) != 0 {
		t.Errorf("staging tables left behind: %v", left)
	}

	again, err := RepairMetricRollups(ctx, func(string) error { return nil })
	if err != nil || again.Partitions != 0 || again.RowsBefore != 0 {
		t.Errorf("a second pass must find nothing in the last year, got %+v %v", again, err)
	}
}

// The staging table is dropped at the end, so it must be empty by then. One whose rows
// the move does not take, here because they belong to another partition than its name
// says, stops the pass and stays.
func TestRepairMetricRollups_KeepsAStagingTableThatStillHoldsRows(t *testing.T) {
	setupTestDB(t)
	ctx := context.Background()
	emptyRollups(t)

	staged := "metric_points_1d_tagrepair_202609_20260901"
	if err := chdb.Conn.Exec(ctx, "CREATE TABLE "+staged+" AS metric_points_1d"); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = chdb.Conn.Exec(ctx, "DROP TABLE IF EXISTS "+staged+" SYNC") })
	if err := chdb.Conn.Exec(ctx, "INSERT INTO "+staged+" SELECT toUUID(?), 'system.network.io', map('a', '1'), toDateTime64('2026-08-15 00:00:00', 3), minState(v), maxState(v), sumState(v), countState() FROM (SELECT toFloat64(1) AS v)",
		uuid.New().String()); err != nil {
		t.Fatal(err)
	}

	report, err := RepairMetricRollups(ctx, func(string) error { return nil })
	if err == nil || report.Partitions != 0 {
		t.Fatalf("a staging table that keeps its rows must stop the pass, got %+v %v", report, err)
	}
	var rows uint64
	if err := chdb.Conn.QueryRow(ctx, "SELECT count() FROM "+staged).Scan(&rows); err != nil || rows != 1 {
		t.Errorf("the staging table must be kept with its row, got %d rows (%v)", rows, err)
	}
}
