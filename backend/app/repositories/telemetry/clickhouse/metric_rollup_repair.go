//go:build telemetry_ch

package clickhouse

import (
	"context"
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/tracewayapp/traceway/backend/v2/app/chdb"
	"github.com/tracewayapp/traceway/backend/v2/app/config"
	"github.com/tracewayapp/traceway/backend/v2/app/repositories/telemetry/shared"
	"github.com/tracewayapp/traceway/backend/v2/app/traceway"
)

// Rollup rows written before tags were sorted (issue #448) hold one series under
// several key orders, and they stay until their TTL: a year in metric_points_1h,
// forever in metric_points_1d. RepairMetricRollups merges them, a partition at a time.
//
// Nothing is rewritten in place. The merged copies of a partition's unsorted rows are
// built in a scratch table, which is then renamed to <table>_tagrepair_<partition>:
// that name existing is the record that the copies are complete. From there every step
// can be repeated: MOVE PARTITION puts the copies into the table at once and leaves the
// staging table empty, so a second move adds nothing; the delete names the unsorted rows
// by what they are; and the staging table is dropped last. A run that dies anywhere is
// picked up by the next one without a row lost or counted twice.
//
// This holds only while nothing writes unsorted rows, which is what the sorted rollup
// views guarantee: the rows to merge can then only become fewer. The tables in front of
// the views, and the minute rollup, are left alone: their rows expire within 30 days.
//
// Every step is a log line with rollup_repair.* attributes and a traceway.rollup_repair.*
// metric, see docs/pages/server/repair-metric-rollups.mdx.

const (
	rollupRepairStaged   = "_tagrepair_"
	rollupRepairBuild    = "_tagbuild"
	rollupRepairUnsorted = "tags != mapSort(tags)"
	// A mutation reserves 1.1 times the part it rewrites, and several parts are rewritten at once.
	rollupRepairSpaceFactor   = 2
	rollupRepairDeleteRetries = 3
	rollupRepairFailedPolls   = 30
	rollupRepairMaxPoll       = 2 * time.Second
	rollupRepairProgressEvery = 30 * time.Second
	// A part that does not fit the free space is skipped without an error, so a mutation
	// that stops making progress is the only sign of a full disk.
	rollupRepairStall = 30 * time.Minute
)

var rollupRepairPartition = regexp.MustCompile(`^[0-9A-Za-z_-]+$`)

type rollupRepairTable struct {
	name    string
	view    string
	keys    string
	columns string
	merged  string
}

var rollupRepairTables = func() []rollupRepairTable {
	pointKeys := "project_id, name, recorded_at"
	pointColumns := "min_val, max_val, sum_val, count_val"
	pointMerged := "minMergeState(min_val), maxMergeState(max_val), sumMergeState(sum_val), countMergeState(count_val)"
	histogramKeys := "project_id, name, recorded_at, " + strings.Join(shared.HistogramDimensionColumnOrder, ", ")
	histogramColumns := "cnt, total, min_val, max_val, zero_cnt, buckets, weight_squares"
	// weight_squares as the rollup views write it, so a row from before the column existed keeps its weight.
	histogramMerged := "sum(cnt), sum(total), min(min_val), max(max_val), sum(zero_cnt), sumMap(buckets.1, buckets.2), sum(if(weight_squares > 0, weight_squares, cnt * 1000))"
	return []rollupRepairTable{
		{"metric_points_1h", "metric_points_1h_mv", pointKeys, pointColumns, pointMerged},
		{"metric_points_1d", "metric_points_1d_mv", pointKeys, pointColumns, pointMerged},
		{"metric_histograms_1h", "metric_histograms_1h_mv", histogramKeys, histogramColumns, histogramMerged},
		{"metric_histograms_1d", "metric_histograms_1d_mv", histogramKeys, histogramColumns, histogramMerged},
	}
}()

type rollupRepairWork struct {
	table     rollupRepairTable
	partition string
	// Unsorted rows found by the scan; 0 for a partition an interrupted run left staged.
	rows uint64
}

type rollupRepairPass struct {
	guard  func(step string) error
	report shared.MetricRollupRepairReport
	// What is left, per table, for the pending gauges.
	partitions map[string]int
	rows       map[string]uint64
}

