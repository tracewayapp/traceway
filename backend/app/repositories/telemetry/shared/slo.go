package shared

import (
	"time"

	"github.com/google/uuid"
)

type SLOBucket struct {
	ProjectId   uuid.UUID
	SLOId       int
	BucketStart time.Time
	Good        int64
	Total       int64
	ComputedAt  time.Time
}

type SLOCount struct {
	Good  int64
	Total int64
}

type SLOSnapshotSums struct {
	Window       SLOCount
	LastSixHours SLOCount
	LastHour     SLOCount
}

type SLOBucketPoint struct {
	Bucket time.Time `json:"bucket"`
	Good   int64     `json:"good"`
	Total  int64     `json:"total"`
}

type EndpointSLISpec struct {
	Endpoint         string
	Latency          bool
	LatencyThreshold time.Duration
}

type TaskSLISpec struct {
	TaskName string
}

type SpanSLISpec struct {
	Filter           OtelSpanSearch
	Latency          bool
	LatencyThreshold time.Duration
}

type SessionSLISpec struct {
	AppVersion      string
	IncludeMessages bool
}

const SessionExceptionSlack = 2 * time.Hour

const TaskExceptionSlack = 6 * time.Hour

type bucketRows interface {
	Next() bool
	Scan(dest ...any) error
	Err() error
}

func ScanSLOBucketPoints(rows bucketRows, from, to time.Time) ([]SLOBucketPoint, error) {
	points := make([]SLOBucketPoint, 0)
	for rows.Next() {
		var bucket, total, good int64
		if err := rows.Scan(&bucket, &total, &good); err != nil {
			return nil, err
		}
		at := time.Unix(bucket, 0).UTC()
		if !at.Before(to) {
			continue
		}
		if at.Before(from) {
			at = from
		}
		points = append(points, SLOBucketPoint{Bucket: at, Good: good, Total: total})
	}
	return points, rows.Err()
}

func SpanGoodCondition(spec SpanSLISpec) (string, []any) {
	if spec.Latency {
		return "duration <= ?", []any{int64(spec.LatencyThreshold)}
	}
	return "status_code != 2", nil
}

func OtelSLICountQuery(search OtelSpanSearch, good string, goodArgs []any, bucketSeconds int, dialect OtelSearchDialect) (string, []any) {
	predicate, args := otelSearchPredicate(search, dialect)
	winners := OtelWinningRows("recorded_at, duration, status_code", predicate, dialect.WinnerOrder)
	query := "SELECT " + dialect.Bucket("recorded_at", bucketSeconds) + " AS bucket, " + dialect.Count + ", " + dialect.CountIf(good) +
		" FROM " + winners + " GROUP BY bucket ORDER BY bucket ASC" + dialect.ReadSetting
	return query, append(append([]any{}, goodArgs...), args...)
}
