package shared

import "time"

// SessionBreadcrumbLimit caps the log records and client spans one timeline read returns.
const SessionBreadcrumbLimit = 2000

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
