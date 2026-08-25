// Direct-Firebolt telemetry benchmark.
//
// Evaluates Firebolt Core (self-managed engine, HTTP API on :3473) as a
// candidate Traceway telemetry backend, before any backend integration
// exists. Mirrors the questions the hardware bench answers for the real
// backends, but speaks SQL-over-HTTP directly:
//
//   - ingest ramp: multi-row INSERT throughput per batch size
//   - read probe: dashboard-shaped reads (grouped endpoints, metric
//     time-series, log page) at increasing table sizes
//
// Tables mirror backend/app/migrations/duckdb_telemetry with Firebolt types.
// Read queries mirror the duckdb repositories' dashboard queries, translated
// to the Firebolt dialect (percentile_cont WITHIN GROUP, epoch-floor time
// buckets, JSON_POINTER_EXTRACT_TEXT).
package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"os"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

const projectId = "9f4a1f6e-0000-4000-8000-b0b0b0b0b0b0"

var (
	target        = flag.String("target", "http://localhost:3473", "engine HTTP endpoint")
	dialect       = flag.String("dialect", "firebolt", "firebolt|clickhouse")
	chUser        = flag.String("ch-user", "default", "clickhouse user (dialect=clickhouse)")
	chPassword    = flag.String("ch-password", "default", "clickhouse password (dialect=clickhouse)")
	chDatabase    = flag.String("ch-database", "bench", "clickhouse database, created if missing; the bench truncates/drops tables inside it (dialect=clickhouse)")
	signal        = flag.String("signal", "spans", "spans|metrics|logs")
	workers       = flag.Int("workers", 4, "concurrent insert workers")
	batchSizes    = flag.String("batch-sizes", "256,1024,4096,8192,16384", "ingest ramp batch sizes")
	stepSeconds   = flag.Int("step-seconds", 20, "seconds per ingest ramp step")
	fillLevels    = flag.String("fill-levels", "100000,1000000,10000000", "row counts to probe reads at")
	probeRuns     = flag.Int("probe-runs", 5, "repetitions per read query")
	reportOut     = flag.String("report-out", "firebolt-bench.json", "output JSON path")
	reset         = flag.Bool("reset", true, "drop + recreate tables before the run")
	skipRamp      = flag.Bool("skip-ramp", false, "skip the ingest ramp, go straight to fill+probe")
	fillBatchSize = flag.Int("fill-batch-size", 8192, "batch size used for fill ingest")
	underWrite    = flag.Bool("probe-under-write", false, "also run each probe while sustained ingest continues in the background")
	cacheBust     = flag.Bool("cache-bust", false, "shift the probe time window per run so every run misses the engine's result cache (dashboard-realistic: the time window moves every request)")
	fbTuned       = flag.Bool("fb-tuned", false, "firebolt only: design around the engine — aggregating indexes, VACUUM after fill, bucket-aligned probe queries")
)

// ---------- HTTP client ----------

var httpClient = &http.Client{Timeout: 120 * time.Second}

type fbStatistics struct {
	Elapsed   float64 `json:"elapsed"`
	RowsRead  int64   `json:"rows_read"`
	BytesRead int64   `json:"bytes_read"`
}

type fbResponse struct {
	Data       []map[string]any `json:"data"`
	Rows       int64            `json:"rows"`
	Errors     []struct {
		Description string `json:"description"`
	} `json:"errors"`
	Statistics fbStatistics `json:"statistics"`
}

// queryURL and header auth are resolved once in main for the chosen dialect.
var (
	queryURL   string
	authHeader map[string]string
)

// exec sends one SQL statement and returns the parsed JSON response.
// Firebolt and ClickHouse return the same JSON shape (meta/data/rows/statistics).
func exec(sql string) (*fbResponse, time.Duration, error) {
	start := time.Now()
	req, err := http.NewRequest("POST", queryURL, strings.NewReader(sql))
	if err != nil {
		return nil, 0, err
	}
	req.Header.Set("Content-Type", "text/plain")
	for k, v := range authHeader {
		req.Header.Set(k, v)
	}
	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, time.Since(start), err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	elapsed := time.Since(start)
	if err != nil {
		return nil, elapsed, err
	}
	var out fbResponse
	if len(bytes.TrimSpace(body)) > 0 {
		if err := json.Unmarshal(body, &out); err != nil {
			return nil, elapsed, fmt.Errorf("bad response (http %d): %s", resp.StatusCode, truncate(string(body), 400))
		}
	}
	if len(out.Errors) > 0 {
		return &out, elapsed, fmt.Errorf("firebolt error: %s", out.Errors[0].Description)
	}
	if resp.StatusCode != 200 {
		return &out, elapsed, fmt.Errorf("http %d: %s", resp.StatusCode, truncate(string(body), 400))
	}
	return &out, elapsed, nil
}

// waitHealthy polls /ping until the engine answers — the dev build has died
// under VACUUM and heavy concurrent load, and docker restarts it within
// seconds; probing through the restart window would record connection
// errors as query results.
func waitHealthy(maxWait time.Duration) bool {
	deadline := time.Now().Add(maxWait)
	for time.Now().Before(deadline) {
		// /health/ready, not /ping: ping answers Ok. before the node can
		// actually serve queries ("Cluster not yet healthy" follows it).
		resp, err := httpClient.Get(*target + "/health/ready")
		if err == nil {
			io.Copy(io.Discard, resp.Body)
			resp.Body.Close()
			if resp.StatusCode == 200 {
				return true
			}
		}
		time.Sleep(3 * time.Second)
	}
	return false
}

func isConnectionError(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "connection reset") || strings.Contains(msg, "EOF") ||
		strings.Contains(msg, "connection refused") || strings.Contains(msg, "Client.Timeout")
}

func truncate(s string, n int) string {
	if len(s) > n {
		return s[:n] + "..."
	}
	return s
}

// ---------- schema ----------

