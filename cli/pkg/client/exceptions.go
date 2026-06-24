package client

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"time"

	"github.com/google/uuid"
)

// ExceptionGroup matches Traceway's models.ExceptionGroup. Hourly trends are
// only present on list responses; on the detail endpoint they're absent.
type ExceptionGroup struct {
	ExceptionHash string                `json:"exceptionHash"`
	StackTrace    string                `json:"stackTrace"`
	FirstSeen     time.Time             `json:"firstSeen"`
	LastSeen      time.Time             `json:"lastSeen"`
	Count         uint64                `json:"count"`
	HourlyTrend   []ExceptionTrendPoint `json:"hourlyTrend,omitempty"`
}

// ExceptionTrendPoint is one entry in an exception's hourly trend.
type ExceptionTrendPoint struct {
	Timestamp time.Time `json:"timestamp"`
	Count     uint64    `json:"count"`
}

// ListExceptionsRequest is the body for POST /api/exception-stack-traces.
// projectId travels as a URL query param (handled by ListExceptions), not in
// the body — the upstream RequireProjectAccess middleware reads it via
// c.Query("projectId").
type ListExceptionsRequest struct {
	TimeRange       TimeRange        `json:"-"` // serialized as fromDate/toDate via MarshalJSON
	Pagination      PaginationParams `json:"pagination"`
	OrderBy         string           `json:"orderBy,omitempty"`
	Search          string           `json:"search,omitempty"`
	SearchType      string           `json:"searchType,omitempty"`
	IncludeArchived bool             `json:"includeArchived,omitempty"`
}

// MarshalJSON expands TimeRange.From / TimeRange.To into top-level fromDate /
// toDate so the wire shape matches Traceway's ExceptionSearchRequest.
func (r ListExceptionsRequest) MarshalJSON() ([]byte, error) {
	type alias ListExceptionsRequest
	wire := struct {
		FromDate time.Time `json:"fromDate"`
		ToDate   time.Time `json:"toDate"`
		alias
	}{
		FromDate: r.TimeRange.From,
		ToDate:   r.TimeRange.To,
		alias:    alias(r),
	}
	return jsonMarshalNoHTMLEscape(wire)
}

// ListExceptionsResponse mirrors the upstream PaginatedResponse[ExceptionGroup].
type ListExceptionsResponse struct {
	Data       []ExceptionGroup `json:"data"`
	Pagination Pagination       `json:"pagination"`
}

// ListExceptions returns one page of grouped exceptions for the given project.
func (c *Client) ListExceptions(ctx context.Context, projectID string, req ListExceptionsRequest) (*ListExceptionsResponse, error) {
	path := "/api/exception-stack-traces?projectId=" + url.QueryEscape(projectID)
	var resp ListExceptionsResponse
	if err := c.do(ctx, http.MethodPost, path, req, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// ExceptionStackTrace is one occurrence of a grouped exception.
type ExceptionStackTrace struct {
	Id                 uuid.UUID         `json:"id"`
	ExceptionHash      string            `json:"exceptionHash"`
	StackTrace         string            `json:"stackTrace"`
	RecordedAt         time.Time         `json:"recordedAt"`
	TraceId            *uuid.UUID        `json:"traceId,omitempty"`
	TraceType          string            `json:"traceType,omitempty"`
	ServerName         string            `json:"serverName,omitempty"`
	AppVersion         string            `json:"appVersion,omitempty"`
	IsMessage          bool              `json:"isMessage,omitempty"`
	Attributes         map[string]string `json:"attributes,omitempty"`
	DistributedTraceId *uuid.UUID        `json:"distributedTraceId,omitempty"`
	SessionId          *uuid.UUID        `json:"sessionId,omitempty"`
}

// getExceptionRequest is the body for POST /api/exception-stack-traces/:hash.
type getExceptionRequest struct {
	Pagination PaginationParams `json:"pagination"`
}

// GetExceptionResponse is the upstream ExceptionDetailResponse minus the
// session-recording blob (we don't expose recordings in v1).
type GetExceptionResponse struct {
	Group       *ExceptionGroup       `json:"group"`
	Occurrences []ExceptionStackTrace `json:"occurrences"`
	Pagination  Pagination            `json:"pagination"`
}

// GetException returns the group + paginated occurrences for the given hash.
func (c *Client) GetException(ctx context.Context, projectID, hash string, page PaginationParams) (*GetExceptionResponse, error) {
	path := "/api/exception-stack-traces/" + url.PathEscape(hash) + "?projectId=" + url.QueryEscape(projectID)
	var resp GetExceptionResponse
	if err := c.do(ctx, http.MethodPost, path, getExceptionRequest{Pagination: page}, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// ExceptionByIdResponse is the body of POST
// /api/exception-stack-traces/by-id/:exceptionId. Over a paginated `exceptions
// show <hash>` occurrence it adds the linked sessionId and an inline
// sessionRecording blob (passed through verbatim), and resolves directly
// without walking pages.
type ExceptionByIdResponse struct {
	Exception        *ExceptionStackTrace `json:"exception"`
	SessionId        *uuid.UUID           `json:"sessionId,omitempty"`
	SessionRecording json.RawMessage      `json:"sessionRecording,omitempty"`
}

// GetExceptionById returns a single exception occurrence by its id. recordedAt
// is the occurrence's recordedAt (from the URL's t= param, a notification's
// "Occurred at", or an `exceptions show` occurrence) and is required for a
// partition-pruned lookup.
func (c *Client) GetExceptionById(ctx context.Context, projectID, id string, recordedAt time.Time) (*ExceptionByIdResponse, error) {
	path := "/api/exception-stack-traces/by-id/" + url.PathEscape(id) + "?projectId=" + url.QueryEscape(projectID)
	var resp ExceptionByIdResponse
	if err := c.do(ctx, http.MethodPost, path, recordedAtBody{RecordedAt: recordedAt}, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// archiveRequest is the body for POST /api/exception-stack-traces/archive
// and .../unarchive. Same shape for both routes.
type archiveRequest struct {
	Hashes []string `json:"hashes"`
}

// ArchiveExceptions marks the given exception hashes as archived for the
// project. The upstream response is just {"success": true}; on success this
// returns nil and the caller can report the count back to the user.
func (c *Client) ArchiveExceptions(ctx context.Context, projectID string, hashes []string) error {
	path := "/api/exception-stack-traces/archive?projectId=" + url.QueryEscape(projectID)
	return c.do(ctx, http.MethodPost, path, archiveRequest{Hashes: hashes}, nil)
}

// UnarchiveExceptions reverses ArchiveExceptions for the given hashes.
func (c *Client) UnarchiveExceptions(ctx context.Context, projectID string, hashes []string) error {
	path := "/api/exception-stack-traces/unarchive?projectId=" + url.QueryEscape(projectID)
	return c.do(ctx, http.MethodPost, path, archiveRequest{Hashes: hashes}, nil)
}