// RepairMetricRollups runs one pass. guard is asked before every step that changes
// data and stops the pass by returning an error: the caller uses it to give up once it
// no longer holds the lock that keeps a second instance out.
func RepairMetricRollups(ctx context.Context, guard func(step string) error) (shared.MetricRollupRepairReport, error) {
	ctx = clickhouse.Context(ctx, clickhouse.WithSettings(clickhouse.Settings{
		"max_execution_time":                 0,
		"max_bytes_before_external_group_by": 512 << 20,
	}))
	started := time.Now()
	pass := &rollupRepairPass{guard: guard, partitions: map[string]int{}, rows: map[string]uint64{}}

	work, err := pass.plan(ctx)
	if err == nil {
		var rows uint64
		for _, item := range work {
			rows += item.rows
		}
		rollupRepairLog(ctx, "", "pass_started", "partitions", len(work), "rows", rows)
		for index, item := range work {
			if err = pass.repair(ctx, item, index+1, len(work)); err != nil {
				err = fmt.Errorf("%s partition %s: %w", item.table.name, item.partition, err)
				break
			}
		}
	}

	report := pass.report
	summary := []any{"partitions_done", report.Partitions, "partitions_skipped", len(report.Skipped), "rows_in", report.RowsBefore, "rows_out", report.RowsAfter, "took_ms", time.Since(started).Milliseconds()}
	traceway.CaptureMetric("traceway.rollup_repair.pass_ms", float64(time.Since(started).Milliseconds()))
	if err != nil {
		traceway.CaptureCount("traceway.rollup_repair.failures", 1, nil)
		rollupRepairLog(ctx, "WARNING: ", "pass_failed", append(summary, "error", err.Error())...)
		return report, err
	}
	rollupRepairLog(ctx, "", "pass_finished", summary...)
	return report, nil
}

// rollupRepairLog writes one line as `metric rollup repair: <event> key=value ...`,
// with the same pairs as rollup_repair.* attributes of the exported record.
func rollupRepairLog(ctx context.Context, severity, event string, pairs ...any) {
	fields := map[string]string{"rollup_repair.event": event}
	var line strings.Builder
	line.WriteString(severity + "metric rollup repair: " + event)
	for i := 0; i+1 < len(pairs); i += 2 {
		key, value := fmt.Sprint(pairs[i]), fmt.Sprint(pairs[i+1])
		fields["rollup_repair."+key] = value
		if strings.ContainsAny(value, " \t\n\"") {
			value = strconv.Quote(value)
		}
		line.WriteString(" " + key + "=" + value)
	}
	config.LogEvent(ctx, fields, "%s", line.String())
}

func (p *rollupRepairPass) pending(table string) {
	tags := map[string]string{"table": table}
	traceway.CaptureMetricWithTags("traceway.rollup_repair.partitions_pending", float64(p.partitions[table]), tags)
	traceway.CaptureMetricWithTags("traceway.rollup_repair.rows_pending", float64(p.rows[table]), tags)
}

func (p *rollupRepairPass) settled(item rollupRepairWork) {
	p.partitions[item.table.name]--
	p.rows[item.table.name] -= min(item.rows, p.rows[item.table.name])
	p.pending(item.table.name)
}

func (p *rollupRepairPass) plan(ctx context.Context) ([]rollupRepairWork, error) {
	var work []rollupRepairWork
	for _, table := range rollupRepairTables {
		if err := table.requireSortedView(ctx); err != nil {
			return nil, err
		}
		started := time.Now()
		partitions, err := table.partitions(ctx)
		if err != nil {
			return nil, fmt.Errorf("%s: listing partitions: %w", table.name, err)
		}
		ids := make([]string, 0, len(partitions))
		for id := range partitions {
			ids = append(ids, id)
		}
		sort.Strings(ids)
		var rows uint64
		for _, id := range ids {
			work = append(work, rollupRepairWork{table: table, partition: id, rows: partitions[id]})
			rows += partitions[id]
		}
		p.partitions[table.name], p.rows[table.name] = len(ids), rows
		p.pending(table.name)
		rollupRepairLog(ctx, "", "table_scanned", "table", table.name, "partitions", len(ids), "rows", rows, "took_ms", time.Since(started).Milliseconds())
	}
	return work, nil
}

