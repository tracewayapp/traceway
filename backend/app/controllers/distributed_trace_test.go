//go:build !telemetry_ch && !transactional_pg && !telemetry_duckdb

package controllers

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/tracewayapp/traceway/backend/app/db"
	"github.com/tracewayapp/traceway/backend/app/models"
	"github.com/tracewayapp/traceway/backend/app/repositories/telemetry"
	"github.com/tracewayapp/traceway/backend/app/repositories/transactional"
)

const (
	cardTrace   = "0102030405060708090a0b0c0d0e0f10"
	cardBrowser = "a1a2a3a4a5a6a7a8a9aaabacadaeaf00"
)

func cardSpan(project uuid.UUID, trace, id, parent, name string, at time.Time) models.Span {
	return models.Span{ProjectId: project, TraceId: trace, SpanId: id, ParentSpanId: parent, Name: name, StartTime: at, RecordedAt: at, Duration: time.Millisecond}
}

func openCard(t *testing.T, userID int, traceId, body string) (DistributedTraceResponse, int, *gin.Context) {
	t.Helper()
	c, recorder := newControllerTestContext(t, nil, userID, http.MethodPost, "/distributed-traces/"+traceId, body)
	c.Params = gin.Params{{Key: "traceId", Value: traceId}}
	DistributedTraceController.GetDistributedTrace(c)
	var response DistributedTraceResponse
	if recorder.Code == http.StatusOK {
		if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
			t.Fatalf("%v: %s", err, recorder.Body.String())
		}
	}
	return response, recorder.Code, c
}

