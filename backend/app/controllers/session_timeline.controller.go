package controllers

import (
	"encoding/json"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/tracewayapp/traceway/backend/v2/app/middleware"
	"github.com/tracewayapp/traceway/backend/v2/app/models"
	"github.com/tracewayapp/traceway/backend/v2/app/repositories/telemetry"
	"github.com/tracewayapp/traceway/backend/v2/app/repositories/telemetry/shared"
	"github.com/tracewayapp/traceway/backend/v2/app/traceway"
)

// A session lasts at most about an hour (shared.SessionMaxSpan); the window leaves room for clock skew.
const maxTimelineWindow = 3 * time.Hour

type sessionTimelineController struct{}

var SessionTimelineController = sessionTimelineController{}

type timelineLog struct {
	Type      string    `json:"type"`
	Timestamp time.Time `json:"timestamp"`
	Level     string    `json:"level"`
	Message   string    `json:"message"`
}

type timelineAction struct {
	Type       string         `json:"type"`
	Timestamp  time.Time      `json:"timestamp"`
	Method     string         `json:"method,omitempty"`
	URL        string         `json:"url,omitempty"`
	DurationMs *int64         `json:"durationMs,omitempty"`
	StatusCode *int           `json:"statusCode,omitempty"`
	Error      string         `json:"error,omitempty"`
	TraceId    string         `json:"traceId,omitempty"`
	SpanId     string         `json:"spanId,omitempty"`
	Action     string         `json:"action,omitempty"`
	From       string         `json:"from,omitempty"`
	To         string         `json:"to,omitempty"`
	Category   string         `json:"category,omitempty"`
	Name       string         `json:"name,omitempty"`
	Data       map[string]any `json:"data,omitempty"`
}

type SessionTimeline struct {
	Logs      []timelineLog    `json:"logs"`
	Actions   []timelineAction `json:"actions"`
	Truncated bool             `json:"truncated"`
}

var timelineSkippedEvents = func() map[string]bool {
	skipped := map[string]bool{}
	for _, event := range shared.SessionTimelineSkippedEvents {
		skipped[event] = true
	}
	return skipped
}()

// Attributes the SDK stamps on every app event; they are context, not the event's own data.
var timelineContextKeys = map[string]bool{
	"session.id": true, "event.name": true, "url": true, "path": true, "referrer": true, "userAgent": true,
	"language": true, "platform": true, "viewport": true, "screen": true, "timezone": true,
}

// Timeline answers the breadcrumbs of a browser session in a time window: its console lines, navigation, clicks
// and app events from the log records, and its requests from the CLIENT spans. The OpenTelemetry SDK sends these as
// telemetry only, so this is their one copy; recordings from older SDKs still carry their own logs and actions.
func (sessionTimelineController) Timeline(c *gin.Context) {
	projectId, err := middleware.GetProjectId(c)
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("RequireProjectAccess middleware must be applied: %w", err))
		return
	}
	sessionId, err := uuid.Parse(c.Param("sessionId"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid session id"})
		return
	}
	from, fromErr := time.Parse(time.RFC3339Nano, c.Query("from"))
	to, toErr := time.Parse(time.RFC3339Nano, c.Query("to"))
	if fromErr != nil || toErr != nil || to.Before(from) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "from and to must be RFC 3339 times, from before to"})
		return
	}
	if to.Sub(from) > maxTimelineWindow {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": "the timeline window can span at most 3 hours"})
		return
	}

	// anchor=end keeps the breadcrumbs nearest `to` when the window holds more than the limit, which is what a view of
	// the moments before an exception needs; the default keeps those nearest `from`.
	anchor := c.DefaultQuery("anchor", "start")
	if anchor != "start" && anchor != "end" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "anchor must be start or end"})
		return
	}

	query := shared.SessionBreadcrumbQuery{ProjectId: projectId, SessionId: sessionId.String(), From: from, To: to,
		Limit: shared.SessionBreadcrumbLimit, Newest: anchor == "end"}
	records, err := telemetry.LogRecordRepository.FindBySessionId(c, query)
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("loading session log records: %w", err))
		return
	}
	spans, err := telemetry.OtelSpanRepository.FindSessionClientSpans(c, query)
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("loading session client spans: %w", err))
		return
	}
	records, spans, truncated := trimToCoveredWindow(query, records, spans)
	timeline := BuildSessionTimeline(records, spans)
	timeline.Truncated = truncated
	c.JSON(http.StatusOK, timeline)
}

// trimToCoveredWindow cuts both reads back to the part of the window that each of them returned in full. When one
// read stops at the limit, rows of the other beyond that point would sit next to a silent stretch of the first.
func trimToCoveredWindow(q shared.SessionBreadcrumbQuery, records []models.LogRecord, spans []shared.SessionSpan) ([]models.LogRecord, []shared.SessionSpan, bool) {
	recordsFull, spansFull := len(records) >= q.Limit, len(spans) >= q.Limit
	if !recordsFull && !spansFull {
		return records, spans, false
	}
	var bounds []time.Time
	if recordsFull {
		bounds = append(bounds, records[lastRead(q, len(records))].Timestamp)
	}
	if spansFull {
		bounds = append(bounds, spans[lastRead(q, len(spans))].StartTime)
	}
	if q.Newest {
		bound := slices.MaxFunc(bounds, time.Time.Compare)
		records = slices.DeleteFunc(records, func(r models.LogRecord) bool { return r.Timestamp.Before(bound) })
		spans = slices.DeleteFunc(spans, func(s shared.SessionSpan) bool { return s.StartTime.Before(bound) })
	} else {
		bound := slices.MinFunc(bounds, time.Time.Compare)
		records = slices.DeleteFunc(records, func(r models.LogRecord) bool { return r.Timestamp.After(bound) })
		spans = slices.DeleteFunc(spans, func(s shared.SessionSpan) bool { return s.StartTime.After(bound) })
	}
	return records, spans, true
}

