package shared

import (
	"time"

	"github.com/google/uuid"
)

// SLOBucket is one computed slice of an SLO: how many events (or time slices)
// the SLI saw in the bucket and how many of them were good.
type SLOBucket struct {
	ProjectId   uuid.UUID
	SLOId       int
	BucketStart time.Time
	Good        int64
	Total       int64
	ComputedAt  time.Time
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

// SpanSLISpec counts spans the way the span explorer filters them. The caller
// sets Filter.ProjectId; From and To are set per query.
type SpanSLISpec struct {
	Filter           OtelSpanSearch
	Latency          bool
	LatencyThreshold time.Duration
}

// SessionSLISpec is the crash-free sessions indicator: a session is bad when an
// exception was recorded in it, captured messages only when IncludeMessages.
type SessionSLISpec struct {
	AppVersion      string
	IncludeMessages bool
}

// SessionExceptionSlack widens the exception window past the session start
// window: a session lasts up to SessionMaxSpan and its exceptions land during it.
const SessionExceptionSlack = 2 * time.Hour

// TaskExceptionSlack widens the exception window past the task window: a task
// is recorded at its start and its exception lands when it fails.
const TaskExceptionSlack = 6 * time.Hour

type bucketRows interface {
	Next() bool
	Scan(dest ...any) error
	Err() error
}

// ScanSLOBucketPoints reads (epoch bucket, total, good) rows and keeps the
// buckets inside [from, to). The source queries use an inclusive upper bound,
// so a row recorded exactly at to lands in a bucket this drops.
func ScanSLOBucketPoints(rows bucketRows, from, to time.Time) ([]SLOBucketPoint, error) {
	points := make([]SLOBucketPoint, 0)
	for rows.Next() {
		var bucket, total, good int64
		if err := rows.Scan(&bucket, &total, &good); err != nil {
			return nil, err
		}
		at := time.Unix(bucket, 0).UTC()
		if at.Before(from) || !at.Before(to) {
			continue
		}
		points = append(points, SLOBucketPoint{Bucket: at, Good: good, Total: total})
	}
	return points, rows.Err()
}

// SpanGoodCondition is the SQL predicate for a good span; latency SLIs bind the threshold.
func SpanGoodCondition(spec SpanSLISpec) (string, []any) {
	if spec.Latency {
		return "duration <= ?", []any{int64(spec.LatencyThreshold)}
	}
	return "status_code != 2", nil
}

// OtelSLICountQuery counts the spans matching the search per bucket, and how
// many of them satisfy good. Retried spans count once, like in Search.
func OtelSLICountQuery(search OtelSpanSearch, good string, goodArgs []any, bucketSeconds int, dialect OtelSearchDialect) (string, []any) {
	predicate, args := otelSearchPredicate(search, dialect)
	scope, scopeArgs := otelSearchPredicate(OtelSpanSearch{ProjectId: search.ProjectId, From: search.From, To: search.To}, dialect)
	winners := OtelWinningRows("*", scope, dialect.WinnerOrder)
	query := "SELECT " + dialect.Bucket("recorded_at", bucketSeconds) + " AS bucket, " + dialect.Count + ", " + dialect.CountIf(good) +
		" FROM " + winners + " WHERE " + predicate + " GROUP BY bucket ORDER BY bucket ASC" + dialect.ReadSetting
	all := append(append(append([]any{}, goodArgs...), scopeArgs...), args...)
	return query, all
}
