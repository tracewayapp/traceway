//go:build telemetry_ch

package clickhouse

import (
	"context"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	ch "github.com/ClickHouse/clickhouse-go/v2"
	"github.com/google/uuid"
	"github.com/tracewayapp/traceway/backend/v2/app/chdb"
	"github.com/tracewayapp/traceway/backend/v2/app/repositories/telemetry/shared"
	"github.com/tracewayapp/traceway/backend/v2/app/traceway"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
)

// Uses an isolated database on a local server; no PostgreSQL or production data is needed.
func TestProjectDataQueryComparisonLive(t *testing.T) {
	addr := os.Getenv("TRACEWAY_QUERY_TEST_ADDR")
	if addr == "" {
		t.Skip("TRACEWAY_QUERY_TEST_ADDR not set")
	}
	host, _, err := net.SplitHostPort(addr)
	if err != nil || (host != "localhost" && !net.ParseIP(host).IsLoopback()) {
		t.Fatal("TRACEWAY_QUERY_TEST_ADDR must be a loopback address")
	}
	ctx := context.Background()
	database := "traceway_has_data_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	admin, err := ch.Open(&ch.Options{Addr: []string{addr}})
	if err != nil {
		t.Fatal(err)
	}
	defer admin.Close()
	if err := admin.Exec(ctx, "CREATE DATABASE "+database); err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := admin.Exec(ctx, "DROP DATABASE "+database+" SYNC"); err != nil {
			t.Error(err)
		}
	}()
	conn, err := ch.Open(&ch.Options{Addr: []string{addr}, Auth: ch.Auth{Database: database}, Settings: ch.Settings{"max_threads": 2, "use_query_cache": 0, "log_queries": 1}})
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	previous := chdb.Conn
	chdb.Conn = conn
	defer func() { chdb.Conn = previous }()
	for _, migration := range []string{"0086_create_endpoints_v2.up.sql", "0089_create_exceptions_v2.up.sql", "0045_create_log_records.up.sql"} {
		schema, err := os.ReadFile(filepath.Join("../../../migrations/ch", migration))
		if err != nil {
			t.Fatal(err)
		}
		if err := conn.Exec(ctx, string(schema)); err != nil {
			t.Fatal(err)
		}
	}
	now := time.Date(2026, 11, 13, 9, 30, 0, 0, time.UTC)
	heavy := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	other := uuid.MustParse("33333333-3333-3333-3333-333333333333")
	empty := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	future, boundary, historical := uuid.New(), uuid.New(), uuid.New()
	type repository struct {
		table string
		rows  int
		has   func(context.Context, uuid.UUID, time.Time) (bool, error)
		count func(context.Context, uuid.UUID, time.Time, time.Time) (int64, error)
	}
	// This candidate stays in the experiment: the empty-project measurements
	// do not establish a fix for the production timeout.
	candidate := func(table string) func(context.Context, uuid.UUID, time.Time) (bool, error) {
		return func(ctx context.Context, project uuid.UUID, cutoff time.Time) (bool, error) {
			var exists uint8
			err := conn.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM "+table+" WHERE project_id = ? AND recorded_at <= ? LIMIT 1)", project, cutoff).Scan(&exists)
			return exists != 0, err
		}
	}
	repos := []repository{
		{"endpoints_v2", 5100000, candidate("endpoints_v2"), EndpointRepository.CountBetween},
		{"exceptions_v2", 510000, candidate("exceptions_v2"), ExceptionStackTraceRepository.CountBetween},
	}
	for _, repo := range repos {
		t.Run(repo.table, func(t *testing.T) {
			// 30 daily partitions, with most rows in one project and the rest in another.
			seed := fmt.Sprintf(`INSERT INTO %s (id, project_id, recorded_at)
    SELECT generateUUIDv4(), if(intDiv(number, 30) %% 5 = 0, toUUID('%s'), toUUID('%s')),
     toDateTime64('2026-10-01 00:00:00', 6, 'UTC') + toIntervalSecond((number %% 30) * 86400 + intDiv(number, 30) %% 3600)
    FROM numbers(%d)`, repo.table, other, heavy, repo.rows)
			if err := conn.Exec(ctx, seed); err != nil {
				t.Fatal(err)
			}
			var seededRows, partitions uint64
			if err := conn.QueryRow(ctx, "SELECT sum(rows), uniqExact(partition) FROM system.parts WHERE active AND database = ? AND table = ?", database, repo.table).Scan(&seededRows, &partitions); err != nil {
				t.Fatal(err)
			}
			if seededRows != uint64(repo.rows) || partitions != 30 {
				t.Fatalf("seed: rows=%d partitions=%d", seededRows, partitions)
			}
			t.Logf("FIXTURE table=%s rows=%d daily_partitions=%d", repo.table, seededRows, partitions)
			for _, row := range []struct {
				project uuid.UUID
				at      time.Time
			}{{future, now.Add(time.Hour)}, {boundary, now}, {historical, now.Add(-48 * time.Hour)}} {
				if err := conn.Exec(ctx, "INSERT INTO "+repo.table+" (id, project_id, recorded_at) VALUES (?, ?, ?)", uuid.New(), row.project, row.at); err != nil {
					t.Fatal(err)
				}
			}
			for _, tc := range []struct {
				name    string
				project uuid.UUID
				cutoff  time.Time
				want    bool
			}{
				{"heavy", heavy, now, true}, {"other_project", other, now, true}, {"empty", empty, now, false},
				{"future_only", future, now, false}, {"future_at_boundary", future, now.Add(time.Hour), true},
				{"inclusive_boundary", boundary, now, true}, {"before_boundary", boundary, now.Add(-time.Microsecond), false},
				{"historical", historical, now, true},
			} {
				got, err := repo.has(ctx, tc.project, tc.cutoff)
				if err != nil || got != tc.want {
					t.Fatalf("%s: got %v, %v; want %v", tc.name, got, err, tc.want)
				}
				n, err := repo.count(ctx, tc.project, time.Time{}, tc.cutoff)
				if err != nil || (n > 0) != tc.want {
					t.Fatalf("baseline %s: count=%d err=%v; want data=%v", tc.name, n, err, tc.want)
				}
			}
			for _, scenario := range []struct {
				name    string
				project uuid.UUID
			}{{"heavy", heavy}, {"empty", empty}} {
				for iteration := range 5 {
					for _, method := range []string{"count", "exists"} {
						qid := fmt.Sprintf("%s_%s_%s_%s_%d", database, repo.table, scenario.name, method, iteration)
						queryCtx := ch.Context(ctx, ch.WithQueryID(qid))
						started := time.Now()
						var got bool
						if method == "count" {
							n, err := repo.count(queryCtx, scenario.project, time.Time{}, now)
							if err != nil {
								t.Fatal(err)
							}
							if scenario.name == "heavy" && n != int64(repo.rows*4/5) {
								t.Fatalf("count = %d; want %d", n, repo.rows*4/5)
							}
							got = n > 0
						} else {
							var err error
							got, err = repo.has(queryCtx, scenario.project, now)
							if err != nil {
								t.Fatal(err)
							}
						}
						elapsed := time.Since(started)
						if got != (scenario.name == "heavy") {
							t.Fatalf("%s %s answered %v", method, scenario.name, got)
						}
						if err := conn.Exec(ctx, "SYSTEM FLUSH LOGS"); err != nil {
							t.Fatal(err)
						}
						var duration, readRows, readBytes uint64
						if err := conn.QueryRow(ctx, "SELECT query_duration_ms, read_rows, read_bytes FROM system.query_log WHERE query_id = ? AND type = 'QueryFinish' ORDER BY event_time_microseconds DESC LIMIT 1", qid).Scan(&duration, &readRows, &readBytes); err != nil {
							t.Fatal(err)
						}
						t.Logf("MEASURE table=%s scenario=%s method=%s iteration=%d wall_ms=%.3f server_ms=%d read_rows=%d read_bytes=%d", repo.table, scenario.name, method, iteration, float64(elapsed.Microseconds())/1000, duration, readRows, readBytes)
					}
				}
			}
			cancelled, cancel := context.WithCancel(ctx)
			cancel()
			if _, err := repo.has(cancelled, heavy, now); err == nil {
				t.Fatal("cancelled query must return an error")
			}
		})
	}
	t.Run("log_search_spans", func(t *testing.T) {
		at := time.Now().UTC().Truncate(time.Second)
		seed := fmt.Sprintf(`INSERT INTO log_records (id, project_id, timestamp, service_name, body)
   SELECT generateUUIDv4(), if(number %% 5 = 0, toUUID('%s'), toUUID('%s')),
    toDateTime64('%s', 9, 'UTC') - toIntervalSecond(number %% 3600), 'synthetic',
    if(number %% 2 = 0, 'synthetic error', 'synthetic info') FROM numbers(200000)`, other, heavy, at.Format("2006-01-02 15:04:05"))
		if err := conn.Exec(ctx, seed); err != nil {
			t.Fatal(err)
		}
		recorder := tracetest.NewSpanRecorder()
		provider := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(recorder))
		defer provider.Shutdown(ctx)
		restore := traceway.Install(traceway.Providers{TracerProvider: provider})
		defer restore()
		parentCtx, parent := provider.Tracer("query-validation").Start(ctx, "log search request")
		defer parent.End()
		params := shared.LogSearchParams{ProjectId: heavy, FromDate: at.Add(-time.Hour), ToDate: at,
			Search: "error", SearchType: "body", Page: 2, PageSize: 10, OrderBy: "timestamp", SortDirection: "desc"}
		records, total, err := LogRecordRepository.Search(parentCtx, params)
		if err != nil || total != 80000 || len(records) != 10 {
			t.Fatalf("search: rows=%d total=%d err=%v", len(records), total, err)
		}
		for i, record := range records {
			if record.ProjectId != heavy || record.Body != "synthetic error" {
				t.Fatalf("wrong project or filter: %+v", record)
			}
			if i > 0 && record.Timestamp.After(records[i-1].Timestamp) {
				t.Fatal("rows not in timestamp order")
			}
		}
		found := map[string]bool{}
		for _, span := range recorder.Ended() {
			if span.Name() != "logs.count" && span.Name() != "logs.fetch" {
				continue
			}
			if span.Parent().SpanID() != parent.SpanContext().SpanID() || span.SpanContext().TraceID() != parent.SpanContext().TraceID() {
				t.Fatal("log phase detached from request")
			}
			found[span.Name()] = true
			t.Logf("SPAN name=%s duration=%s", span.Name(), span.EndTime().Sub(span.StartTime()))
		}
		if !found["logs.count"] || !found["logs.fetch"] {
			t.Fatalf("missing phase spans: %v", found)
		}
		params.ProjectId = empty
		records, total, err = LogRecordRepository.Search(parentCtx, params)
		if err != nil || total != 0 || len(records) != 0 {
			t.Fatalf("empty search: rows=%d total=%d err=%v", len(records), total, err)
		}
	})

}