func (t rollupRepairTable) requireSortedView(ctx context.Context) error {
	var sorted uint64
	err := chdb.Conn.QueryRow(ctx, "SELECT count() FROM system.tables WHERE database = currentDatabase() AND name = ? AND position(as_select, 'mapSort(') > 0", t.view).Scan(&sorted)
	if err != nil {
		return fmt.Errorf("%s: reading the view: %w", t.view, err)
	}
	if sorted == 0 {
		return fmt.Errorf("%s does not sort tags, so rows of %s cannot be merged safely", t.view, t.name)
	}
	return nil
}

// partitions returns the partitions with work in them and their unsorted rows.
func (t rollupRepairTable) partitions(ctx context.Context) (map[string]uint64, error) {
	found := map[string]uint64{}
	collect := func(query string, args ...any) error {
		rows, err := chdb.Conn.Query(ctx, query, args...)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var partition string
			var count uint64
			if err := rows.Scan(&partition, &count); err != nil {
				return err
			}
			found[partition] += count
		}
		return rows.Err()
	}
	staged := t.name + rollupRepairStaged
	if err := collect("SELECT substring(name, ?), toUInt64(0) FROM system.tables WHERE database = currentDatabase() AND startsWith(name, ?)", len(staged)+1, staged); err != nil {
		return nil, err
	}
	if err := collect("SELECT _partition_id, count() FROM " + t.name + " WHERE " + rollupRepairUnsorted + " GROUP BY _partition_id"); err != nil {
		return nil, err
	}
	return found, nil
}

func (p *rollupRepairPass) repair(ctx context.Context, item rollupRepairWork, index, total int) error {
	t, partition := item.table, item.partition
	if !rollupRepairPartition.MatchString(partition) {
		return fmt.Errorf("unexpected partition id %q", partition)
	}
	started := time.Now()
	tags := map[string]string{"table": t.name}
	position := fmt.Sprintf("%d/%d", index, total)
	staged := t.name + rollupRepairStaged + partition
	var exists uint64
	if err := chdb.Conn.QueryRow(ctx, "SELECT count() FROM system.tables WHERE database = currentDatabase() AND name = ?", staged).Scan(&exists); err != nil {
		return err
	}

	var before, after uint64
	if exists == 0 {
		reason, err := t.lacksSpace(ctx, partition)
		if err != nil {
			return err
		}
		if reason != "" {
			p.report.Skipped = append(p.report.Skipped, t.name+" partition "+partition+": "+reason)
			traceway.CaptureCount("traceway.rollup_repair.partitions_skipped", 1, tags)
			rollupRepairLog(ctx, "WARNING: ", "partition_skipped", "table", t.name, "partition", partition, "position", position, "reason", reason)
			p.settled(item)
			return nil
		}
		if before, after, err = p.stage(ctx, t, partition, staged); err != nil {
			return err
		}
		if before == 0 {
			p.settled(item)
			return nil
		}
		p.report.RowsBefore += before
		p.report.RowsAfter += after
	} else {
		rollupRepairLog(ctx, "", "partition_resumed", "table", t.name, "partition", partition, "position", position)
	}
	stagedAt := time.Now()

	if err := p.guard("move"); err != nil {
		return err
	}
	if err := chdb.Conn.Exec(ctx, "ALTER TABLE "+staged+" MOVE PARTITION ID '"+partition+"' TO TABLE "+t.name); err != nil {
		return fmt.Errorf("moving the merged rows in: %w", err)
	}
	movedAt := time.Now()
	if err := p.guard("delete"); err != nil {
		return err
	}
	if err := t.deleteUnsorted(ctx, partition); err != nil {
		return fmt.Errorf("deleting the unsorted rows: %w", err)
	}
	deletedAt := time.Now()
	if err := p.guard("drop"); err != nil {
		return err
	}
	if err := chdb.Conn.Exec(ctx, "DROP TABLE IF EXISTS "+staged+" SYNC"); err != nil {
		return err
	}

	p.report.Partitions++
	traceway.CaptureCount("traceway.rollup_repair.partitions_done", 1, tags)
	traceway.CaptureCount("traceway.rollup_repair.rows_merged", int64(before), tags)
	traceway.CaptureCount("traceway.rollup_repair.rows_written", int64(after), tags)
	for step, took := range map[string]time.Duration{"stage": stagedAt.Sub(started), "move": movedAt.Sub(stagedAt), "delete": deletedAt.Sub(movedAt), "total": time.Since(started)} {
		traceway.CaptureMetricMax("traceway.rollup_repair.partition_ms", float64(took.Milliseconds()), map[string]string{"table": t.name, "step": step})
	}
	p.settled(item)
	rollupRepairLog(ctx, "", "partition_done", "table", t.name, "partition", partition, "position", position, "rows_in", before, "rows_out", after,
		"stage_ms", stagedAt.Sub(started).Milliseconds(), "move_ms", movedAt.Sub(stagedAt).Milliseconds(), "delete_ms", deletedAt.Sub(movedAt).Milliseconds(), "took_ms", time.Since(started).Milliseconds())
	return nil
}

