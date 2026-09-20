//go:build !telemetry_ch && !transactional_pg && !telemetry_duckdb

package controllers

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/tracewayapp/traceway/backend/app/db"
	"github.com/tracewayapp/traceway/backend/app/middleware"
	"github.com/tracewayapp/traceway/backend/app/models"
	"github.com/tracewayapp/traceway/backend/app/repositories/telemetry"
	"github.com/tracewayapp/traceway/backend/app/repositories/transactional"
)

func TestSpanExplorerEndpoints(t *testing.T) {
	setupSetupControllerDB(t)
	tx, err := db.DB.Begin()
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()
	user, org := createSetupTestAccount(t, tx, "explorer@example.com", "owner")
	project, err := transactional.ProjectRepository.CreateWithOrganization(tx, "Explorer", "react", org)
	if err != nil {
		t.Fatal(err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
	trace, now := uuid.New(), time.Now().UTC()
	traceHex := hex.EncodeToString(trace[:])
	span := models.OtelSpan{Span: models.Span{ProjectId: project.Id, TraceId: traceHex, SpanId: "0102030405060708", Name: "documentLoad",
		ServiceName: "web", SpanKind: 1, StartTime: now.Add(-time.Minute), Duration: time.Second, Attributes: map[string]string{"url.path": "/checkout"}}}
	if _, err := telemetry.OtelSpanRepository.InsertAsync(context.Background(), []models.OtelSpan{span}); err != nil {
		t.Fatal(err)
	}
	call := func(method, path, body string, params gin.Params, handler gin.HandlerFunc) (int, []byte) {
		c, response := newControllerTestContext(t, nil, user, method, path, body)
		c.Set(middleware.ProjectIdContextKey, project.Id)
		c.Params = params
		handler(c)
		return response.Code, response.Body.Bytes()
	}
	search := func(body string) (int, SpanSearchResponse) {
		code, raw := call(http.MethodPost, "/spans/search", body, nil, SpanExplorerController.Search)
		var parsed SpanSearchResponse
		_ = json.Unmarshal(raw, &parsed)
		return code, parsed
	}
	pagination := `"pagination":{"page":1,"pageSize":50}`
	window := fmt.Sprintf(`"fromDate":%q,"toDate":%q`, now.Add(-time.Hour).Format(time.RFC3339Nano), now.Add(time.Minute).Format(time.RFC3339Nano))

	for name, body := range map[string]string{
		"no time range":         `{` + pagination + `}`,
		"range over a month":    fmt.Sprintf(`{"fromDate":%q,"toDate":%q,%s}`, now.Add(-40*24*time.Hour).Format(time.RFC3339Nano), now.Format(time.RFC3339Nano), pagination),
		"malformed trace id":    `{` + window + `,"traceId":"xyz",` + pagination + `}`,
		"quoted attribute key":  `{` + window + `,"attributeFilters":[{"key":"a\"b","value":"x"}],` + pagination + `}`,
		"empty attribute value": `{` + window + `,"attributeFilters":[{"key":"url.path","value":""}],` + pagination + `}`,
	} {
		if code, _ := search(body); code != http.StatusUnprocessableEntity {
			t.Fatalf("%s: a search is always bounded and validated, got %d", name, code)
		}
	}
	code, found := search(`{` + window + `,"attributeFilters":[{"key":"url.path","value":"/checkout"}],` + pagination + `}`)
	if code != http.StatusOK || found.Pagination.Total != 1 || len(found.Data) != 1 || found.Data[0].Name != "documentLoad" || len(found.Services) != 1 || found.Services[0] != "web" {
		t.Fatalf("a browser project's spans must be searchable: %d %+v", code, found)
	}

	traceParams := gin.Params{{Key: "traceId", Value: traceHex}}
	if code, _ := call(http.MethodGet, "/spans/traces/"+traceHex, "", traceParams, SpanExplorerController.GetTrace); code != http.StatusBadRequest {
		t.Fatalf("a trace read without a time must be refused, got %d", code)
	}
	code, raw := call(http.MethodGet, "/spans/traces/"+traceHex+"?at="+now.Format(time.RFC3339Nano), "", traceParams, SpanExplorerController.GetTrace)
	var whole SpanTraceResponse
	if err := json.Unmarshal(raw, &whole); err != nil || code != http.StatusOK || len(whole.Spans) != 1 || len(whole.Spans[0].Attributes) != 0 {
		t.Fatalf("whole trace, without attributes: %d %s", code, raw)
	}
	if code, _ := call(http.MethodGet, "/spans/traces/"+traceHex+"?at="+now.Add(-72*time.Hour).Format(time.RFC3339Nano), "", traceParams, SpanExplorerController.GetTrace); code != http.StatusNotFound {
		t.Fatalf("anchored three days away the trace is outside the window, got %d", code)
	}
	spanParams := gin.Params{{Key: "traceId", Value: traceHex}, {Key: "spanId", Value: span.SpanId}}
	at := "?at=" + now.Format(time.RFC3339Nano)
	if code, _ := call(http.MethodGet, "/spans/traces/x/spans/y/attributes", "", spanParams, SpanExplorerController.GetSpanAttributes); code != http.StatusBadRequest {
		t.Fatalf("span attributes need a time, got %d", code)
	}
	code, raw = call(http.MethodGet, "/spans/traces/x/spans/y/attributes"+at, "", spanParams, SpanExplorerController.GetSpanAttributes)
	var popover SpanAttributesResponse
	if err := json.Unmarshal(raw, &popover); err != nil || code != http.StatusOK || popover.Attributes["url.path"] != "/checkout" {
		t.Fatalf("the popover loads the attributes the trace read left behind: %d %s", code, raw)
	}
	unknown := gin.Params{{Key: "traceId", Value: traceHex}, {Key: "spanId", Value: "ffffffffffffffff"}}
	if code, _ := call(http.MethodGet, "/spans/traces/x/spans/y/attributes"+at, "", unknown, SpanExplorerController.GetSpanAttributes); code != http.StatusNotFound {
		t.Fatalf("unknown span, got %d", code)
	}
	malformed := gin.Params{{Key: "traceId", Value: traceHex}, {Key: "spanId", Value: "not-hex"}}
	if code, _ := call(http.MethodGet, "/spans/traces/x/spans/y/attributes"+at, "", malformed, SpanExplorerController.GetSpanAttributes); code != http.StatusBadRequest {
		t.Fatalf("malformed span id, got %d", code)
	}
	if code, _ := call(http.MethodGet, "/otel/spans/x/y", "", spanParams, GetOtelSpan); code != http.StatusBadRequest {
		t.Fatalf("the OTLP span read needs a time too, got %d", code)
	}
	if code, _ := call(http.MethodGet, "/otel/spans/x/y?at="+now.Format(time.RFC3339Nano), "", spanParams, GetOtelSpan); code != http.StatusOK {
		t.Fatalf("OTLP span with a time: %d", code)
	}
}

func TestSpanTraceReadsTheWholeOrganizationAndNothingBeyond(t *testing.T) {
	setupSetupControllerDB(t)
	tx, err := db.DB.Begin()
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()
	user, org := createSetupTestAccount(t, tx, "org-trace@example.com", "owner")
	_, otherOrg := createSetupTestAccount(t, tx, "other-org@example.com", "owner")
	_, strangerOrg := createSetupTestAccount(t, tx, "stranger-org@example.com", "owner")
	// The same person also belongs to a second organization. A trace is read inside one organization at a time.
	if _, err := transactional.OrganizationRepository.AddUser(tx, otherOrg, user, "user"); err != nil {
		t.Fatal(err)
	}
	createProject := func(name string, organization int) *models.Project {
		project, err := transactional.ProjectRepository.CreateWithOrganization(tx, name, "opentelemetry", organization)
		if err != nil {
			t.Fatal(err)
		}
		return project
	}
	gateway, payments := createProject("gateway", org), createProject("payments", org)
	sameUserOtherOrg, stranger := createProject("elsewhere", otherOrg), createProject("stranger", strangerOrg)
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}

	trace, now := uuid.New(), time.Now().UTC()
	traceHex := hex.EncodeToString(trace[:])
	rootID := "0000000000000001"
	stored := func(project *models.Project, spanId, name, parent string, offset time.Duration) models.OtelSpan {
		return models.OtelSpan{Span: models.Span{ProjectId: project.Id, TraceId: traceHex, SpanId: spanId, ParentSpanId: parent, Name: name,
			ServiceName: project.Name, SpanKind: 2, StartTime: now.Add(offset), Duration: time.Second}}
	}
	if _, err := telemetry.OtelSpanRepository.InsertAsync(context.Background(), []models.OtelSpan{
		stored(gateway, "0000000000000001", "POST /api/checkout", "", -time.Minute),
		stored(payments, "0000000000000002", "POST /charge", rootID, -59*time.Second),
		stored(sameUserOtherOrg, "0000000000000003", "another organization", rootID, -58*time.Second),
		stored(stranger, "0000000000000004", "not a member", rootID, -57*time.Second),
	}); err != nil {
		t.Fatal(err)
	}

	c, response := newControllerTestContext(t, nil, user, http.MethodGet, "/spans/traces/"+traceHex+"?at="+now.Format(time.RFC3339Nano), "")
	c.Set(middleware.ProjectIdContextKey, payments.Id)
	c.Params = gin.Params{{Key: "traceId", Value: traceHex}}
	SpanExplorerController.GetTrace(c)
	var whole SpanTraceResponse
	if err := json.Unmarshal(response.Body.Bytes(), &whole); err != nil || response.Code != http.StatusOK {
		t.Fatalf("%d %v %s", response.Code, err, response.Body.String())
	}
	if len(whole.Spans) != 2 || whole.Spans[0].Name != "POST /api/checkout" || whole.Spans[0].ProjectId != gateway.Id || whole.Spans[1].Name != "POST /charge" {
		t.Fatalf("opened from payments, the trace holds the gateway span too and nothing from outside the organization: %s", response.Body.String())
	}
	if len(whole.Projects) != 2 || whole.Projects[0].Name != "gateway" || whole.Projects[1].Name != "payments" {
		t.Fatalf("projects that hold spans are named: %+v", whole.Projects)
	}
}
