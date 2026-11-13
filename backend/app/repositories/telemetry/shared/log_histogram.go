package shared

import "time"

const (
	LogLevelUnknown = iota
	LogLevelTrace
	LogLevelDebug
	LogLevelInfo
	LogLevelWarn
	LogLevelError
	LogLevelFatal
)

// LogLevelSQL groups a log record into one of the LogLevel* codes the same way
// the dashboard's severity badge labels it: by the severity text when that names
// a level, otherwise by the OpenTelemetry severity number range. The expression is
// valid on SQLite, DuckDB and ClickHouse.
const LogLevelSQL = `CASE
		WHEN UPPER(severity_text) IN ('FATAL', 'CRITICAL') THEN 6
		WHEN UPPER(severity_text) = 'ERROR' THEN 5
		WHEN UPPER(severity_text) IN ('WARN', 'WARNING') THEN 4
		WHEN UPPER(severity_text) = 'INFO' THEN 3
		WHEN UPPER(severity_text) = 'DEBUG' THEN 2
		WHEN UPPER(severity_text) = 'TRACE' THEN 1
		WHEN severity_number >= 21 THEN 6
		WHEN severity_number >= 17 THEN 5
		WHEN severity_number >= 13 THEN 4
		WHEN severity_number >= 9 THEN 3
		WHEN severity_number >= 5 THEN 2
		WHEN severity_number >= 1 THEN 1
		ELSE 0
	END`

const (
	LogHistogramDefaultBuckets = 80
	LogHistogramMinBuckets     = 10
	LogHistogramMaxBuckets     = 200
)

var logHistogramSteps = []int64{
	1, 2, 5, 10, 15, 30,
	60, 120, 300, 600, 900, 1800,
	3600, 7200, 10800, 21600, 43200,
	86400, 172800, 604800,
}

// LogHistogramRow is one (bucket, level) count read from storage. Bucket is the
// bucket's start in Unix seconds, floored to the step from the epoch so every
// backend draws the same boundaries.
type LogHistogramRow struct {
	Bucket int64
	Level  int
	Count  int64
}

type LogHistogramBucket struct {
	Start   time.Time `json:"start"`
	Fatal   int64     `json:"fatal"`
	Error   int64     `json:"error"`
	Warn    int64     `json:"warn"`
	Info    int64     `json:"info"`
	Debug   int64     `json:"debug"`
	Trace   int64     `json:"trace"`
	Unknown int64     `json:"unknown"`
}

func (b *LogHistogramBucket) add(level int, count int64) {
	switch level {
	case LogLevelFatal:
		b.Fatal += count
	case LogLevelError:
		b.Error += count
	case LogLevelWarn:
		b.Warn += count
	case LogLevelInfo:
		b.Info += count
	case LogLevelDebug:
		b.Debug += count
	case LogLevelTrace:
		b.Trace += count
	default:
		b.Unknown += count
	}
}

func (b LogHistogramBucket) Total() int64 {
	return b.Fatal + b.Error + b.Warn + b.Info + b.Debug + b.Trace + b.Unknown
}

func floorDiv(a, b int64) int64 {
	q := a / b
	if a%b != 0 && (a < 0) != (b < 0) {
		q--
	}
	return q
}

func logHistogramBucketCount(from, to time.Time, step int64) int64 {
	return floorDiv(to.Unix(), step) - floorDiv(from.Unix(), step) + 1
}

// LogHistogramStep returns the smallest round step, in seconds, that splits
// [from, to] into at most maxBuckets epoch-aligned buckets. Past the largest
// round step it falls back to whole days.
func LogHistogramStep(from, to time.Time, maxBuckets int) int64 {
	if maxBuckets < 1 {
		maxBuckets = 1
	}
	for _, step := range logHistogramSteps {
		if logHistogramBucketCount(from, to, step) <= int64(maxBuckets) {
			return step
		}
	}
	const day = int64(86400)
	span := int64(to.Sub(from).Seconds())
	days := span/day/int64(maxBuckets) + 1
	for logHistogramBucketCount(from, to, days*day) > int64(maxBuckets) {
		days++
	}
	return days * day
}

// BuildLogHistogram lays the sparse rows onto one bucket per step from the bucket
// holding from to the bucket holding to, so empty stretches come back as zeros.
func BuildLogHistogram(from, to time.Time, step int64, rows []LogHistogramRow) []LogHistogramBucket {
	if step <= 0 || to.Before(from) {
		return []LogHistogramBucket{}
	}
	first := floorDiv(from.Unix(), step) * step
	n := logHistogramBucketCount(from, to, step)
	buckets := make([]LogHistogramBucket, n)
	for i := range buckets {
		buckets[i].Start = time.Unix(first+int64(i)*step, 0).UTC()
	}
	for _, row := range rows {
		if row.Bucket < first {
			continue
		}
		i := (row.Bucket - first) / step
		if i >= n {
			continue
		}
		buckets[i].add(row.Level, row.Count)
	}
	return buckets
}