func (t rollupRepairTable) lacksSpace(ctx context.Context, partition string) (string, error) {
	rows, err := chdb.Conn.Query(ctx,
		"SELECT parts.disk_name, max(parts.bytes_on_disk), any(disks.unreserved_space) FROM system.parts AS parts "+
			"INNER JOIN system.disks AS disks ON disks.name = parts.disk_name "+
			"WHERE parts.database = currentDatabase() AND parts.table = ? AND parts.partition_id = ? AND parts.active GROUP BY parts.disk_name",
		t.name, partition)
	if err != nil {
		return "", err
	}
	defer rows.Close()
	for rows.Next() {
		var disk string
		var largest, free uint64
		if err := rows.Scan(&disk, &largest, &free); err != nil {
			return "", err
		}
		if needed := largest * rollupRepairSpaceFactor; free < needed {
			return fmt.Sprintf("disk %s has %d MB free, rewriting its parts needs about %d MB", disk, free>>20, needed>>20), nil
		}
	}
	return "", rows.Err()
}

func (p *rollupRepairPass) stage(ctx context.Context, t rollupRepairTable, partition, staged string) (before, after uint64, err error) {
	build := t.name + rollupRepairBuild
	if err := chdb.Conn.Exec(ctx, "DROP TABLE IF EXISTS "+build+" SYNC"); err != nil {
		return 0, 0, err
	}
	rows, err := chdb.Conn.Query(ctx, "SELECT toString(toDate(recorded_at)) AS day, count() FROM "+t.name+
		" WHERE _partition_id = ? AND "+rollupRepairUnsorted+" GROUP BY day ORDER BY day", partition)
	if err != nil {
		return 0, 0, err
	}
	type batch struct {
		day  string
		rows uint64
	}
	var batches []batch
	for rows.Next() {
		var b batch
		if err := rows.Scan(&b.day, &b.rows); err != nil {
			rows.Close()
			return 0, 0, err
		}
		batches = append(batches, b)
		before += b.rows
	}
	rows.Close()
	if err := rows.Err(); err != nil || before == 0 {
		return 0, 0, err
	}

	if err := chdb.Conn.Exec(ctx, "CREATE TABLE "+build+" AS "+t.name); err != nil {
		return 0, 0, err
	}
	// A day at a time bounds the GROUP BY; a rollup bucket never spans two days.
	insert := "INSERT INTO " + build + " (" + t.keys + ", tags, " + t.columns + ") SELECT " + t.keys + ", mapSort(tags) AS sorted_tags, " + t.merged +
		" FROM " + t.name + " WHERE _partition_id = ? AND toString(toDate(recorded_at)) = ? AND " + rollupRepairUnsorted + " GROUP BY " + t.keys + ", sorted_tags"
	tags := map[string]string{"table": t.name}
	for index, b := range batches {
		if err := p.guard("stage"); err != nil {
			return 0, 0, err
		}
		started := time.Now()
		if err := chdb.Conn.Exec(ctx, insert, partition, b.day); err != nil {
			return 0, 0, fmt.Errorf("merging %s: %w", b.day, err)
		}
		var written uint64
		if err := chdb.Conn.QueryRow(ctx, "SELECT count() FROM "+build).Scan(&written); err != nil {
			return 0, 0, err
		}
		took := time.Since(started).Milliseconds()
		traceway.CaptureCount("traceway.rollup_repair.batches", 1, tags)
		traceway.CaptureMetricMax("traceway.rollup_repair.batch_ms", float64(took), tags)
		p.pending(t.name)
		rollupRepairLog(ctx, "", "batch_merged", "table", t.name, "partition", partition, "batch", b.day, "position", fmt.Sprintf("%d/%d", index+1, len(batches)),
			"rows_in", b.rows, "rows_out", written-after, "took_ms", took)
		after = written
	}
	if err := p.guard("publish"); err != nil {
		return 0, 0, err
	}
	if err := chdb.Conn.Exec(ctx, "RENAME TABLE "+build+" TO "+staged); err != nil {
		return 0, 0, err
	}
	return before, after, nil
}