var ddl = map[string][]string{
	"spans": {
		`CREATE TABLE IF NOT EXISTS endpoints (
			id TEXT NOT NULL,
			project_id TEXT NOT NULL,
			endpoint TEXT NOT NULL,
			duration BIGINT NOT NULL,
			recorded_at TIMESTAMP NOT NULL,
			status_code BIGINT NOT NULL,
			body_size BIGINT NOT NULL,
			client_ip TEXT NOT NULL,
			attributes TEXT NOT NULL,
			app_version TEXT NOT NULL,
			server_name TEXT NOT NULL,
			distributed_trace_id TEXT,
			span_id TEXT,
			is_stream BIGINT NOT NULL,
			is_root BIGINT NOT NULL
		) PRIMARY INDEX project_id, recorded_at`,
		`CREATE TABLE IF NOT EXISTS slow_endpoints (
			project_id TEXT NOT NULL,
			endpoint TEXT NOT NULL,
			offset_ms BIGINT NOT NULL,
			reason TEXT NOT NULL
		)`,
	},
	"metrics": {
		`CREATE TABLE IF NOT EXISTS metric_points (
			project_id TEXT NOT NULL,
			name TEXT NOT NULL,
			value DOUBLE PRECISION NOT NULL,
			tags TEXT NOT NULL,
			recorded_at TIMESTAMP NOT NULL,
			server_name TEXT NOT NULL
		) PRIMARY INDEX project_id, name, recorded_at`,
	},
	"logs": {
		`CREATE TABLE IF NOT EXISTS log_records (
			id TEXT NOT NULL,
			project_id TEXT NOT NULL,
			timestamp TIMESTAMP NOT NULL,
			trace_id TEXT NOT NULL,
			span_id TEXT NOT NULL,
			trace_flags BIGINT NOT NULL,
			severity_text TEXT NOT NULL,
			severity_number BIGINT NOT NULL,
			service_name TEXT NOT NULL,
			body TEXT NOT NULL,
			resource_schema_url TEXT NOT NULL,
			resource_attributes TEXT NOT NULL,
			scope_schema_url TEXT NOT NULL,
			scope_name TEXT NOT NULL,
			scope_version TEXT NOT NULL,
			scope_attributes TEXT NOT NULL,
			log_attributes TEXT NOT NULL
		) PRIMARY INDEX project_id, timestamp`,
	},
}

// Firebolt-native aggregating indexes for the tuned mode. Maintained at
// insert time; queries whose grouping keys are a subset of the index keys
// are rewritten to merge the stored aggregate states (incl. exact
// percentile_cont states). Percentiles must NOT use FILTER — that blocks the
// rewrite — so the tuned grouped query filters is_stream via WHERE on the
// is_stream index key instead.
var ddlTuned = map[string][]string{
	"spans": {
		`CREATE AGGREGATING INDEX IF NOT EXISTS endpoints_dash_idx ON endpoints (
			project_id, DATE_TRUNC('hour', recorded_at), endpoint, is_stream,
			COUNT(*), AVG(duration), MAX(recorded_at),
			SUM(CASE WHEN status_code >= 500 THEN 1 ELSE 0 END),
			SUM(CASE WHEN status_code >= 400 AND status_code < 500 THEN 1 ELSE 0 END),
			SUM(CASE WHEN duration <= 750000000 AND status_code < 500 THEN 1 ELSE 0 END),
			SUM(CASE WHEN duration > 750000000 AND duration <= 1500000000 AND status_code < 500 THEN 1 ELSE 0 END),
			SUM(CASE WHEN duration > 1500000000 OR status_code >= 500 THEN 1 ELSE 0 END),
			MAX(is_stream), MAX(is_root), MAX(CASE WHEN is_root = 0 THEN 1 ELSE 0 END),
			PERCENTILE_CONT(0.50) WITHIN GROUP (ORDER BY duration),
			PERCENTILE_CONT(0.95) WITHIN GROUP (ORDER BY duration),
			PERCENTILE_CONT(0.99) WITHIN GROUP (ORDER BY duration)
		)`,
	},
	"metrics": {
		`CREATE AGGREGATING INDEX IF NOT EXISTS metric_points_dash_idx ON metric_points (
			project_id, name, DATE_TRUNC('minute', recorded_at), JSON_POINTER_EXTRACT_TEXT(tags, '/host'),
			COUNT(*), AVG(value), MAX(recorded_at)
		)`,
	},
	"logs": {
		`CREATE AGGREGATING INDEX IF NOT EXISTS log_records_count_idx ON log_records (
			project_id, DATE_TRUNC('hour', timestamp), severity_text,
			COUNT(*)
		)`,
	},
}

// ClickHouse twins of the same logical schema — MergeTree with the ORDER BY
// matching each signal's read pattern, mirroring the ch migrations' style.
var ddlCH = map[string][]string{
	"spans": {
		`CREATE TABLE IF NOT EXISTS endpoints (
			id String, project_id String, endpoint String, duration Int64,
			recorded_at DateTime64(3), status_code Int64, body_size Int64,
			client_ip String, attributes String, app_version String, server_name String,
			distributed_trace_id String, span_id String, is_stream UInt8, is_root UInt8
		) ENGINE = MergeTree ORDER BY (project_id, recorded_at)`,
		`CREATE TABLE IF NOT EXISTS slow_endpoints (
			project_id String, endpoint String, offset_ms Int64, reason String
		) ENGINE = MergeTree ORDER BY (project_id, endpoint)`,
	},
	"metrics": {
		`CREATE TABLE IF NOT EXISTS metric_points (
			project_id String, name String, value Float64, tags String,
			recorded_at DateTime64(3), server_name String
		) ENGINE = MergeTree ORDER BY (project_id, name, recorded_at)`,
	},
	"logs": {
		`CREATE TABLE IF NOT EXISTS log_records (
			id String, project_id String, timestamp DateTime64(3), trace_id String,
			span_id String, trace_flags Int64, severity_text String, severity_number Int64,
			service_name String, body String, resource_schema_url String,
			resource_attributes String, scope_schema_url String, scope_name String,
			scope_version String, scope_attributes String, log_attributes String
		) ENGINE = MergeTree ORDER BY (project_id, timestamp)`,
	},
}

var fbIndexNames = map[string][]string{
	"spans":   {"endpoints_dash_idx"},
	"metrics": {"metric_points_dash_idx"},
	"logs":    {"log_records_count_idx"},
}

var dropTables = map[string][]string{
	"spans":   {"endpoints", "slow_endpoints"},
	"metrics": {"metric_points"},
	"logs":    {"log_records"},
}

var mainTable = map[string]string{
	"spans":   "endpoints",
	"metrics": "metric_points",
	"logs":    "log_records",
}

// ---------- data generation (mirrors loadgen's data variety) ----------

