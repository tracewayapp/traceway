package shared

import (
	"slices"
	"strings"
	"time"

	"github.com/google/uuid"
)

// SessionBreadcrumbLimit caps the log records and client spans one timeline read returns.
const SessionBreadcrumbLimit = 2000

// SessionTimelineSkippedEvents are the log events a session timeline leaves out: they describe the SDK itself, or
// page metrics, rather than what the user did. The repositories filter them in SQL so they never use up the limit.
var SessionTimelineSkippedEvents = []string{
	"session.start", "session.end", "traceway.session.update",
	"traceway.trigger.fired", "traceway.capture.started", "traceway.config.applied",
	"browser.web_vital", "browser.resource_timing", "browser.navigation_timing",
	"browser.long_task", "browser.route_timing",
}

// SessionTimelineSkippedEventsSQL is SessionTimelineSkippedEvents as a list of SQL string literals, for NOT IN.
var SessionTimelineSkippedEventsSQL = sqlStringList(SessionTimelineSkippedEvents)

func sqlStringList(values []string) string {
	quoted := make([]string, len(values))
	for i, value := range values {
		quoted[i] = "'" + strings.ReplaceAll(value, "'", "''") + "'"
	}
	return strings.Join(quoted, ", ")
}

// SessionBreadcrumbQuery selects the breadcrumbs of one session in a time window.
type SessionBreadcrumbQuery struct {
	ProjectId uuid.UUID
	SessionId string
	From, To  time.Time
	Limit     int
	// Newest keeps the rows nearest To when the window holds more than Limit, rather than those nearest From.
	Newest bool
}

// Order is the direction the read walks the window in, so that the LIMIT keeps the rows the query asks for.
func (q SessionBreadcrumbQuery) Order() string {
	if q.Newest {
		return "DESC"
	}
	return "ASC"
}

// SessionBreadcrumbsChronological puts rows read in the query's Order back oldest first.
func SessionBreadcrumbsChronological[T any](q SessionBreadcrumbQuery, rows []T) []T {
	if q.Newest {
		slices.Reverse(rows)
	}
	return rows
}

// SessionSpan is a request a browser session made (a CLIENT span), with the attributes its timeline row shows.
type SessionSpan struct {
	TraceId    string
	SpanId     string
	Name       string
	StartTime  time.Time
	Duration   time.Duration
	StatusCode int32
	Method     string
	URL        string
	HTTPStatus string
	ErrorType  string
}
