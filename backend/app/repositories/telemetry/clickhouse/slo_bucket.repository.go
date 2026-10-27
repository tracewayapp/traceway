//go:build telemetry_ch

package clickhouse

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/tracewayapp/traceway/backend/app/chdb"
	"github.com/tracewayapp/traceway/backend/app/repositories/telemetry/shared"
)

type sloBucketRepository struct{}

func (r *sloBucketRepository) Upsert(ctx context.Context, buckets []shared.SLOBucket) error {
	if len(buckets) == 0 {
		return nil
	}
	batch, err := chdb.Conn.PrepareBatch(chdb.BatchCtx(), "INSERT INTO slo_buckets (project_id, slo_id, bucket_start, good, total, computed_at)")
	if err != nil {
		return err
	}
	for _, b := range buckets {
		if err := batch.Append(b.ProjectId, int64(b.SLOId), b.BucketStart.UTC(), b.Good, b.Total, b.ComputedAt.UTC()); err != nil {
			return err
		}
	}
	return batch.Send()
}

func (r *sloBucketRepository) SumRange(ctx context.Context, projectId uuid.UUID, sloId int, from, to time.Time) (int64, int64, error) {
	var good, total int64
	err := chdb.Conn.QueryRow(ctx,
		"SELECT toInt64(sum(good)), toInt64(sum(total)) FROM slo_buckets FINAL WHERE project_id = ? AND slo_id = ? AND bucket_start >= ? AND bucket_start < ?",
		projectId, int64(sloId), from.UTC(), to.UTC()).Scan(&good, &total)
	return good, total, err
}

func (r *sloBucketRepository) SnapshotSums(ctx context.Context, projectId uuid.UUID, sloId int, from, to, sixHoursFrom, hourFrom time.Time) (shared.SLOSnapshotSums, error) {
	var sums shared.SLOSnapshotSums
	err := chdb.Conn.QueryRow(ctx,
		"SELECT toInt64(sum(good)), toInt64(sum(total)), toInt64(sumIf(good, bucket_start >= ?)), toInt64(sumIf(total, bucket_start >= ?)), toInt64(sumIf(good, bucket_start >= ?)), toInt64(sumIf(total, bucket_start >= ?)) "+
			"FROM slo_buckets FINAL WHERE project_id = ? AND slo_id = ? AND bucket_start >= ? AND bucket_start < ?",
		sixHoursFrom.UTC(), sixHoursFrom.UTC(), hourFrom.UTC(), hourFrom.UTC(), projectId, int64(sloId), from.UTC(), to.UTC()).
		Scan(&sums.Window.Good, &sums.Window.Total, &sums.LastSixHours.Good, &sums.LastSixHours.Total, &sums.LastHour.Good, &sums.LastHour.Total)
	return sums, err
}

func (r *sloBucketRepository) Series(ctx context.Context, projectId uuid.UUID, sloId int, from, to time.Time, intervalMinutes int) ([]shared.SLOBucketPoint, error) {
	query := "SELECT " + epochBucket("bucket_start", intervalMinutes*60) + " AS bucket, toInt64(sum(total)), toInt64(sum(good)) FROM slo_buckets FINAL " +
		"WHERE project_id = ? AND slo_id = ? AND bucket_start >= ? AND bucket_start < ? GROUP BY bucket ORDER BY bucket ASC"
	rows, err := chdb.Conn.Query(ctx, query, projectId, int64(sloId), from.UTC(), to.UTC())
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return shared.ScanSLOBucketPoints(rows, from, to)
}

func (r *sloBucketRepository) DeleteBySLO(ctx context.Context, projectId uuid.UUID, sloId int) error {
	return chdb.Conn.Exec(ctx,
		"INSERT INTO slo_buckets (project_id, slo_id, bucket_start, good, total, computed_at) SELECT project_id, slo_id, bucket_start, 0, 0, fromUnixTimestamp64Milli(?) FROM slo_buckets FINAL WHERE project_id = ? AND slo_id = ? AND (good != 0 OR total != 0)",
		time.Now().UnixMilli(), projectId, int64(sloId))
}

func (r *sloBucketRepository) PruneBefore(ctx context.Context, cutoff time.Time) error {
	return nil
}

var SLOBucketRepository = &sloBucketRepository{}