var endpointPaths = []string{
	"GET /api/users", "GET /api/users/:id", "POST /api/users", "GET /api/orders",
	"POST /api/orders", "GET /api/orders/:id", "GET /api/products", "GET /api/products/:id",
	"POST /api/checkout", "GET /api/cart", "POST /api/cart/items", "GET /api/search",
	"GET /api/health",
}

var statusCodes = []int{200, 200, 200, 200, 200, 200, 201, 404, 500}

var metricNames = []string{
	"bench.metric.cpu", "bench.metric.mem", "bench.metric.qps", "bench.metric.lat",
	"bench.metric.errs", "bench.metric.disk", "bench.metric.net", "bench.metric.heap",
	"bench.metric.gc", "bench.metric.fd",
}

var severities = []struct {
	text string
	num  int
}{
	{"INFO", 9}, {"INFO", 9}, {"INFO", 9}, {"INFO", 9}, {"WARN", 13}, {"ERROR", 17},
}

const bodyChars = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789 "

func randBody(rng *rand.Rand, n int) string {
	b := make([]byte, n)
	for i := range b {
		b[i] = bodyChars[rng.Intn(len(bodyChars))]
	}
	return string(b)
}

func randTs(rng *rand.Rand, base time.Time) string {
	// spread over the trailing 24h so time-range reads scan realistically
	return base.Add(-time.Duration(rng.Int63n(int64(24 * time.Hour)))).UTC().Format("2006-01-02 15:04:05.000")
}

func fakeHex(rng *rand.Rand, n int) string {
	const hexc = "0123456789abcdef"
	b := make([]byte, n)
	for i := range b {
		b[i] = hexc[rng.Intn(16)]
	}
	return string(b)
}

// appendRow writes one VALUES tuple for the signal.
func appendRow(sb *strings.Builder, sig string, rng *rand.Rand, base time.Time, seq int64) {
	switch sig {
	case "spans":
		dur := (10 + rng.Int63n(990)) * 1_000_000 // 10–1000ms in ns
		fmt.Fprintf(sb, "('%s','%s','%s',%d,'%s',%d,%d,'10.0.0.%d','{\"bench\":\"1\"}','1.0.0','bench-host-%d','%s','%s',0,1)",
			fakeHex(rng, 32), projectId, endpointPaths[rng.Intn(len(endpointPaths))], dur, randTs(rng, base),
			statusCodes[rng.Intn(len(statusCodes))], rng.Int63n(65536), rng.Intn(255)+1, rng.Intn(4)+1,
			fakeHex(rng, 32), fakeHex(rng, 16))
	case "metrics":
		fmt.Fprintf(sb, "('%s','%s',%s,'{\"host\":\"bench-host-%d\"}','%s','bench-host-%d')",
			projectId, metricNames[rng.Intn(len(metricNames))],
			strconv.FormatFloat(rng.Float64()*100, 'f', 4, 64), rng.Intn(4)+1, randTs(rng, base), rng.Intn(4)+1)
	case "logs":
		sev := severities[rng.Intn(len(severities))]
		fmt.Fprintf(sb, "('%s','%s','%s','%s','%s',0,'%s',%d,'bench-service','%s','','{\"service_name\":\"bench-service\"}','','bench-scope','1.0.0','{}','{\"trace_id\":\"%s\",\"k1\":\"v1\",\"k2\":\"v2\"}')",
			fakeHex(rng, 32), projectId, randTs(rng, base), fakeHex(rng, 32), fakeHex(rng, 16),
			sev.text, sev.num, randBody(rng, 120), fakeHex(rng, 32))
	}
}

var insertPrefix = map[string]string{
	"spans":   "INSERT INTO endpoints VALUES ",
	"metrics": "INSERT INTO metric_points VALUES ",
	"logs":    "INSERT INTO log_records VALUES ",
}

func buildInsert(sig string, rng *rand.Rand, base time.Time, batch int) string {
	var sb strings.Builder
	sb.Grow(batch * 260)
	sb.WriteString(insertPrefix[sig])
	for i := 0; i < batch; i++ {
		if i > 0 {
			sb.WriteByte(',')
		}
		appendRow(&sb, sig, rng, base, int64(i))
	}
	return sb.String()
}

// ---------- read queries ----------

var windowShift atomic.Int64

func timeWindow() (string, string) {
	now := time.Now().UTC()
	if *cacheBust {
		// A unique millisecond offset per query build changes the SQL text
		// and the (empty) tail of the scanned range without materially
		// changing the work, so result caches cannot answer repeat runs.
		now = now.Add(time.Duration(windowShift.Add(1)) * time.Millisecond)
	}
	return now.Add(-25 * time.Hour).Format("2006-01-02 15:04:05.000"), now.Format("2006-01-02 15:04:05.000")
}

// tunedWindow returns hour-aligned bounds so DATE_TRUNC-keyed aggregating
// indexes can serve the filter. Cache-busting shifts the upper bound into
// empty future buckets: the SQL text changes, the work scanned does not.
func tunedWindow() (string, string) {
	now := time.Now().UTC().Truncate(time.Hour)
	to := now.Add(time.Hour)
	if *cacheBust {
		to = to.Add(time.Duration(windowShift.Add(1)) * time.Hour)
	}
	return now.Add(-25 * time.Hour).Format("2006-01-02 15:04:05"), to.Format("2006-01-02 15:04:05")
}

