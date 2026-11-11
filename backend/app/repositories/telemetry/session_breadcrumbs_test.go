package telemetry

import (
	"context"
	"encoding/hex"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/tracewayapp/traceway/backend/app/models"
)

func TestSessionBreadcrumbsAreReadBySessionId(t *testing.T) {
	setupTestDB(t)
	ctx := context.Background()
	projectId := uuid.New()
	session := uuid.New().String()
	other := uuid.New().String()
	now := truncateMs(time.Now().UTC())

	withSession := func(r models.LogRecord, id string) models.LogRecord {
		r.LogAttributes = map[string]string{"session.id": id, "event.name": "browser.console"}
		return r
	}
	records := []models.LogRecord{
		withSession(makeLogRecord(projectId, 9, "second", now.Add(2*time.Second)), session),
		withSession(makeLogRecord(projectId, 17, "first", now.Add(time.Second)), session),
		withSession(makeLogRecord(projectId, 9, "other session", now.Add(time.Second)), other),
		withSession(makeLogRecord(projectId, 9, "outside the window", now.Add(2*time.Hour)), session),
		withSession(makeLogRecord(uuid.New(), 9, "other project", now.Add(time.Second)), session),
		makeLogRecord(projectId, 9, "no session", now.Add(time.Second)),
	}
	if err := LogRecordRepository.InsertAsync(ctx, records); err != nil {
		t.Fatalf("InsertAsync failed: %v", err)
	}
	found, err := LogRecordRepository.FindBySessionId(ctx, projectId, session, now.Add(-time.Minute), now.Add(time.Minute), 100)
	if err != nil {
		t.Fatalf("FindBySessionId failed: %v", err)
	}
	if len(found) != 2 || found[0].Body != "first" || found[1].Body != "second" {
		t.Fatalf("FindBySessionId = %+v, want first and second in time order", found)
	}
	if found[0].LogAttributes["event.name"] != "browser.console" {
		t.Errorf("attributes were not read back: %v", found[0].LogAttributes)
	}
	if limited, err := LogRecordRepository.FindBySessionId(ctx, projectId, session, now.Add(-time.Minute), now.Add(time.Minute), 1); err != nil || len(limited) != 1 {
		t.Errorf("limit 1 = %d rows, %v", len(limited), err)
	}

	trace := uuid.New()
	span := func(kind int32, sessionId string, start time.Time, attrs map[string]string) models.OtelSpan {
		s := canonicalSpan(projectId, trace, uuid.New(), nil)
		s.SpanKind = kind
		s.StartTime, s.RecordedAt, s.Duration = start, start, 250*time.Millisecond
		s.Attributes = map[string]string{"session.id": sessionId}
		for k, v := range attrs {
			s.Attributes[k] = v
		}
		return s
	}
	request := span(3, session, now.Add(time.Second), map[string]string{
		"http.request.method": "GET", "url.full": "https://shop.test/api/cart", "http.response.status_code": "404",
	})
	failed := span(3, session, now.Add(500*time.Millisecond), map[string]string{"http.request.method": "POST", "error.type": "TypeError"})
	spans := []models.OtelSpan{request, failed,
		span(1, session, now.Add(time.Second), nil),
		span(3, other, now.Add(time.Second), nil),
	}
	if _, err := OtelSpanRepository.InsertAsync(ctx, spans); err != nil {
		t.Fatalf("span InsertAsync failed: %v", err)
	}
	client, err := OtelSpanRepository.FindSessionClientSpans(ctx, projectId, session, now.Add(-time.Minute), now.Add(time.Minute), 100)
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