func TestGetDistributedTraceIncludesOwnedSpans(t *testing.T) {
	setupSetupControllerDB(t)
	tx, err := db.DB.Begin()
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()
	userID, orgID := createSetupTestAccount(t, tx, "distributed@example.com", "owner")
	_, privateOrgID := createSetupTestAccount(t, tx, "private@example.com", "owner")
	createProject := func(name string, org int) uuid.UUID {
		project, err := transactional.ProjectRepository.CreateWithOrganization(tx, name, "opentelemetry", org)
		if err != nil {
			t.Fatal(err)
		}
		return project.Id
	}
	api, worker := createProject("API", orgID), createProject("Worker", orgID)
	privateProject := createProject("Private", privateOrgID)
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Millisecond)
	// GET /jobs (e1) -> queue publish (c1) -> worker task (k1) -> query (k2). The AI call (a1) and an empty task (k9) hang under e1.
	if err := telemetry.EndpointRepository.InsertAsync(ctx, []models.Endpoint{
		{Id: uuid.New(), ProjectId: api, Endpoint: "GET /jobs", RecordedAt: now, TraceId: cardTrace, SpanId: "e100000000000001", IsRoot: true},
		{Id: uuid.New(), ProjectId: privateProject, Endpoint: "GET /private", RecordedAt: now, TraceId: cardTrace, SpanId: "e900000000000009", IsRoot: true},
	}); err != nil {
		t.Fatal(err)
	}
	if err := telemetry.TaskRepository.InsertAsync(ctx, []models.Task{
		{Id: uuid.New(), ProjectId: worker, TaskName: "worker", RecordedAt: now, TraceId: cardTrace, SpanId: "7a00000000000001", ParentSpanId: "c100000000000001"},
		{Id: uuid.New(), ProjectId: api, TaskName: "empty", RecordedAt: now, TraceId: cardTrace, SpanId: "7a00000000000009", ParentSpanId: "e100000000000001"},
	}); err != nil {
		t.Fatal(err)
	}
	if err := telemetry.AiTraceRepository.InsertAsync(ctx, []models.AiTrace{{
		Id: uuid.New(), ProjectId: api, TraceName: "chat", RecordedAt: now, TraceId: cardTrace, SpanId: "a100000000000001", ParentSpanId: "e100000000000001",
	}}); err != nil {
		t.Fatal(err)
	}
	if err := telemetry.ExceptionStackTraceRepository.InsertAsync(ctx, []models.ExceptionStackTrace{
		{Id: uuid.New(), ProjectId: worker, TraceId: cardTrace, SpanId: "7b00000000000001", ExceptionHash: "worker-error", RecordedAt: now},
		{Id: uuid.New(), ProjectId: worker, TraceId: cardTrace, SpanId: "0f00000000000001", ExceptionHash: "orphan", RecordedAt: now},
		{Id: uuid.New(), ProjectId: api, ExceptionHash: "no-trace", RecordedAt: now},
	}); err != nil {
		t.Fatal(err)
	}
	first := cardSpan(api, cardTrace, "c100000000000001", "e100000000000001", "first", now)
	first.Attributes = map[string]string{"db.system": "postgresql"}
	if err := telemetry.SpanRepository.InsertAsync(ctx, []models.Span{
		cardSpan(api, cardTrace, "e100000000000001", "", "GET /jobs", now),
		cardSpan(api, cardTrace, "b200000000000001", "e100000000000001", "second", now.Add(time.Second)),
		first,
		cardSpan(api, cardTrace, "a100000000000001", "e100000000000001", "chat", now.Add(2*time.Second)),
		cardSpan(api, cardTrace, "a200000000000001", "a100000000000001", "ai-child", now.Add(2*time.Second)),
		cardSpan(api, cardTrace, "7a00000000000009", "e100000000000001", "empty", now.Add(3*time.Second)),
		cardSpan(worker, cardTrace, "7a00000000000001", "c100000000000001", "worker", now),
		cardSpan(worker, cardTrace, "7b00000000000001", "7a00000000000001", "worker-child", now),
		cardSpan(worker, cardTrace, "0f00000000000001", "0e00000000000001", "orphan-span", now),
		cardSpan(worker, cardTrace, "0f00000000000002", "0f00000000000001", "orphan-child", now),
		cardSpan(privateProject, cardTrace, "e900000000000009", "", "private-child", now),
		cardSpan(api, cardTrace, "b300000000000001", "e100000000000001", "outside-window", now.Add(-25*time.Hour)),
	}); err != nil {
		t.Fatal(err)
	}

	names := func(spans []models.Span) string {
		out := ""
		for _, span := range spans {
			out += span.Name + " "
		}
		return out
	}
	// The dashed spelling of the id is what older links carry.
	dashed := uuid.MustParse(cardTrace).String()
	for _, request := range [][2]string{{cardTrace, "{}"}, {dashed, `{"recordedAt":"` + now.Format(time.RFC3339Nano) + `"}`}} {
		response, status, c := openCard(t, userID, request[0], request[1])
		if status != http.StatusOK || response.TraceId != cardTrace {
			t.Fatalf("status %d, trace %q, errors: %v", status, response.TraceId, c.Errors)
		}
		if len(response.Nodes) != 5 {
			t.Fatalf("got %d nodes, want the endpoint, two tasks, the AI trace and the orphan exception: %+v", len(response.Nodes), response.Nodes)
		}
		for _, node := range response.Nodes {
			if node.Spans == nil {
				t.Fatal("spans must serialize as an array")
			}
			switch {
			case node.Endpoint != nil:
				if got := names(node.Spans); got != "first second chat ai-child empty " {
					t.Fatalf("the endpoint's subtree stays in its own project and inside the window: %q", got)
				}
				if node.Exception != nil || node.ParentEntitySpanId != "" || node.SpanId != "e100000000000001" || node.Spans[0].Attributes["db.system"] != "postgresql" {
					t.Fatalf("incorrect endpoint node: %+v", node)
				}
			case node.Task != nil && node.Task.TaskName == "worker":
				if node.ProjectId != worker || names(node.Spans) != "worker-child " || node.ParentEntitySpanId != "e100000000000001" {
					t.Fatalf("worker node: %+v", node)
				}
				if node.Exception == nil || node.Exception.ExceptionHash != "worker-error" {
					t.Fatalf("an exception below the task's span belongs to the task: %+v", node.Exception)
				}
			case node.Task != nil:
				if len(node.Spans) != 0 || node.ParentEntitySpanId != "e100000000000001" || node.Exception != nil {
					t.Fatalf("empty task: %+v", node)
				}
			case node.AiTrace != nil:
				if names(node.Spans) != "ai-child " || node.ParentEntitySpanId != "e100000000000001" {
					t.Fatalf("AI node: %+v", node)
				}
			case node.TraceType == "exception":
				if node.Exception.ExceptionHash != "orphan" || names(node.Spans) != "orphan-child " || node.ProjectName != "Worker" {
					t.Fatalf("an exception with nothing promoted above it stands alone with the spans under its own: %+v", node)
				}
			default:
				t.Fatalf("unexpected node: %+v", node)
			}
		}
	}

	if _, status, _ := openCard(t, userID, "not-a-trace-id", "{}"); status != http.StatusBadRequest {
		t.Fatalf("an id that is not 32 hex characters: %d", status)
	}
	if _, err := db.TelemetryDB.Exec("DROP TABLE spans_v2"); err != nil {
		t.Fatal(err)
	}
	if _, status, c := openCard(t, userID, cardTrace, "{}"); status != http.StatusInternalServerError || len(c.Errors) == 0 {
		t.Fatalf("span lookup failure must return 500, got %d", status)
	}
	if response, status, _ := openCard(t, userID, "ffffffffffffffffffffffffffffffff", "{}"); status != http.StatusOK || response.Nodes == nil || len(response.Nodes) != 0 {
		t.Fatalf("a trace nobody recorded skips the span reads and returns []: %d %+v", status, response)
	}
}