func probeQueriesFBTuned(sig string) []struct{ Name, SQL string } {
	from, to := tunedWindow()
	switch sig {
	case "spans":
		grouped := fmt.Sprintf(`SELECT c.endpoint, c.total_count, c.avg_duration, c.last_seen, 0 as offset_ms,
			c.server_error_count, c.client_error_count, c.satisfied_count, c.tolerating_count, c.bad_count,
			c.is_stream, c.has_root, c.has_non_root, p.p50, p.p95, p.p99
		FROM (
			SELECT endpoint,
				COUNT(*) as total_count,
				AVG(duration) as avg_duration,
				MAX(recorded_at) as last_seen,
				SUM(CASE WHEN status_code >= 500 THEN 1 ELSE 0 END) as server_error_count,
				SUM(CASE WHEN status_code >= 400 AND status_code < 500 THEN 1 ELSE 0 END) as client_error_count,
				SUM(CASE WHEN duration <= 750000000 AND status_code < 500 THEN 1 ELSE 0 END) as satisfied_count,
				SUM(CASE WHEN duration > 750000000 AND duration <= 1500000000 AND status_code < 500 THEN 1 ELSE 0 END) as tolerating_count,
				SUM(CASE WHEN duration > 1500000000 OR status_code >= 500 THEN 1 ELSE 0 END) as bad_count,
				MAX(is_stream) as is_stream,
				MAX(is_root) as has_root,
				MAX(CASE WHEN is_root = 0 THEN 1 ELSE 0 END) as has_non_root
			FROM endpoints
			WHERE project_id = '%[1]s' AND DATE_TRUNC('hour', recorded_at) >= '%[2]s' AND DATE_TRUNC('hour', recorded_at) <= '%[3]s'
			GROUP BY endpoint
		) c LEFT JOIN (
			SELECT endpoint,
				PERCENTILE_CONT(0.50) WITHIN GROUP (ORDER BY duration) as p50,
				PERCENTILE_CONT(0.95) WITHIN GROUP (ORDER BY duration) as p95,
				PERCENTILE_CONT(0.99) WITHIN GROUP (ORDER BY duration) as p99
			FROM endpoints
			WHERE project_id = '%[1]s' AND is_stream = 0 AND DATE_TRUNC('hour', recorded_at) >= '%[2]s' AND DATE_TRUNC('hour', recorded_at) <= '%[3]s'
			GROUP BY endpoint
		) p ON c.endpoint = p.endpoint`, projectId, from, to)
		chart := fmt.Sprintf(`SELECT DATE_TRUNC('hour', recorded_at) AS bucket,
			COUNT(*) AS cnt, PERCENTILE_CONT(0.95) WITHIN GROUP (ORDER BY duration) AS p95
		FROM endpoints WHERE project_id = '%s' AND DATE_TRUNC('hour', recorded_at) >= '%s' AND DATE_TRUNC('hour', recorded_at) <= '%s'
		GROUP BY bucket ORDER BY bucket ASC`, projectId, from, to)
		return []struct{ Name, SQL string }{
			{"grouped-endpoints", grouped},
			{"endpoint-latency-chart", chart},
		}
	case "metrics":
		series := fmt.Sprintf(`SELECT DATE_TRUNC('minute', recorded_at) AS bucket, AVG(value) AS agg_value
		FROM metric_points
		WHERE project_id = '%s' AND name = 'bench.metric.cpu' AND DATE_TRUNC('minute', recorded_at) >= '%s' AND DATE_TRUNC('minute', recorded_at) <= '%s'
		GROUP BY bucket ORDER BY bucket ASC`, projectId, from, to)
		grouped := fmt.Sprintf(`SELECT DATE_TRUNC('minute', recorded_at) AS bucket,
			JSON_POINTER_EXTRACT_TEXT(tags, '/host') AS group_key, AVG(value) AS agg_value
		FROM metric_points
		WHERE project_id = '%s' AND name = 'bench.metric.cpu' AND DATE_TRUNC('minute', recorded_at) >= '%s' AND DATE_TRUNC('minute', recorded_at) <= '%s'
		GROUP BY bucket, group_key ORDER BY bucket ASC`, projectId, from, to)
		discover := fmt.Sprintf(`SELECT name, COUNT(*) AS cnt, MAX(recorded_at) AS last_seen
		FROM metric_points WHERE project_id = '%s' AND DATE_TRUNC('minute', recorded_at) >= '%s' AND DATE_TRUNC('minute', recorded_at) <= '%s'
		GROUP BY name ORDER BY name`, projectId, from, to)
		return []struct{ Name, SQL string }{
			{"timeseries-avg-1m", series},
			{"timeseries-grouped-by-tag", grouped},
			{"discover-names", discover},
		}
	case "logs":
		// Page + search stay raw scans (no aggregation to index); the count
		// is index-served. VACUUM after fill is what helps the scans.
		base := probeQueriesFBTunedLogsRaw()
		count := fmt.Sprintf(`SELECT SUM(c) AS count FROM (
			SELECT COUNT(*) AS c FROM log_records
			WHERE project_id = '%s' AND DATE_TRUNC('hour', timestamp) >= '%s' AND DATE_TRUNC('hour', timestamp) <= '%s'
			GROUP BY severity_text
		)`, projectId, from, to)
		return append(base[:1], append([]struct{ Name, SQL string }{{"log-count", count}}, base[1:]...)...)
	}
	return nil
}

func probeQueriesFBTunedLogsRaw() []struct{ Name, SQL string } {
	from, to := timeWindow()
	page := fmt.Sprintf(`SELECT id, project_id, timestamp, trace_id, span_id, trace_flags,
		severity_text, severity_number, service_name, body,
		resource_attributes, scope_name, log_attributes
	FROM log_records
	WHERE project_id = '%s' AND timestamp >= '%s' AND timestamp <= '%s'
	ORDER BY timestamp DESC, id LIMIT 50`, projectId, from, to)
	filtered := fmt.Sprintf(`SELECT id, timestamp, severity_text, body FROM log_records
	WHERE project_id = '%s' AND timestamp >= '%s' AND timestamp <= '%s'
	AND severity_text = 'ERROR' AND body LIKE '%%a1%%'
	ORDER BY timestamp DESC, id LIMIT 50`, projectId, from, to)
	return []struct{ Name, SQL string }{
		{"log-page", page},
		{"log-severity-search", filtered},
	}
}

