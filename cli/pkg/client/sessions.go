package client

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/google/uuid"
)

// SessionRecordingReason names the Data Collection trigger that recorded a session.
type SessionRecordingReason struct {
	TriggerId string    `json:"triggerId"`
	Name      string    `json:"name"`
	Sequence  string    `json:"sequence"`
	Revision  string    `json:"revision"`
	FiredAt   time.Time `json:"firedAt"`
}

// SessionListItem is one row of POST /api/sessions: a Session plus the triggers that recorded it.
type SessionListItem struct {
	Session
	RecordingReasons []SessionRecordingReason `json:"recordingReasons,omitempty"`
}

// ListSessionsRequest is the body for POST /api/sessions. Search matches the session id, the client IP and the
// attribute values (user id, email, ...).
type ListSessionsRequest struct {
	TimeRange     TimeRange        `json:"-"`
	Pagination    PaginationParams `json:"pagination"`
	OrderBy       string           `json:"orderBy,omitempty"`
	SortDirection string           `json:"sortDirection,omitempty"`
	Search        string           `json:"search,omitempty"`
}

// MarshalJSON expands TimeRange into top-level fromDate/toDate and sends OrderBy as the column the backend sorts on.
func (r ListSessionsRequest) MarshalJSON() ([]byte, error) {
	type alias ListSessionsRequest
	wire := struct {
		FromDate time.Time `json:"fromDate"`
		ToDate   time.Time `json:"toDate"`
		alias
	}{r.TimeRange.From, r.TimeRange.To, alias(r)}
	wire.OrderBy = orderByColumn(r.OrderBy)
	return jsonMarshalNoHTMLEscape(wire)
}

// ListSessionsResponse mirrors the upstream PaginatedResponse[Session].
type ListSessionsResponse struct {
	Data       []SessionListItem `json:"data"`
	Pagination Pagination        `json:"pagination"`
}

// ListSessions returns one page of the project's sessions that started inside the time range, newest first by
// default.
func (c *Client) ListSessions(ctx context.Context, projectID string, req ListSessionsRequest) (*ListSessionsResponse, error) {
	path := "/api/sessions?projectId=" + url.QueryEscape(projectID)
	var resp ListSessionsResponse
	if err := c.do(ctx, http.MethodPost, path, req, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// SessionStartLookback is how far back FindSessionStart looks for a session.
const SessionStartLookback = 30 * 24 * time.Hour

// ErrSessionNotFound means no session with the id started inside SessionStartLookback.
var ErrSessionNotFound = errors.New("no session with this id started in the last 30 days")

// FindSessionStart looks a session's startedAt up by its id, for a GetSession call without the timestamp: the
// session list searches ids, so one bounded list query replaces an unbounded scan of the sessions table.
func (c *Client) FindSessionStart(ctx context.Context, projectID, id string) (time.Time, error) {
	// The list searches the stored spelling, so an id written without dashes or in braces is searched as stored.
	if parsed, err := uuid.Parse(id); err == nil {
		id = parsed.String()
	}
	now := time.Now().UTC()
	resp, err := c.ListSessions(ctx, projectID, ListSessionsRequest{
		TimeRange:  TimeRange{From: now.Add(-SessionStartLookback), To: now},
		Pagination: PaginationParams{Page: 1, PageSize: 10},
		Search:     id,
	})
	if err != nil {
		return time.Time{}, err
	}
	for _, s := range resp.Data {
		if strings.EqualFold(s.Id.String(), id) {
			return s.StartedAt, nil
		}
	}
	return time.Time{}, ErrSessionNotFound
}