func TestDistributedTraceNestsAcrossAnUnpromotedHopInAnotherProject(t *testing.T) {
	setupSetupControllerDB(t)
	tx, err := db.DB.Begin()
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()
	userID, orgID := createSetupTestAccount(t, tx, "hops@example.com", "owner")
	createProject := func(name string) uuid.UUID {
		project, err := transactional.ProjectRepository.CreateWithOrganization(tx, name, "opentelemetry", orgID)
		if err != nil {
			t.Fatal(err)
		}
		return project.Id
	}
	gateway, inventory, warehouse := createProject("gateway"), createProject("inventory-grpc"), createProject("warehouse")
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Millisecond)
	// The gRPC hop reports to its own project and is never promoted, so no entity's subtree holds it.
	if err := telemetry.SpanRepository.InsertAsync(ctx, []models.Span{
		cardSpan(gateway, cardTrace, "0101010101010101", "", "GET /api/stock", now), cardSpan(gateway, cardTrace, "0202020202020202", "0101010101010101", "inventory.Stock/Get", now),
		cardSpan(inventory, cardTrace, "0303030303030303", "0202020202020202", "inventory.Stock/Get", now), cardSpan(inventory, cardTrace, "0404040404040404", "0303030303030303", "GET", now),
		cardSpan(warehouse, cardTrace, "0505050505050505", "0404040404040404", "GET /shelves", now),
	}); err != nil {
		t.Fatal(err)
	}
	if err := telemetry.EndpointRepository.InsertAsync(ctx, []models.Endpoint{
		{Id: uuid.New(), ProjectId: gateway, Endpoint: "GET /api/stock", RecordedAt: now, TraceId: cardTrace, SpanId: "0101010101010101", IsRoot: true},
		{Id: uuid.New(), ProjectId: warehouse, Endpoint: "GET /shelves", RecordedAt: now, TraceId: cardTrace, SpanId: "0505050505050505", ParentSpanId: "0404040404040404"},
	}); err != nil {
		t.Fatal(err)
	}

	response, status, _ := openCard(t, userID, cardTrace, `{"recordedAt":"`+now.Format(time.RFC3339Nano)+`"}`)
	if status != http.StatusOK || len(response.Nodes) != 2 {
		t.Fatalf("status %d: %+v", status, response)
	}
	for _, node := range response.Nodes {
		switch node.Endpoint.Endpoint {
		case "GET /shelves":
			if node.ParentEntitySpanId != "0101010101010101" {
				t.Fatalf("warehouse must nest under gateway through the unpromoted gRPC hop, got parent %q", node.ParentEntitySpanId)
			}
		case "GET /api/stock":
			if node.ParentEntitySpanId != "" {
				t.Fatalf("the first entity of the trace has nothing above it, got %q", node.ParentEntitySpanId)
			}
		}
	}
}

