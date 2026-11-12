package telemetry

import (
	"context"
	"encoding/hex"
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/tracewayapp/traceway/backend/v2/app/models"
	"github.com/tracewayapp/traceway/backend/v2/app/repositories/telemetry/shared"
)

func breadcrumbQuery(projectId uuid.UUID, session string, from, to time.Time, limit int) shared.SessionBreadcrumbQuery {
	return shared.SessionBreadcrumbQuery{ProjectId: projectId, SessionId: session, From: from, To: to, Limit: limit}
}

func sessionLog(projectId uuid.UUID, session, event, body string, at time.Time) models.LogRecord {
	r := makeLogRecord(projectId, 9, body, at)
	r.LogAttributes = map[string]string{"session.id": session}
	if event != "" {
		r.LogAttributes["event.name"] = event
	}
	return r
}

func sessionClientSpan(projectId, trace uuid.UUID, kind int32, sessionId string, start time.Time, attrs map[string]string) models.OtelSpan {
	s := canonicalSpan(projectId, trace, uuid.New(), nil)
	s.SpanKind = kind
	s.StartTime, s.RecordedAt, s.Duration = start, start, 250*time.Millisecond
	s.Attributes = map[string]string{"session.id": sessionId}
	for k, v := range attrs {
		s.Attributes[k] = v
	}
	return s
}

func TestSessionBreadcrumbsAreReadBySessionId(t *testing.T) {
	setupTestDB(t)
	ctx := context.Background()
	projectId := uuid.New()
	session := uuid.New().String()
	other := uuid.New().String()
	now := truncateMs(time.Now().UTC())

	records := []models.LogRecord{
		sessionLog(projectId, session, "browser.console", "second", now.Add(2*time.Second)),
		sessionLog(projectId, session, "browser.console", "first", now.Add(time.Second)),
		sessionLog(projectId, other, "browser.console", "other session", now.Add(time.Second)),
		sessionLog(projectId, session, "browser.console", "outside the window", now.Add(2*time.Hour)),
		sessionLog(uuid.New(), session, "browser.console", "other project", now.Add(time.Second)),
		makeLogRecord(projectId, 9, "no session", now.Add(time.Second)),
	}
	records[1].SeverityNumber = 17
	if err := LogRecordRepository.InsertAsync(ctx, records); err != nil {
		t.Fatalf("InsertAsync failed: %v", err)
	}
	window := breadcrumbQuery(projectId, session, now.Add(-time.Minute), now.Add(time.Minute), 100)
	found, err := LogRecordRepository.FindBySessionId(ctx, window)
	if err != nil {
		t.Fatalf("FindBySessionId failed: %v", err)
	}
	if len(found) != 2 || found[0].Body != "first" || found[1].Body != "second" {
		t.Fatalf("FindBySessionId = %+v, want first and second in time order", found)
	}
	if found[0].LogAttributes["event.name"] != "browser.console" {
		t.Errorf("attributes were not read back: %v", found[0].LogAttributes)
	}
	window.Limit = 1
	if limited, err := LogRecordRepository.FindBySessionId(ctx, window); err != nil || len(limited) != 1 {
		t.Errorf("limit 1 = %d rows, %v", len(limited), err)
	}

	trace := uuid.New()
	request := sessionClientSpan(projectId, trace, 3, session, now.Add(time.Second), map[string]string{
		"http.request.method": "GET", "url.full": "https://shop.test/api/cart", "http.response.status_code": "404",
	})
	failed := sessionClientSpan(projectId, trace, 3, session, now.Add(500*time.Millisecond), map[string]string{"http.request.method": "POST", "error.type": "TypeError"})
	spans := []models.OtelSpan{request, failed,
		sessionClientSpan(projectId, trace, 1, session, now.Add(time.Second), nil),
		sessionClientSpan(projectId, trace, 3, other, now.Add(time.Second), nil),
	}
	if _, err := OtelSpanRepository.InsertAsync(ctx, spans); err != nil {
		t.Fatalf("span InsertAsync failed: %v", err)
	}
	window.Limit = 100
	client, err := OtelSpanRepository.FindSessionClientSpans(ctx, window)
	if err != nil {
		t.Fatalf("FindSessionClientSpans failed: %v", err)
	}
	if len(client) != 2 || client[0].SpanId != failed.SpanId || client[1].SpanId != request.SpanId {
		t.Fatalf("FindSessionClientSpans = %+v, want the two client spans of the session in start order", client)
	}
	got := client[1]
	if got.Method != "GET" || got.URL != "https://shop.test/api/cart" || got.HTTPStatus != "404" ||
		got.TraceId != hex.EncodeToString(trace[:]) || got.Duration != 250*time.Millisecond || !got.StartTime.Equal(request.StartTime) {
		t.Errorf("client span = %+v", got)
	}
	if client[0].ErrorType != "TypeError" {
		t.Errorf("error.type = %q", client[0].ErrorType)
	}
}

