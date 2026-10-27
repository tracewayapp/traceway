//go:build telemetry_ch

package clickhouse

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/tracewayapp/traceway/backend/app/chdb"
	"github.com/tracewayapp/traceway/backend/app/repositories/telemetry/shared"
)

func epochBucket(column string, seconds int) string {
	return fmt.Sprintf("toInt64(toUnixTimestamp(toStartOfInterval(%s, INTERVAL %d second)))", column, seconds)
}

func countIf(condition string) string {
	return "toInt64(countIf(" + condition + "))"
}

func distinctIf(condition string) string {
	return "toInt64(uniqExactIf(id, " + condition + "))"
}

func (e *endpointRepository) SLICounts(ctx context.Context, projectId uuid.UUID, spec shared.EndpointSLISpec, from, to time.Time, bucketMinutes int) ([]shared.SLOBucketPoint, error) {
	good := "status_code < 500"
	var args []any
	where := ""
	if spec.Latency {
		good = "duration <= ?"
		args = append(args, int64(spec.LatencyThreshold))
		where += " AND is_stream = 0"
	}
	args = append(args, projectId, from.UTC(), to.UTC())
	if spec.Endpoint != "" {
		where += " AND endpoint = ?"
		args = append(args, spec.Endpoint)
	}
	query := "SELECT " + epochBucket("recorded_at", bucketMinutes*60) + " AS bucket, toInt64(uniqExact(id)), " + distinctIf(good) +
		" FROM endpoints_v2 WHERE project_id = ? AND recorded_at >= ? AND recorded_at <= ?" + where + " GROUP BY bucket ORDER BY bucket ASC"
	rows, err := chdb.Conn.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return shared.ScanSLOBucketPoints(rows, from, to)
}

func (r *taskRepository) SLICounts(ctx context.Context, projectId uuid.UUID, spec shared.TaskSLISpec, from, to time.Time, bucketMinutes int) ([]shared.SLOBucketPoint, error) {
	failed := "(trace_id, span_id) IN (SELECT trace_id, span_id FROM exceptions_v2 WHERE project_id = ? AND trace_type = 'task' AND recorded_at >= ? AND recorded_at <= ?)"
	args := []any{projectId, from.UTC(), to.Add(shared.TaskExceptionSlack).UTC(), projectId, from.UTC(), to.UTC()}
	where := ""
	if spec.TaskName != "" {
		where = " AND task_name = ?"
		args = append(args, spec.TaskName)
	}
	query := "SELECT " + epochBucket("recorded_at", bucketMinutes*60) + " AS bucket, toInt64(uniqExact(id)), " + distinctIf("NOT ("+failed+")") +
		" FROM tasks_v2 WHERE project_id = ? AND recorded_at >= ? AND recorded_at <= ?" + where + " GROUP BY bucket ORDER BY bucket ASC"
	rows, err := chdb.Conn.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return shared.ScanSLOBucketPoints(rows, from, to)
}

func (r *otelSpanRepository) SLICounts(ctx context.Context, spec shared.SpanSLISpec, from, to time.Time, bucketMinutes int) ([]shared.SLOBucketPoint, error) {
	search := spec.Filter
	search.From, search.To = from, to
	good, goodArgs := shared.SpanGoodCondition(spec)
	query, args := shared.OtelSLICountQuery(search, good, goodArgs, bucketMinutes*60, clickhouseSearchDialect)
	rows, err := chdb.Conn.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return shared.ScanSLOBucketPoints(rows, from, to)
}