// The browser SDK sends its own id in `traceway-trace-id`. Only the gateway receives that header, so only its rows
// link to the browser's trace. Everything keeps the trace id it arrived with, and the card follows the link both ways.
func TestDistributedTraceIsWholeUnderEitherOfItsIds(t *testing.T) {
	setupSetupControllerDB(t)
	tx, err := db.DB.Begin()
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()
	userID, orgID := createSetupTestAccount(t, tx, "aliases@example.com", "owner")
	createProject := func(name, framework string) uuid.UUID {
		project, err := transactional.ProjectRepository.CreateWithOrganization(tx, name, framework, orgID)
		if err != nil {
			t.Fatal(err)
		}
		return project.Id
	}
	web, gateway, payments, warehouse := createProject("web", "react"), createProject("gateway", "opentelemetry"), createProject("payments", "opentelemetry"), createProject("warehouse", "opentelemetry")
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Millisecond)
	if err := telemetry.SpanRepository.InsertAsync(ctx, []models.Span{
		cardSpan(gateway, cardTrace, "0101010101010101", "", "POST /one", now), cardSpan(gateway, cardTrace, "0202020202020202", "0101010101010101", "POST", now),
		cardSpan(payments, cardTrace, "0303030303030303", "0202020202020202", "POST /two", now), cardSpan(payments, cardTrace, "0404040404040404", "0303030303030303", "POST", now),
		cardSpan(warehouse, cardTrace, "0505050505050505", "0404040404040404", "POST /three", now),
	}); err != nil {
		t.Fatal(err)
	}
	if err := telemetry.EndpointRepository.InsertAsync(ctx, []models.Endpoint{
		{Id: uuid.New(), ProjectId: gateway, Endpoint: "POST /one", RecordedAt: now, TraceId: cardTrace, SpanId: "0101010101010101", Attributes: map[string]string{"traceway.distributed_trace_id": cardBrowser}, IsRoot: true},
		{Id: uuid.New(), ProjectId: payments, Endpoint: "POST /two", RecordedAt: now, TraceId: cardTrace, SpanId: "0303030303030303", ParentSpanId: "0202020202020202"},
		{Id: uuid.New(), ProjectId: warehouse, Endpoint: "POST /three", RecordedAt: now, TraceId: cardTrace, SpanId: "0505050505050505", ParentSpanId: "0404040404040404"},
	}); err != nil {
		t.Fatal(err)
	}
	if err := telemetry.ExceptionStackTraceRepository.InsertAsync(ctx, []models.ExceptionStackTrace{
		{Id: uuid.New(), ProjectId: web, ExceptionHash: "browser-error", RecordedAt: now, TraceId: cardTrace},
	}); err != nil {
		t.Fatal(err)
	}

	for opened, id := range map[string]string{"the shared W3C trace ID": cardTrace} {
		response, status, _ := openCard(t, userID, id, `{"recordedAt":"`+now.Format(time.RFC3339Nano)+`"}`)
		if status != http.StatusOK {
			t.Fatalf("%s: status %d", opened, status)
		}
		parents, browserErrors := map[string]string{}, 0
		for _, node := range response.Nodes {
			if node.Endpoint != nil {
				parents[node.Endpoint.Endpoint] = node.ParentEntitySpanId
			} else if node.Exception != nil && node.Exception.ExceptionHash == "browser-error" {
				browserErrors++
			}
		}
		if len(response.Nodes) != 4 || browserErrors != 1 || len(parents) != 3 || parents["POST /two"] != "0101010101010101" || parents["POST /three"] != "0303030303030303" || parents["POST /one"] != "" {
			t.Fatalf("opened by %s: want all three endpoints nested plus the browser error once, got %d nodes, parents %v, browser errors %d", opened, len(response.Nodes), parents, browserErrors)
		}
	}
}

func TestTraceEntityLookupKeepsMigratedIdentitiesAndBoundsMissingTime(t *testing.T) {
	setupSetupControllerDB(t)
	ctx := context.Background()
	project, id := uuid.New(), uuid.New()
	now := time.Now().UTC()
	rows := []models.Endpoint{
		{Id: id, ProjectId: project, TraceId: cardTrace, Attributes: map[string]string{"traceway.distributed_trace_id": cardBrowser}, SpanId: "0102030405060708", RecordedAt: now},
		{Id: id, ProjectId: project, TraceId: cardBrowser, SpanId: "0102030405060708", RecordedAt: now},
		{Id: uuid.New(), ProjectId: project, TraceId: cardTrace, SpanId: "0102030405060709", RecordedAt: now.Add(-7 * 24 * time.Hour)},
	}
	if err := telemetry.EndpointRepository.InsertAsync(ctx, rows); err != nil {
		t.Fatal(err)
	}
	found, err := findTraceEntities(ctx, cardBrowser, []uuid.UUID{project}, nil)
	if err != nil || len(found.endpoints) != 1 {
		t.Fatalf("unanchored lookup lost an identity or scanned old history: %+v, %v", found, err)
	}
	old := rows[2].RecordedAt
	found, err = findTraceEntities(ctx, cardTrace, []uuid.UUID{project}, &old)
	if err != nil || len(found.endpoints) != 1 || found.endpoints[0].Id != rows[2].Id {
		t.Fatalf("historical anchor: %+v, %v", found, err)
	}
}

func TestDistributedTraceRejectsMalformedAnchor(t *testing.T) {
	_, code, _ := openCard(t, 1, cardTrace, `{"recordedAt":"invalid"}`)
	if code != http.StatusBadRequest {
		t.Fatalf("invalid date must not silently become an unanchored scan: %d", code)
	}
}