func probeQueries(sig string) []struct{ Name, SQL string } {
	if *dialect == "clickhouse" {
		return probeQueriesCH(sig)
	}
	if *fbTuned {
		return probeQueriesFBTuned(sig)
	}
	from, to := timeWindow()
	switch sig {
	case "spans":
		grouped := fmt.Sprintf(`SELECT
			e.endpoint,
			COUNT(*) as total_count,
			AVG(e.duration) as avg_duration,
			MAX(e.recorded_at) as last_seen,
			COALESCE(s.offset_ms, 0) as offset_ms,
			CAST(SUM(CASE WHEN e.status_code >= 500 THEN 1 ELSE 0 END) AS BIGINT) as server_error_count,
			CAST(SUM(CASE WHEN e.status_code >= 400 AND e.status_code < 500 THEN 1 ELSE 0 END) AS BIGINT) as client_error_count,
			CAST(SUM(CASE WHEN e.duration <= (750000000 + COALESCE(s.offset_ms, 0) * 1000000) AND e.status_code < 500 THEN 1 ELSE 0 END) AS BIGINT) as satisfied_count,
			CAST(SUM(CASE WHEN e.duration > (750000000 + COALESCE(s.offset_ms, 0) * 1000000) AND e.duration <= (1500000000 + COALESCE(s.offset_ms, 0) * 1000000) AND e.status_code < 500 THEN 1 ELSE 0 END) AS BIGINT) as tolerating_count,
			CAST(SUM(CASE WHEN e.duration > (1500000000 + COALESCE(s.offset_ms, 0) * 1000000) OR e.status_code >= 500 THEN 1 ELSE 0 END) AS BIGINT) as bad_count,
			MAX(e.is_stream) as is_stream,
			MAX(e.is_root) as has_root,
			MAX(CASE WHEN e.is_root = 0 THEN 1 ELSE 0 END) as has_non_root,
			percentile_cont(0.50) WITHIN GROUP (ORDER BY e.duration) FILTER (WHERE e.is_stream = 0) AS p50,
			percentile_cont(0.95) WITHIN GROUP (ORDER BY e.duration) FILTER (WHERE e.is_stream = 0) AS p95,
			percentile_cont(0.99) WITHIN GROUP (ORDER BY e.duration) FILTER (WHERE e.is_stream = 0) AS p99
		FROM endpoints e
		LEFT JOIN slow_endpoints s ON e.endpoint = s.endpoint AND e.project_id = s.project_id
		WHERE e.project_id = '%s' AND e.recorded_at >= '%s' AND e.recorded_at <= '%s'
		GROUP BY e.endpoint, s.offset_ms`, projectId, from, to)
		chart := fmt.Sprintf(`SELECT TO_TIMESTAMP(FLOOR(EXTRACT(EPOCH FROM recorded_at) / 300) * 300) AS bucket,
			COUNT(*) AS cnt, percentile_cont(0.95) WITHIN GROUP (ORDER BY duration) AS p95
		FROM endpoints WHERE project_id = '%s' AND recorded_at >= '%s' AND recorded_at <= '%s'
		GROUP BY bucket ORDER BY bucket ASC`, projectId, from, to)
		return []struct{ Name, SQL string }{
			{"grouped-endpoints", grouped},
			{"endpoint-latency-chart", chart},
		}
	case "metrics":
		series := fmt.Sprintf(`SELECT TO_TIMESTAMP(FLOOR(EXTRACT(EPOCH FROM recorded_at) / 60) * 60) AS bucket, AVG(value) AS agg_value
		FROM metric_points
		WHERE project_id = '%s' AND name = 'bench.metric.cpu' AND recorded_at >= '%s' AND recorded_at <= '%s'
		GROUP BY bucket ORDER BY bucket ASC`, projectId, from, to)
		grouped := fmt.Sprintf(`SELECT TO_TIMESTAMP(FLOOR(EXTRACT(EPOCH FROM recorded_at) / 60) * 60) AS bucket,
			JSON_POINTER_EXTRACT_TEXT(tags, '/host') AS group_key, AVG(value) AS agg_value
		FROM metric_points
		WHERE project_id = '%s' AND name = 'bench.metric.cpu' AND recorded_at >= '%s' AND recorded_at <= '%s'
		GROUP BY bucket, group_key ORDER BY bucket ASC`, projectId, from, to)
		discover := fmt.Sprintf(`SELECT name, COUNT(*) AS cnt, MAX(recorded_at) AS last_seen
		FROM metric_points WHERE project_id = '%s' AND recorded_at >= '%s' AND recorded_at <= '%s'
		GROUP BY name ORDER BY name`, projectId, from, to)
		return []struct{ Name, SQL string }{
			{"timeseries-avg-1m", series},
			{"timeseries-grouped-by-tag", grouped},
			{"discover-names", discover},
		}
	case "logs":
		page := fmt.Sprintf(`SELECT id, project_id, timestamp, trace_id, span_id, trace_flags,
			severity_text, severity_number, service_name, body,
			resource_attributes, scope_name, log_attributes
		FROM log_records
		WHERE project_id = '%s' AND timestamp >= '%s' AND timestamp <= '%s'
		ORDER BY timestamp DESC, id LIMIT 50`, projectId, from, to)
		count := fmt.Sprintf(`SELECT COUNT(*) AS count FROM log_records
		WHERE project_id = '%s' AND timestamp >= '%s' AND timestamp <= '%s'`, projectId, from, to)
		filtered := fmt.Sprintf(`SELECT id, timestamp, severity_text, body FROM log_records
		WHERE project_id = '%s' AND timestamp >= '%s' AND timestamp <= '%s'
		AND severity_text = 'ERROR' AND body LIKE '%%a1%%'
		ORDER BY timestamp DESC, id LIMIT 50`, projectId, from, to)
		return []struct{ Name, SQL string }{
			{"log-page", page},
			{"log-count", count},
			{"log-severity-search", filtered},
		}
	}
	return nil
}

