package telemetry

import (
	"context"
	"encoding/hex"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/tracewayapp/traceway/backend/app/db"
	"github.com/tracewayapp/traceway/backend/app/models"
	"github.com/tracewayapp/traceway/backend/app/repositories/telemetry/shared"
)

func sloTestBase() time.Time {
	return time.Now().UTC().Truncate(time.Hour).Add(-2 * time.Hour)
}

func bucketAt(points []shared.SLOBucketPoint, at time.Time) (shared.SLOBucketPoint, bool) {
	for _, point := range points {
		if point.Bucket.Equal(at) {
			return point, true
		}
	}
	return shared.SLOBucketPoint{}, false
}

func expectBucket(t *testing.T, points []shared.SLOBucketPoint, at time.Time, good, total int64) {
	t.Helper()
	point, ok := bucketAt(points, at)
	if !ok {
		t.Fatalf("no bucket at %s in %+v", at, points)
	}
	if point.Good != good || point.Total != total {
		t.Fatalf("bucket %s = good %d / total %d, want %d / %d", at, point.Good, point.Total, good, total)
	}
}

func TestSLOBucketRepository_UpsertSumSeries(t *testing.T) {
	setupTestDB(t)
	ctx := context.Background()
	project := uuid.New()
	base := sloTestBase()
	now := time.Now().UTC().Add(-time.Minute)
	rows := []shared.SLOBucket{
		{ProjectId: project, SLOId: 7, BucketStart: base, Good: 90, Total: 100, ComputedAt: now},
		{ProjectId: project, SLOId: 7, BucketStart: base.Add(time.Minute), Good: 100, Total: 100, ComputedAt: now},
		{ProjectId: project, SLOId: 7, BucketStart: base.Add(2 * time.Minute), Good: 50, Total: 100, ComputedAt: now},
		{ProjectId: project, SLOId: 8, BucketStart: base, Good: 1, Total: 1, ComputedAt: now},
	}
	if err := SLOBucketRepository.Upsert(ctx, rows); err != nil {
		t.Fatal(err)
	}
	if err := SLOBucketRepository.Upsert(ctx, []shared.SLOBucket{{ProjectId: project, SLOId: 7, BucketStart: base, Good: 95, Total: 100, ComputedAt: now.Add(time.Second)}}); err != nil {
		t.Fatal(err)
	}

	good, total, err := SLOBucketRepository.SumRange(ctx, project, 7, base, base.Add(3*time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	if good != 245 || total != 300 {
		t.Fatalf("sum = %d/%d, want 245/300", good, total)
	}
	good, total, err = SLOBucketRepository.SumRange(ctx, project, 7, base, base.Add(2*time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	if good != 195 || total != 200 {
		t.Fatalf("upper bound is exclusive: sum = %d/%d, want 195/200", good, total)
	}

	series, err := SLOBucketRepository.Series(ctx, project, 7, base, base.Add(4*time.Minute), 2)
	if err != nil {
		t.Fatal(err)
	}
	if len(series) != 2 {
		t.Fatalf("series re-bucketed to 2 minutes: %+v", series)
	}
	expectBucket(t, series, base, 195, 200)
	expectBucket(t, series, base.Add(2*time.Minute), 50, 100)

	series, err = SLOBucketRepository.Series(ctx, project, 7, base.Add(time.Minute), base.Add(4*time.Minute), 2)
	if err != nil {
		t.Fatal(err)
	}
	expectBucket(t, series, base.Add(time.Minute), 100, 100)
	expectBucket(t, series, base.Add(2*time.Minute), 50, 100)

	sums, err := SLOBucketRepository.SnapshotSums(ctx, project, 7, base, base.Add(3*time.Minute), base.Add(time.Minute), base.Add(2*time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	if sums.Window != (shared.SLOCount{Good: 245, Total: 300}) || sums.LastSixHours != (shared.SLOCount{Good: 150, Total: 200}) || sums.LastHour != (shared.SLOCount{Good: 50, Total: 100}) {
		t.Fatalf("snapshot sums = %+v", sums)
	}

	if err := SLOBucketRepository.DeleteBySLO(ctx, project, 7); err != nil {
		t.Fatal(err)
	}
	good, total, err = SLOBucketRepository.SumRange(ctx, project, 7, base, base.Add(3*time.Minute))
	if err != nil || good != 0 || total != 0 {
		t.Fatalf("buckets survive delete: %d/%d, %v", good, total, err)
	}
	good, total, err = SLOBucketRepository.SumRange(ctx, project, 8, base, base.Add(time.Minute))
	if err != nil || good != 1 || total != 1 {
		t.Fatalf("delete touched another SLO: %d/%d, %v", good, total, err)
	}
	if err := SLOBucketRepository.Upsert(ctx, []shared.SLOBucket{{ProjectId: project, SLOId: 7, BucketStart: base.Add(time.Minute), Good: 1, Total: 1, ComputedAt: now}}); err != nil {
		t.Fatal(err)
	}
	if err := SLOBucketRepository.Upsert(ctx, []shared.SLOBucket{{ProjectId: project, SLOId: 7, BucketStart: base, Good: 3, Total: 4, ComputedAt: time.Now().UTC().Add(time.Second)}}); err != nil {
		t.Fatal(err)
	}
	good, total, err = SLOBucketRepository.SumRange(ctx, project, 7, base, base.Add(3*time.Minute))
	wantGood, wantTotal := int64(4), int64(5)
	if db.TelemetryBackendName() == "clickhouse" {
		wantGood, wantTotal = 3, 4
	}
	if err != nil || good != wantGood || total != wantTotal {
		t.Fatalf("after delete: %d/%d (%v), want %d/%d", good, total, err, wantGood, wantTotal)
	}
	if db.TelemetryBackendName() != "clickhouse" {
		if err := SLOBucketRepository.PruneBefore(ctx, base.Add(time.Minute)); err != nil {
			t.Fatal(err)
		}
		good, total, err = SLOBucketRepository.SumRange(ctx, project, 8, base, base.Add(time.Minute))
		if err != nil || total != 0 {
			t.Fatalf("prune kept an old bucket: %d/%d, %v", good, total, err)
		}
	}
}

func TestSLOBucketRepository_DeleteRightAfterTheWrite(t *testing.T) {
	setupTestDB(t)
	ctx := context.Background()
	project := uuid.New()
	base := sloTestBase()
	var rows []shared.SLOBucket
	for minute := 0; minute < 30; minute++ {
		rows = append(rows, shared.SLOBucket{ProjectId: project, SLOId: 3, BucketStart: base.Add(time.Duration(minute) * time.Minute), Good: 1, Total: 1, ComputedAt: time.Now().UTC()})
	}
	for attempt := 0; attempt < 3; attempt++ {
		if err := SLOBucketRepository.Upsert(ctx, rows); err != nil {
			t.Fatal(err)
		}
		if err := SLOBucketRepository.DeleteBySLO(ctx, project, 3); err != nil {
			t.Fatal(err)
		}
		if _, total, err := SLOBucketRepository.SumRange(ctx, project, 3, base, base.Add(time.Hour)); err != nil || total != 0 {
			t.Fatalf("attempt %d: a delete in the same second as the write left %d events (%v)", attempt, total, err)
		}
		for i := range rows {
			rows[i].ComputedAt = time.Now().UTC()
		}
	}
}

func TestEndpointRepository_SLICounts(t *testing.T) {
	setupTestDB(t)
	ctx := context.Background()
	project := uuid.New()
	base := sloTestBase()
	first, second := base, base.Add(time.Minute)
	endpoints := []models.Endpoint{
		makeEndpoint(project, "GET /users", 100*time.Millisecond, 200, first.Add(5*time.Second)),
		makeEndpoint(project, "GET /users", 100*time.Millisecond, 200, first.Add(10*time.Second)),
		makeEndpoint(project, "GET /users", 100*time.Millisecond, 404, first.Add(15*time.Second)),
		makeEndpoint(project, "GET /users", 100*time.Millisecond, 500, first.Add(20*time.Second)),
		makeEndpoint(project, "GET /orders", 100*time.Millisecond, 200, second.Add(5*time.Second)),
		makeEndpoint(project, "GET /orders", 900*time.Millisecond, 200, second.Add(10*time.Second)),
		makeEndpoint(project, "GET /users", 100*time.Millisecond, 200, second.Add(40*time.Second)),
		makeEndpoint(project, "GET /users", 100*time.Millisecond, 200, base.Add(2*time.Minute)),
	}
	stream := makeEndpoint(project, "GET /events", 30*time.Second, 200, second.Add(20*time.Second))
	stream.IsStream = true
	retried := endpoints[3]
	endpoints = append(endpoints, stream, retried)
	if err := EndpointRepository.InsertAsync(ctx, endpoints); err != nil {
		t.Fatal(err)
	}

	availability, err := EndpointRepository.SLICounts(ctx, project, shared.EndpointSLISpec{}, base, base.Add(2*time.Minute), 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(availability) != 2 {
		t.Fatalf("two buckets inside [from, to): %+v", availability)
	}
	expectBucket(t, availability, first, 3, 4)
	expectBucket(t, availability, second, 4, 4)

	latency, err := EndpointRepository.SLICounts(ctx, project, shared.EndpointSLISpec{Latency: true, LatencyThreshold: 500 * time.Millisecond}, base, base.Add(2*time.Minute), 1)
	if err != nil {
		t.Fatal(err)
	}
	expectBucket(t, latency, second, 2, 3)

	named, err := EndpointRepository.SLICounts(ctx, project, shared.EndpointSLISpec{Endpoint: "GET /orders"}, base, base.Add(2*time.Minute), 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(named) != 1 {
		t.Fatalf("endpoint filter: %+v", named)
	}
	expectBucket(t, named, second, 2, 2)

	wide, err := EndpointRepository.SLICounts(ctx, project, shared.EndpointSLISpec{}, base, base.Add(10*time.Minute), 5)
	if err != nil {
		t.Fatal(err)
	}
	expectBucket(t, wide, base.Truncate(5*time.Minute), 8, 9)
}

func TestTaskRepository_SLICounts(t *testing.T) {
	setupTestDB(t)
	ctx := context.Background()
	project := uuid.New()
	base := sloTestBase()
	failing := makeTask(project, "send-email", time.Second, base.Add(10*time.Second))
	failing.TraceId, failing.SpanId = hexId(uuid.New()), hexId(uuid.New())
	healthy := makeTask(project, "send-email", time.Second, base.Add(20*time.Second))
	healthy.TraceId, healthy.SpanId = hexId(uuid.New()), hexId(uuid.New())
	other := makeTask(project, "rebuild-index", time.Second, base.Add(30*time.Second))
	other.TraceId, other.SpanId = hexId(uuid.New()), hexId(uuid.New())
	if err := TaskRepository.InsertAsync(ctx, []models.Task{failing, healthy, other, healthy}); err != nil {
		t.Fatal(err)
	}
	onTask := makeException(project, "hash-a", "boom", base.Add(5*time.Minute))
	onTask.TraceType, onTask.TraceId, onTask.SpanId = "task", failing.TraceId, failing.SpanId
	onChild := makeException(project, "hash-b", "caught", base.Add(15*time.Second))
	onChild.TraceType, onChild.TraceId, onChild.SpanId = "task", healthy.TraceId, hexId(uuid.New())
	if err := ExceptionStackTraceRepository.InsertAsync(ctx, []models.ExceptionStackTrace{onTask, onChild}); err != nil {
		t.Fatal(err)
	}

	all, err := TaskRepository.SLICounts(ctx, project, shared.TaskSLISpec{}, base, base.Add(time.Minute), 1)
	if err != nil {
		t.Fatal(err)
	}
	expectBucket(t, all, base, 2, 3)

	named, err := TaskRepository.SLICounts(ctx, project, shared.TaskSLISpec{TaskName: "send-email"}, base, base.Add(time.Minute), 1)
	if err != nil {
		t.Fatal(err)
	}
	expectBucket(t, named, base, 1, 2)
}

func sloTestSpan(project uuid.UUID, name, service string, kind, status int32, duration time.Duration, at time.Time, attributes map[string]string) models.OtelSpan {
	trace, id := uuid.New(), uuid.New()
	return models.OtelSpan{Span: models.Span{
		ProjectId: project, TraceId: hex.EncodeToString(trace[:]), SpanId: hex.EncodeToString(id[8:]),
		Name: name, ServiceName: service, SpanKind: kind, StatusCode: status,
		StartTime: at, RecordedAt: at, Duration: duration, Attributes: attributes,
	}}
}

func TestOtelSpanRepository_SLICounts(t *testing.T) {
	setupTestDB(t)
	ctx := context.Background()
	project := uuid.New()
	base := sloTestBase()
	checkout := sloTestSpan(project, "POST /checkout", "api", 2, 0, 100*time.Millisecond, base.Add(5*time.Second), map[string]string{"http.route": "/checkout"})
	failed := sloTestSpan(project, "POST /checkout", "api", 2, 2, 100*time.Millisecond, base.Add(10*time.Second), map[string]string{"http.route": "/checkout"})
	slow := sloTestSpan(project, "GET /cart", "api", 2, 0, 2*time.Second, base.Add(15*time.Second), map[string]string{"http.route": "/cart"})
	worker := sloTestSpan(project, "process", "worker", 5, 0, 100*time.Millisecond, base.Add(20*time.Second), nil)
	if _, err := OtelSpanRepository.InsertAsync(ctx, []models.OtelSpan{checkout, failed, slow, worker}); err != nil {
		t.Fatal(err)
	}
	if _, err := OtelSpanRepository.InsertAsync(ctx, []models.OtelSpan{failed}); err != nil {
		t.Fatal(err)
	}

	all, err := OtelSpanRepository.SLICounts(ctx, shared.SpanSLISpec{Filter: shared.OtelSpanSearch{ProjectId: project}}, base, base.Add(time.Minute), 1)
	if err != nil {
		t.Fatal(err)
	}
	expectBucket(t, all, base, 3, 4)

	api, err := OtelSpanRepository.SLICounts(ctx, shared.SpanSLISpec{Filter: shared.OtelSpanSearch{ProjectId: project, Service: "api"}}, base, base.Add(time.Minute), 1)
	if err != nil {
		t.Fatal(err)
	}
	expectBucket(t, api, base, 2, 3)

	route, err := OtelSpanRepository.SLICounts(ctx, shared.SpanSLISpec{Filter: shared.OtelSpanSearch{ProjectId: project, Attributes: []shared.OtelAttributeFilter{{Key: "http.route", Value: "/checkout"}}}}, base, base.Add(time.Minute), 1)
	if err != nil {
		t.Fatal(err)
	}
	expectBucket(t, route, base, 1, 2)

	latency, err := OtelSpanRepository.SLICounts(ctx, shared.SpanSLISpec{Filter: shared.OtelSpanSearch{ProjectId: project, Name: "/"}, Latency: true, LatencyThreshold: time.Second}, base, base.Add(time.Minute), 1)
	if err != nil {
		t.Fatal(err)
	}
	expectBucket(t, latency, base, 2, 3)
}

func TestSessionRepository_SLICounts(t *testing.T) {
	setupTestDB(t)
	ctx := context.Background()
	project := uuid.New()
	base := sloTestBase()
	clean := models.Session{Id: uuid.New(), ProjectId: project, StartedAt: base.Add(5 * time.Second), AppVersion: "1.4.2", Attributes: map[string]string{}}
	crashed := models.Session{Id: uuid.New(), ProjectId: project, StartedAt: base.Add(10 * time.Second), AppVersion: "1.4.2", Attributes: map[string]string{}}
	logged := models.Session{Id: uuid.New(), ProjectId: project, StartedAt: base.Add(15 * time.Second), AppVersion: "1.4.1", Attributes: map[string]string{}}
	later := models.Session{Id: uuid.New(), ProjectId: project, StartedAt: base.Add(70 * time.Second), AppVersion: "1.4.2", Attributes: map[string]string{}}
	if err := SessionRepository.Upsert(ctx, []models.Session{clean, crashed, logged, later}); err != nil {
		t.Fatal(err)
	}
	crash := makeException(project, "crash", "boom", base.Add(20*time.Minute))
	crash.SessionId = &crashed.Id
	message := makeException(project, "note", "captured message", base.Add(40*time.Second))
	message.SessionId = &logged.Id
	message.IsMessage = true
	if err := ExceptionStackTraceRepository.InsertAsync(ctx, []models.ExceptionStackTrace{crash, message}); err != nil {
		t.Fatal(err)
	}

	all, err := SessionRepository.SLICounts(ctx, project, shared.SessionSLISpec{}, base, base.Add(2*time.Minute), 1)
	if err != nil {
		t.Fatal(err)
	}
	expectBucket(t, all, base, 2, 3)
	expectBucket(t, all, base.Add(time.Minute), 1, 1)

	strict, err := SessionRepository.SLICounts(ctx, project, shared.SessionSLISpec{IncludeMessages: true}, base, base.Add(time.Minute), 1)
	if err != nil {
		t.Fatal(err)
	}
	expectBucket(t, strict, base, 1, 3)

	version, err := SessionRepository.SLICounts(ctx, project, shared.SessionSLISpec{AppVersion: "1.4.2"}, base, base.Add(time.Minute), 1)
	if err != nil {
		t.Fatal(err)
	}
	expectBucket(t, version, base, 1, 2)
}
