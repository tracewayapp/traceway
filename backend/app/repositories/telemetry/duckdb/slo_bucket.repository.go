//go:build telemetry_duckdb

package duckdb

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/tracewayapp/traceway/backend/app/db"
	"github.com/tracewayapp/traceway/backend/app/repositories/telemetry/shared"
)

type sloBucketRepository struct{}

const sloBucketUpsert = "INSERT INTO slo_buckets (project_id, slo_id, bucket_start, good, total, computed_at) VALUES (?, ?, ?, ?, ?, ?) " +
	"ON CONFLICT (project_id, slo_id, bucket_start) DO UPDATE SET good = excluded.good, total = excluded.total, computed_at = excluded.computed_at"

func (r *sloBucketRepository) Upsert(ctx context.Context, buckets []shared.SLOBucket) error {
	if len(buckets) == 0 {
		return nil
	}
	tx, err := db.TelemetryDB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for _, b := range buckets {
		if _, err := tx.ExecContext(ctx, sloBucketUpsert, b.ProjectId.String(), int64(b.SLOId), b.BucketStart.UTC(), b.Good, b.Total, b.ComputedAt.UTC()); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (r *sloBucketRepository) SumRange(ctx context.Context, projectId uuid.UUID, sloId int, from, to time.Time) (int64, int64, error) {
	var good, total int64
	err := db.TelemetryDB.QueryRowContext(ctx,
		"SELECT COALESCE(CAST(SUM(good) AS BIGINT), 0), COALESCE(CAST(SUM(total) AS BIGINT), 0) FROM slo_buckets WHERE project_id = ? AND slo_id = ? AND bucket_start >= ? AND bucket_start < ?",
		projectId.String(), int64(sloId), from.UTC(), to.UTC()).Scan(&good, &total)
	return good, total, err
}

func (r *sloBucketRepository) SnapshotSums(ctx context.Context, projectId uuid.UUID, sloId int, from, to, sixHoursFrom, hourFrom time.Time) (shared.SLOSnapshotSums, error) {
	var sums shared.SLOSnapshotSums
	six, hour := sixHoursFrom.UTC(), hourFrom.UTC()
	err := db.TelemetryDB.QueryRowContext(ctx,
		"SELECT COALESCE(CAST(SUM(good) AS BIGINT), 0), COALESCE(CAST(SUM(total) AS BIGINT), 0), COALESCE(CAST(SUM(CASE WHEN bucket_start >= ? THEN good ELSE 0 END) AS BIGINT), 0), COALESCE(CAST(SUM(CASE WHEN bucket_start >= ? THEN total ELSE 0 END) AS BIGINT), 0), "+
			"COALESCE(CAST(SUM(CASE WHEN bucket_start >= ? THEN good ELSE 0 END) AS BIGINT), 0), COALESCE(CAST(SUM(CASE WHEN bucket_start >= ? THEN total ELSE 0 END) AS BIGINT), 0) "+
			"FROM slo_buckets WHERE project_id = ? AND slo_id = ? AND bucket_start >= ? AND bucket_start < ?",
		six, six, hour, hour, projectId.String(), int64(sloId), from.UTC(), to.UTC()).
		Scan(&sums.Window.Good, &sums.Window.Total, &sums.LastSixHours.Good, &sums.LastSixHours.Total, &sums.LastHour.Good, &sums.LastHour.Total)
	return sums, err
}

func (r *sloBucketRepository) Series(ctx context.Context, projectId uuid.UUID, sloId int, from, to time.Time, intervalMinutes int) ([]shared.SLOBucketPoint, error) {
	query := "SELECT " + epochBucket("bucket_start", intervalMinutes*60) + " AS bucket, CAST(SUM(total) AS BIGINT), CAST(SUM(good) AS BIGINT) FROM slo_buckets " +
		"WHERE project_id = ? AND slo_id = ? AND bucket_start >= ? AND bucket_start < ? GROUP BY bucket ORDER BY bucket ASC"
	rows, err := db.TelemetryDB.QueryContext(ctx, query, projectId.String(), int64(sloId), from.UTC(), to.UTC())
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return shared.ScanSLOBucketPoints(rows, from, to)
}

func (r *sloBucketRepository) DeleteBySLO(ctx context.Context, projectId uuid.UUID, sloId int) error {
	_, err := db.TelemetryDB.ExecContext(ctx, "DELETE FROM slo_buckets WHERE project_id = ? AND slo_id = ?", projectId.String(), int64(sloId))
	return err
}

func (r *sloBucketRepository) PruneBefore(ctx context.Context, cutoff time.Time) error {
	_, err := db.TelemetryDB.ExecContext(ctx, "DELETE FROM slo_buckets WHERE bucket_start < ?", cutoff.UTC())
	return err
}

var SLOBucketRepository = &sloBucketRepository{}