// probeQueriesCH mirrors probeQueries in the ClickHouse dialect, matching how
// the clickhouse repositories actually write these reads (countIf/quantileIf,
// toStartOfInterval, JSONExtractString).
func probeQueriesCH(sig string) []struct{ Name, SQL string } {
	from, to := timeWindow()
	switch sig {
	case "spans":
		grouped := fmt.Sprintf(`SELECT
			e.endpoint,
			count() as total_count,
			avg(e.duration) as avg_duration,
			max(e.recorded_at) as last_seen,
			s.offset_ms as offset_ms,
			countIf(e.status_code >= 500) as server_error_count,
			countIf(e.status_code >= 400 AND e.status_code < 500) as client_error_count,
			countIf(e.duration <= (750000000 + s.offset_ms * 1000000) AND e.status_code < 500) as satisfied_count,
			countIf(e.duration > (750000000 + s.offset_ms * 1000000) AND e.duration <= (1500000000 + s.offset_ms * 1000000) AND e.status_code < 500) as tolerating_count,
			countIf(e.duration > (1500000000 + s.offset_ms * 1000000) OR e.status_code >= 500) as bad_count,
			max(e.is_stream) as is_stream,
			max(e.is_root) as has_root,
			max(e.is_root = 0) as has_non_root,
			quantileIf(0.50)(e.duration, e.is_stream = 0) AS p50,
			quantileIf(0.95)(e.duration, e.is_stream = 0) AS p95,
			quantileIf(0.99)(e.duration, e.is_stream = 0) AS p99
		FROM endpoints e
		LEFT JOIN slow_endpoints s ON e.endpoint = s.endpoint AND e.project_id = s.project_id
		WHERE e.project_id = '%s' AND e.recorded_at >= '%s' AND e.recorded_at <= '%s'
		GROUP BY e.endpoint, s.offset_ms`, projectId, from, to)
		chart := fmt.Sprintf(`SELECT toStartOfInterval(recorded_at, INTERVAL 300 SECOND) AS bucket,
			count() AS cnt, quantile(0.95)(duration) AS p95
		FROM endpoints WHERE project_id = '%s' AND recorded_at >= '%s' AND recorded_at <= '%s'
		GROUP BY bucket ORDER BY bucket ASC`, projectId, from, to)
		return []struct{ Name, SQL string }{
			{"grouped-endpoints", grouped},
			{"endpoint-latency-chart", chart},
		}
	case "metrics":
		series := fmt.Sprintf(`SELECT toStartOfInterval(recorded_at, INTERVAL 60 SECOND) AS bucket, avg(value) AS agg_value
		FROM metric_points
		WHERE project_id = '%s' AND name = 'bench.metric.cpu' AND recorded_at >= '%s' AND recorded_at <= '%s'
		GROUP BY bucket ORDER BY bucket ASC`, projectId, from, to)
		grouped := fmt.Sprintf(`SELECT toStartOfInterval(recorded_at, INTERVAL 60 SECOND) AS bucket,
			JSONExtractString(tags, 'host') AS group_key, avg(value) AS agg_value
		FROM metric_points
		WHERE project_id = '%s' AND name = 'bench.metric.cpu' AND recorded_at >= '%s' AND recorded_at <= '%s'
		GROUP BY bucket, group_key ORDER BY bucket ASC`, projectId, from, to)
		discover := fmt.Sprintf(`SELECT name, count() AS cnt, max(recorded_at) AS last_seen
		FROM metric_points WHERE project_id = '%s' AND recorded_at >= '%s' AND recorded_at <= '%s'
		GROUP BY name ORDER BY name`, projectId, from, to)
		return []struct{ Name, SQL string }{
			{"timeseries-avg-1m", series},
			{"timeseries-grouped-by-tag", grouped},
			{"discover-names", discover},
		}
	case "logs":
		page := fmt.Sprintf(`SELECT id, project_id, timestamp, trace_id, span_id, trace_flags,
			severity_text, severity_number, service_name, body,
			resource_attributes, scope_name, log_attributes
		FROM log_records
		WHERE project_id = '%s' AND timestamp >= '%s' AND timestamp <= '%s'
		ORDER BY timestamp DESC, id LIMIT 50`, projectId, from, to)
		count := fmt.Sprintf(`SELECT count() AS count FROM log_records
		WHERE project_id = '%s' AND timestamp >= '%s' AND timestamp <= '%s'`, projectId, from, to)
		filtered := fmt.Sprintf(`SELECT id, timestamp, severity_text, body FROM log_records
		WHERE project_id = '%s' AND timestamp >= '%s' AND timestamp <= '%s'
		AND severity_text = 'ERROR' AND body LIKE '%%a1%%'
		ORDER BY timestamp DESC, id LIMIT 50`, projectId, from, to)
		return []struct{ Name, SQL string }{
			{"log-page", page},
			{"log-count", count},
			{"log-severity-search", filtered},
		}
	}
	return nil
}

// ---------- result shapes ----------

type rampStep struct {
	BatchSize        int     `json:"batchSize"`
	Seconds          float64 `json:"seconds"`
	Requests         int64   `json:"requests"`
	Errors           int64   `json:"errors"`
	RowsAcked        int64   `json:"rowsAcked"`
	RowsPerSec       float64 `json:"rowsPerSec"`
	P50Ms            float64 `json:"p50Ms"`
	P95Ms            float64 `json:"p95Ms"`
	P99Ms            float64 `json:"p99Ms"`
	FirstError       string  `json:"firstError,omitempty"`
	AvgPayloadBytes  int64   `json:"avgPayloadBytes"`
}

type queryResult struct {
	Name            string    `json:"name"`
	RunsMs          []float64 `json:"runsMs"`
	MinMs           float64   `json:"minMs"`
	MedianMs        float64   `json:"medianMs"`
	ServerElapsedMs []float64 `json:"serverElapsedMs"`
	RowsReturned    int64     `json:"rowsReturned"`
	Error           string    `json:"error,omitempty"`
}

type fillLevel struct {
	Target                      int64         `json:"target"`
	TableRows                   int64         `json:"tableRows"`
	FillSeconds                 float64       `json:"fillSeconds"`
	FillRowsSec                 float64       `json:"fillRowsPerSec"`
	WriteRowsPerSecDuringProbes float64       `json:"writeRowsPerSecDuringProbes,omitempty"`
	VacuumSeconds               float64       `json:"vacuumSeconds,omitempty"`
	Queries                     []queryResult `json:"queries"`
}

// runProbes executes every probe query for the signal probeRuns times.
func runProbes(sig string, tableRows int64, tag string) []queryResult {
	var out []queryResult
	for qi, q := range probeQueries(sig) {
		qr := queryResult{Name: q.Name}
		for i := 0; i < *probeRuns; i++ {
			sql := q.SQL
			if *cacheBust && i > 0 {
				sql = probeQueries(sig)[qi].SQL // fresh time window per run
			}
			resp, elapsed, err := exec(sql)
			if err != nil && isConnectionError(err) {
				// Engine died mid-probe (it restarts in seconds); wait and
				// retry once so a crash-window doesn't erase the level.
				fmt.Fprintf(os.Stderr, "probe %s: engine unreachable (%v), waiting for restart...\n", qr.Name, err)
				if waitHealthy(3 * time.Minute) {
					resp, elapsed, err = exec(sql)
				}
			}
			if err != nil {
				qr.Error = err.Error()
				break
			}
			qr.RunsMs = append(qr.RunsMs, float64(elapsed)/float64(time.Millisecond))
			qr.ServerElapsedMs = append(qr.ServerElapsedMs, resp.Statistics.Elapsed*1000)
			qr.RowsReturned = resp.Rows
		}
		if len(qr.RunsMs) > 0 {
			sorted := append([]float64(nil), qr.RunsMs...)
			sort.Float64s(sorted)
			qr.MinMs = sorted[0]
			qr.MedianMs = sorted[len(sorted)/2]
		}
		out = append(out, qr)
		if qr.Error != "" {
			fmt.Fprintf(os.Stderr, "probe @%d %s%s: ERROR %s\n", tableRows, qr.Name, tag, qr.Error)
		} else {
			fmt.Fprintf(os.Stderr, "probe @%d rows %s%s: cold=%.1fms warm-min=%.1fms median=%.1fms rows=%d\n",
				tableRows, qr.Name, tag, qr.RunsMs[0], qr.MinMs, qr.MedianMs, qr.RowsReturned)
		}
	}
	return out
}