// deleteUnsorted returns once the partition holds no unsorted row. The delete runs as
// a background mutation that is polled, not waited on in one query: a large partition
// takes longer than the connection's read timeout. The count of rows left is what
// decides, since a mutation can be invisible for a moment on a replicated table, and
// dropping the staging table while unsorted rows remain would merge them a second time.
func (t rollupRepairTable) deleteUnsorted(ctx context.Context, partition string) error {
	command := "DELETE IN PARTITION ID '" + partition + "' WHERE " + rollupRepairUnsorted
	tags := map[string]string{"table": t.name}
	kill := func() {
		// A mutation that cannot finish would hold up every later one on the table.
		_ = chdb.Conn.Exec(ctx, "KILL MUTATION WHERE database = currentDatabase() AND table = ? AND position(command, ?) > 0", t.name, command)
	}
	started := time.Now()
	issued, failedPolls := 0, 0
	poll := 50 * time.Millisecond
	lastParts, lastProgress, lastLog := uint64(0), time.Now(), time.Now()
	for {
		var running, parts uint64
		var reason string
		err := chdb.Conn.QueryRow(ctx, "SELECT countIf(NOT is_done), toUInt64(sumIf(parts_to_do, NOT is_done)), anyIf(latest_fail_reason, NOT is_done AND latest_fail_reason != '') FROM system.mutations "+
			"WHERE database = currentDatabase() AND table = ? AND position(command, ?) > 0", t.name, command).Scan(&running, &parts, &reason)
		if err != nil {
			return err
		}
		traceway.CaptureMetricWithTags("traceway.rollup_repair.delete_parts_to_do", float64(parts), tags)
		if reason != "" {
			if failedPolls++; failedPolls > rollupRepairFailedPolls {
				kill()
				return fmt.Errorf("the mutation keeps failing: %s", reason)
			}
		} else {
			failedPolls = 0
		}
		if running == 0 {
			var left uint64
			if err := chdb.Conn.QueryRow(ctx, "SELECT count() FROM "+t.name+" WHERE _partition_id = ? AND "+rollupRepairUnsorted, partition).Scan(&left); err != nil {
				return err
			}
			if left == 0 {
				return nil
			}
			if issued++; issued > rollupRepairDeleteRetries {
				return fmt.Errorf("%d unsorted rows are left after %d deletes: something still writes them", left, rollupRepairDeleteRetries)
			}
			if err := chdb.Conn.Exec(ctx, "ALTER TABLE "+t.name+" "+command+" SETTINGS mutations_sync = 0"); err != nil {
				return err
			}
			lastParts, lastProgress = 0, time.Now()
		} else {
			if parts != lastParts {
				lastParts, lastProgress = parts, time.Now()
			}
			if time.Since(lastProgress) > rollupRepairStall {
				kill()
				return fmt.Errorf("the mutation made no progress for %s with %d parts to do: check the free disk space", rollupRepairStall, parts)
			}
			if time.Since(lastLog) >= rollupRepairProgressEvery {
				lastLog = time.Now()
				rollupRepairLog(ctx, "", "delete_running", "table", t.name, "partition", partition, "parts_to_do", parts, "elapsed_ms", time.Since(started).Milliseconds())
			}
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(poll):
		}
		poll = min(poll*2, rollupRepairMaxPoll)
	}
}