// lastRead is the index of the row a read reached last; reads come back oldest first whichever way they walked.
func lastRead(q shared.SessionBreadcrumbQuery, n int) int {
	if q.Newest {
		return 0
	}
	return n - 1
}

// BuildSessionTimeline maps log records and client spans onto the log and action rows the replay player shows.
func BuildSessionTimeline(records []models.LogRecord, spans []shared.SessionSpan) SessionTimeline {
	timeline := SessionTimeline{Logs: []timelineLog{}, Actions: []timelineAction{}}
	lastURL := ""
	for _, record := range records {
		attrs := record.LogAttributes
		event := attrs["event.name"]
		if timelineSkippedEvents[event] {
			continue
		}
		switch event {
		case "", "browser.console":
			timeline.Logs = append(timeline.Logs, timelineLog{Type: "log", Timestamp: record.Timestamp, Level: timelineLevel(record.SeverityNumber), Message: record.Body})
		case "exception":
			message := strings.TrimSpace(attrs["exception.type"] + ": " + attrs["exception.message"])
			timeline.Logs = append(timeline.Logs, timelineLog{Type: "log", Timestamp: record.Timestamp, Level: "error", Message: strings.TrimPrefix(message, ": ")})
		case "traceway.message":
			timeline.Logs = append(timeline.Logs, timelineLog{Type: "log", Timestamp: record.Timestamp, Level: "info", Message: record.Body})
		case "browser.navigation":
			action := navigationAction(attrs)
			to := attrs["url.full"]
			timeline.Actions = append(timeline.Actions, timelineAction{Type: "navigation", Timestamp: record.Timestamp, Action: action, From: lastURL, To: to})
			lastURL = to
		case "browser.user_action.click":
			data := map[string]any{}
			for key, attr := range map[string]string{"selector": "browser.css_selector", "tag": "browser.tag_name"} {
				if v := attrs[attr]; v != "" {
					data[key] = v
				}
			}
			timeline.Actions = append(timeline.Actions, timelineAction{Type: "custom", Timestamp: record.Timestamp, Category: "ui", Name: "click", Data: data})
		case "traceway.action":
			var data map[string]any
			if raw := attrs["traceway.action.data"]; raw != "" {
				_ = json.Unmarshal([]byte(raw), &data)
			}
			timeline.Actions = append(timeline.Actions, timelineAction{Type: "custom", Timestamp: record.Timestamp,
				Category: attrs["traceway.action.category"], Name: attrs["traceway.action.name"], Data: data})
		default:
			data := map[string]any{}
			for key, value := range attrs {
				if !timelineContextKeys[key] && len(data) < 20 {
					data[key] = value
				}
			}
			timeline.Actions = append(timeline.Actions, timelineAction{Type: "custom", Timestamp: record.Timestamp, Category: "event", Name: event, Data: data})
		}
	}

	seen := map[string]bool{}
	for _, span := range spans {
		if seen[span.SpanId] {
			continue
		}
		seen[span.SpanId] = true
		method := span.Method
		if method == "" {
			method = span.Name
		}
		duration := span.Duration.Milliseconds()
		action := timelineAction{Type: "network", Timestamp: span.StartTime, Method: method, URL: span.URL, DurationMs: &duration,
			TraceId: span.TraceId, SpanId: span.SpanId}
		if status, err := strconv.Atoi(span.HTTPStatus); err == nil {
			action.StatusCode = &status
		}
		if span.ErrorType != "" {
			action.Error = span.ErrorType
		} else if span.StatusCode == 2 && action.StatusCode == nil {
			action.Error = "failed"
		}
		timeline.Actions = append(timeline.Actions, action)
	}
	slices.SortStableFunc(timeline.Actions, func(a, b timelineAction) int { return a.Timestamp.Compare(b.Timestamp) })
	return timeline
}

// navigationAction names a navigation the way the replay player does. The instrumentation only sets
// browser.navigation.type when the browser's Navigation API reports one, so a full page load has none.
func navigationAction(attrs map[string]string) string {
	switch {
	case attrs["browser.navigation.type"] != "":
		return attrs["browser.navigation.type"]
	case attrs["browser.navigation.hash_change"] == "true":
		return "hash"
	case attrs["browser.navigation.same_document"] == "true":
		return "push"
	}
	return "load"
}

func timelineLevel(severity uint8) string {
	switch {
	case severity >= 17:
		return "error"
	case severity >= 13:
		return "warn"
	case severity > 0 && severity < 9:
		return "debug"
	}
	return "info"
}