type report struct {
	Target        string      `json:"target"`
	Dialect       string      `json:"dialect"`
	EngineVersion string      `json:"engineVersion"`
	Scenario      string      `json:"scenario"`
	Signal        string      `json:"signal"`
	Workers       int         `json:"workers"`
	StartedAt     time.Time   `json:"startedAt"`
	EndedAt       time.Time   `json:"endedAt"`
	IngestRamp    []rampStep  `json:"ingestRamp,omitempty"`
	MaxRowsPerSec float64     `json:"maxRowsPerSec"`
	FillLevels    []fillLevel `json:"fillLevels"`
}

func percentileMs(durs []time.Duration, p float64) float64 {
	if len(durs) == 0 {
		return 0
	}
	sorted := append([]time.Duration(nil), durs...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i] < sorted[j] })
	idx := int(float64(len(sorted)-1) * p)
	return float64(sorted[idx]) / float64(time.Millisecond)
}

// ---------- ingest ----------

// runIngest inserts with N workers until the duration elapses, the row target
// is reached, or stop is closed (whichever applies). Returns stats.
func runIngest(sig string, batch int, dur time.Duration, rowTarget int64, startRows int64, stop <-chan struct{}) rampStep {
	var (
		mu        sync.Mutex
		latencies []time.Duration
		requests  int64
		errors    int64
		rowsAcked int64
		firstErr  atomic.Value
		payload   int64
	)
	base := time.Now()
	deadline := base.Add(dur)
	var wg sync.WaitGroup
	for w := 0; w < *workers; w++ {
		wg.Add(1)
		go func(seed int64) {
			defer wg.Done()
			rng := rand.New(rand.NewSource(seed))
			for {
				if stop != nil {
					select {
					case <-stop:
						return
					default:
					}
				}
				if dur > 0 && time.Now().After(deadline) {
					return
				}
				if rowTarget > 0 && startRows+atomic.LoadInt64(&rowsAcked) >= rowTarget {
					return
				}
				sql := buildInsert(sig, rng, time.Now(), batch)
				_, elapsed, err := exec(sql)
				atomic.AddInt64(&requests, 1)
				atomic.AddInt64(&payload, int64(len(sql)))
				if err != nil {
					atomic.AddInt64(&errors, 1)
					firstErr.CompareAndSwap(nil, err.Error())
					time.Sleep(200 * time.Millisecond)
					continue
				}
				atomic.AddInt64(&rowsAcked, int64(batch))
				mu.Lock()
				latencies = append(latencies, elapsed)
				mu.Unlock()
			}
		}(time.Now().UnixNano() + int64(w))
	}
	wg.Wait()
	elapsed := time.Since(base).Seconds()
	step := rampStep{
		BatchSize:  batch,
		Seconds:    elapsed,
		Requests:   requests,
		Errors:     errors,
		RowsAcked:  rowsAcked,
		RowsPerSec: float64(rowsAcked) / elapsed,
		P50Ms:      percentileMs(latencies, 0.50),
		P95Ms:      percentileMs(latencies, 0.95),
		P99Ms:      percentileMs(latencies, 0.99),
	}
	if requests > 0 {
		step.AvgPayloadBytes = payload / requests
	}
	if v := firstErr.Load(); v != nil {
		step.FirstError = v.(string)
	}
	return step
}

func tableCount(sig string) (int64, error) {
	resp, _, err := exec(fmt.Sprintf("SELECT COUNT(*) AS c FROM %s", mainTable[sig]))
	if err != nil {
		return 0, err
	}
	if len(resp.Data) == 0 {
		return 0, fmt.Errorf("no count row")
	}
	switch v := resp.Data[0]["c"].(type) {
	case float64:
		return int64(v), nil
	case string:
		return strconv.ParseInt(v, 10, 64)
	}
	return 0, fmt.Errorf("unexpected count type %T", resp.Data[0]["c"])
}

func parseInts(csv string) []int64 {
	var out []int64
	for _, part := range strings.Split(csv, ",") {
		n, err := strconv.ParseInt(strings.TrimSpace(part), 10, 64)
		if err != nil {
			fmt.Fprintf(os.Stderr, "bad number %q: %v\n", part, err)
			os.Exit(1)
		}
		out = append(out, n)
	}
	return out
}