// The issue page asks for the minutes before an exception, so a read anchored at the end of the window must keep
// the rows nearest it, and the events a timeline never shows must not use up the limit.
func TestSessionBreadcrumbsNewestFirstAndSkippedEvents(t *testing.T) {
	setupTestDB(t)
	ctx := context.Background()
	projectId := uuid.New()
	session := uuid.New().String()
	now := truncateMs(time.Now().UTC())

	var records []models.LogRecord
	var spans []models.OtelSpan
	trace := uuid.New()
	for i := range 5 {
		at := now.Add(time.Duration(i) * time.Second)
		records = append(records, sessionLog(projectId, session, "browser.console", fmt.Sprintf("line %d", i), at))
		spans = append(spans, sessionClientSpan(projectId, trace, 3, session, at, map[string]string{"url.full": fmt.Sprintf("/api/%d", i)}))
	}
	records = append(records, sessionLog(projectId, session, "", "plain log", now.Add(4500*time.Millisecond)))
	for _, event := range shared.SessionTimelineSkippedEvents {
		records = append(records, sessionLog(projectId, session, event, "", now.Add(4800*time.Millisecond)))
	}
	if err := LogRecordRepository.InsertAsync(ctx, records); err != nil {
		t.Fatal(err)
	}
	if _, err := OtelSpanRepository.InsertAsync(ctx, spans); err != nil {
		t.Fatal(err)
	}

	window := breadcrumbQuery(projectId, session, now.Add(-time.Minute), now.Add(time.Minute), 100)
	all, err := LogRecordRepository.FindBySessionId(ctx, window)
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != 6 {
		t.Fatalf("got %d records, want the 5 console lines and the plain log without the skipped events: %+v", len(all), all)
	}

	window.Limit = 3
	oldest, err := LogRecordRepository.FindBySessionId(ctx, window)
	if err != nil {
		t.Fatal(err)
	}
	if bodies(oldest) != "line 0,line 1,line 2" {
		t.Errorf("oldest = %s", bodies(oldest))
	}
	window.Newest = true
	newest, err := LogRecordRepository.FindBySessionId(ctx, window)
	if err != nil {
		t.Fatal(err)
	}
	if bodies(newest) != "line 3,line 4,plain log" {
		t.Errorf("newest = %s, want the last three in time order", bodies(newest))
	}

	window.Limit = 2
	newestSpans, err := OtelSpanRepository.FindSessionClientSpans(ctx, window)
	if err != nil {
		t.Fatal(err)
	}
	if len(newestSpans) != 2 || newestSpans[0].URL != "/api/3" || newestSpans[1].URL != "/api/4" {
		t.Errorf("newest spans = %+v, want /api/3 then /api/4", newestSpans)
	}
	window.Newest = false
	oldestSpans, err := OtelSpanRepository.FindSessionClientSpans(ctx, window)
	if err != nil {
		t.Fatal(err)
	}
	if len(oldestSpans) != 2 || oldestSpans[0].URL != "/api/0" || oldestSpans[1].URL != "/api/1" {
		t.Errorf("oldest spans = %+v, want /api/0 then /api/1", oldestSpans)
	}
}

func bodies(records []models.LogRecord) string {
	out := ""
	for i, r := range records {
		if i > 0 {
			out += ","
		}
		out += r.Body
	}
	return out
}
