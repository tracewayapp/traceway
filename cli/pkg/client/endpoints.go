package client

import (
	"context"
	"net/http"
	"net/url"
	"time"

	"github.com/google/uuid"
)

// EndpointStats matches the upstream models.EndpointStats. Durations are
// time.Duration values which Go marshals/unmarshals as nanoseconds.
type EndpointStats struct {
	Endpoint     string        `json:"endpoint"`
	Count        uint64        `json:"count"`
	P50Duration  time.Duration `json:"p50Duration"`
	P95Duration  time.Duration `json:"p95Duration"`
	P99Duration  time.Duration `json:"p99Duration"`
	AvgDuration  time.Duration `json:"avgDuration"`
	LastSeen     time.Time     `json:"lastSeen"`
	Impact       float64       `json:"impact"`
	ImpactReason string        `json:"impactReason"`
}

// ListEndpointsRequest is the body for POST /api/endpoints/grouped.
type ListEndpointsRequest struct {
	TimeRange     TimeRange        `json:"-"`
	Pagination    PaginationParams `json:"pagination"`
	OrderBy       string           `json:"orderBy,omitempty"`
	SortDirection string           `json:"sortDirection,omitempty"`
	Search        string           `json:"search,omitempty"`
}

// MarshalJSON expands TimeRange into top-level fromDate/toDate.
func (r ListEndpointsRequest) MarshalJSON() ([]byte, error) {
	type alias ListEndpointsRequest
	wire := struct {
		FromDate time.Time `json:"fromDate"`
		ToDate   time.Time `json:"toDate"`
		alias
	}{r.TimeRange.From, r.TimeRange.To, alias(r)}
	return jsonMarshalNoHTMLEscape(wire)
}

// ListEndpointsResponse mirrors PaginatedResponse[EndpointStats].
type ListEndpointsResponse struct {
	Data       []EndpointStats `json:"data"`
	Pagination Pagination      `json:"pagination"`
}

// ListEndpoints returns p50/p95/p99 stats grouped by endpoint route. We use
// the /grouped variant rather than the bare /endpoints (which returns one row
// per request).
func (c *Client) ListEndpoints(ctx context.Context, projectID string, req ListEndpointsRequest) (*ListEndpointsResponse, error) {
	path := "/api/endpoints/grouped?projectId=" + url.QueryEscape(projectID)
	var resp ListEndpointsResponse
	if err := c.do(ctx, http.MethodPost, path, req, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// Endpoint mirrors models.Endpoint — one request (transaction), as returned by
// the by-id detail endpoint. EndpointStats (above) is the grouped/list shape;
// this is the single-row shape keyed by the transaction's id.
type Endpoint struct {
	Id                 uuid.UUID         `json:"id"`
	ProjectId          uuid.UUID         `json:"projectId"`
	Endpoint           string            `json:"endpoint"`
	Duration           time.Duration     `json:"duration"`
	RecordedAt         time.Time         `json:"recordedAt"`
	StatusCode         int16             `json:"statusCode"`
	BodySize           int32             `json:"bodySize"`
	ClientIP           string            `json:"clientIP"`
	Attributes         map[string]string `json:"attributes"`
	AppVersion         string            `json:"appVersion"`
	ServerName         string            `json:"serverName"`
	DistributedTraceId *uuid.UUID        `json:"distributedTraceId,omitempty"`
	SpanId             *uuid.UUID        `json:"spanId,omitempty"`
	IsStream           bool              `json:"isStream"`
	IsRoot             bool              `json:"isRoot"`
}

// EndpointDetailResponse is the body of POST /api/endpoints/:endpointId.
type EndpointDetailResponse struct {
	Endpoint  *Endpoint        `json:"endpoint"`
	Spans     []Span           `json:"spans"`
	HasSpans  bool             `json:"hasSpans"`
	Exception *LinkedException `json:"exception,omitempty"`
	Messages  []LinkedMessage  `json:"messages"`
}

// GetEndpoint returns one request (transaction) by id plus its spans and any
// linked exception/messages. recordedAt is the transaction's recordedAt and is
// required for a partition-pruned lookup; without it the server scans every
// daily partition of the endpoints table.
func (c *Client) GetEndpoint(ctx context.Context, projectID, id string, recordedAt time.Time) (*EndpointDetailResponse, error) {
	path := "/api/endpoints/" + url.PathEscape(id) + "?projectId=" + url.QueryEscape(projectID)
	var resp EndpointDetailResponse
	if err := c.do(ctx, http.MethodPost, path, recordedAtBody{RecordedAt: recordedAt}, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}