func main() {
	flag.Parse()
	sig := *signal
	if _, ok := ddl[sig]; !ok {
		fmt.Fprintf(os.Stderr, "unknown signal %q (spans|metrics|logs)\n", sig)
		os.Exit(1)
	}

	signalDDL := ddl[sig]
	switch *dialect {
	case "firebolt":
		queryURL = *target + "/?output_format=JSON"
		if *fbTuned {
			signalDDL = append(append([]string{}, signalDDL...), ddlTuned[sig]...)
		}
	case "clickhouse":
		authHeader = map[string]string{
			"X-ClickHouse-User": *chUser,
			"X-ClickHouse-Key":  *chPassword,
		}
		// Create the dedicated bench database before scoping every later
		// statement to it — the bench must never touch other databases.
		queryURL = *target + "/?default_format=JSON"
		if _, _, err := exec("CREATE DATABASE IF NOT EXISTS " + *chDatabase); err != nil {
			fmt.Fprintf(os.Stderr, "create database: %v\n", err)
			os.Exit(1)
		}
		queryURL = *target + "/?default_format=JSON&database=" + *chDatabase
		signalDDL = ddlCH[sig]
	default:
		fmt.Fprintf(os.Stderr, "unknown dialect %q (firebolt|clickhouse)\n", *dialect)
		os.Exit(1)
	}

	scenario := "engine-direct"
	if *fbTuned {
		scenario = "engine-direct-tuned"
	}
	rep := report{
		Target:    *target,
		Dialect:   *dialect,
		Scenario:  scenario,
		Signal:    sig,
		Workers:   *workers,
		StartedAt: time.Now().UTC(),
	}
	if resp, _, err := exec("SELECT version() AS v"); err == nil && len(resp.Data) > 0 {
		rep.EngineVersion, _ = resp.Data[0]["v"].(string)
	}

	if *reset {
		// Firebolt refuses to drop a table that an aggregating index depends
		// on, and a prior tuned run may have left one behind — drop indexes
		// first, unconditionally.
		if *dialect == "firebolt" {
			for _, idx := range fbIndexNames[sig] {
				if _, _, err := exec("DROP AGGREGATING INDEX IF EXISTS " + idx); err != nil {
					fmt.Fprintf(os.Stderr, "drop index %s: %v\n", idx, err)
					os.Exit(1)
				}
			}
		}
		for _, t := range dropTables[sig] {
			if _, _, err := exec("DROP TABLE IF EXISTS " + t); err != nil {
				fmt.Fprintf(os.Stderr, "drop %s: %v\n", t, err)
				os.Exit(1)
			}
		}
	}
	for _, stmt := range signalDDL {
		if _, _, err := exec(stmt); err != nil {
			fmt.Fprintf(os.Stderr, "ddl: %v\n", err)
			os.Exit(1)
		}
	}

	// Phase 1: ingest ramp
	if !*skipRamp {
		for _, b := range parseInts(*batchSizes) {
			step := runIngest(sig, int(b), time.Duration(*stepSeconds)*time.Second, 0, 0, nil)
			rep.IngestRamp = append(rep.IngestRamp, step)
			if step.RowsPerSec > rep.MaxRowsPerSec && step.Errors == 0 {
				rep.MaxRowsPerSec = step.RowsPerSec
			}
			fmt.Fprintf(os.Stderr, "ramp batch=%d: %.0f rows/s reqs=%d errs=%d p50=%.0fms p95=%.0fms p99=%.0fms\n",
				step.BatchSize, step.RowsPerSec, step.Requests, step.Errors, step.P50Ms, step.P95Ms, step.P99Ms)
			if step.Errors > 0 {
				fmt.Fprintf(os.Stderr, "  first error: %s\n", step.FirstError)
			}
		}
	}

	// Phase 2: fill + read probe. Truncate first so fill levels are exact
	// rather than inflated by whatever the ramp left behind.
	if !*skipRamp {
		if _, _, err := exec("TRUNCATE TABLE " + mainTable[sig]); err != nil {
			fmt.Fprintf(os.Stderr, "truncate: %v\n", err)
			os.Exit(1)
		}
	}
	for _, level := range parseInts(*fillLevels) {
		current, err := tableCount(sig)
		if err != nil {
			fmt.Fprintf(os.Stderr, "count: %v\n", err)
			os.Exit(1)
		}
		fl := fillLevel{Target: level}
		if current < level {
			fmt.Fprintf(os.Stderr, "filling %s from %d to %d rows...\n", mainTable[sig], current, level)
			fillStart := time.Now()
			step := runIngest(sig, *fillBatchSize, 0, level, current, nil)
			fl.FillSeconds = time.Since(fillStart).Seconds()
			if fl.FillSeconds > 0 {
				fl.FillRowsSec = float64(step.RowsAcked) / fl.FillSeconds
			}
			if step.Errors > 0 {
				fmt.Fprintf(os.Stderr, "  fill errors=%d first: %s\n", step.Errors, step.FirstError)
			}
		}
		fl.TableRows, err = tableCount(sig)
		if err != nil {
			fmt.Fprintf(os.Stderr, "count: %v\n", err)
			os.Exit(1)
		}

		// Tuned mode compacts before probing — the same reason the main
		// bench has a digestion gate: fresh small insert-tablets are not
		// the steady state a dashboard reads.
		if *fbTuned {
			vStart := time.Now()
			if _, _, err := exec("VACUUM " + mainTable[sig]); err != nil {
				fmt.Fprintf(os.Stderr, "vacuum: %v\n", err)
				if isConnectionError(err) {
					fmt.Fprintf(os.Stderr, "vacuum killed the engine, waiting for restart...\n")
					waitHealthy(3 * time.Minute)
				}
			}
			fl.VacuumSeconds = time.Since(vStart).Seconds()
			fmt.Fprintf(os.Stderr, "vacuum %s: %.1fs\n", mainTable[sig], fl.VacuumSeconds)
		}

		fl.Queries = runProbes(sig, fl.TableRows, "")

		// Optionally repeat the probes while sustained ingest hammers the
		// same table — the production shape: dashboards read during ingest.
		if *underWrite {
			stop := make(chan struct{})
			done := make(chan rampStep, 1)
			go func() {
				done <- runIngest(sig, *fillBatchSize, 0, 0, 0, stop)
			}()
			time.Sleep(2 * time.Second) // let ingest reach steady state
			uw := runProbes(sig, fl.TableRows, " (under write)")
			close(stop)
			bg := <-done
			fmt.Fprintf(os.Stderr, "  background ingest during probes: %.0f rows/s errs=%d\n", bg.RowsPerSec, bg.Errors)
			for i := range uw {
				uw[i].Name += "-under-write"
			}
			fl.Queries = append(fl.Queries, uw...)
			fl.WriteRowsPerSecDuringProbes = bg.RowsPerSec
		}
		rep.FillLevels = append(rep.FillLevels, fl)
	}

	rep.EndedAt = time.Now().UTC()
	out, _ := json.MarshalIndent(rep, "", "  ")
	if err := os.WriteFile(*reportOut, out, 0o644); err != nil {
		fmt.Fprintf(os.Stderr, "write report: %v\n", err)
		os.Exit(1)
	}
	fmt.Fprintf(os.Stderr, "wrote %s: signal=%s maxRowsPerSec=%.0f\n", *reportOut, sig, rep.MaxRowsPerSec)
}
