package client

import (
	"context"
	"net/http"
	"net/url"
	"time"
)

// TaskStats matches the upstream models.TaskStats — aggregates for one task
// name. Durations are time.Duration values (nanoseconds on the wire).
type TaskStats struct {
	TaskName    string        `json:"taskName"`
	Count       uint64        `json:"count"`
	P50Duration time.Duration `json:"p50Duration"`
	P95Duration time.Duration `json:"p95Duration"`
	AvgDuration time.Duration `json:"avgDuration"`
	LastSeen    time.Time     `json:"lastSeen"`
	HasRoot     bool          `json:"hasRoot"`
	HasNonRoot  bool          `json:"hasNonRoot"`
}

// ListTasksRequest is the body for POST /api/tasks/grouped. OrderBy uses the
// server's snake_case field names (count, p50_duration, p95_duration,
// avg_duration, last_seen, impact); RootFilter is "root", "non_root", or ""
// for all.
type ListTasksRequest struct {
	TimeRange     TimeRange        `json:"-"`
	Pagination    PaginationParams `json:"pagination"`
	OrderBy       string           `json:"orderBy,omitempty"`
	SortDirection string           `json:"sortDirection,omitempty"`
	Search        string           `json:"search,omitempty"`
	RootFilter    string           `json:"rootFilter,omitempty"`
}

// MarshalJSON expands TimeRange into top-level fromDate/toDate.
func (r ListTasksRequest) MarshalJSON() ([]byte, error) {
	type alias ListTasksRequest
	wire := struct {
		FromDate time.Time `json:"fromDate"`
		ToDate   time.Time `json:"toDate"`
		alias
	}{r.TimeRange.From, r.TimeRange.To, alias(r)}
	return jsonMarshalNoHTMLEscape(wire)
}

// ListTasksResponse mirrors PaginatedResponse[TaskStats].
type ListTasksResponse struct {
	Data       []TaskStats `json:"data"`
	Pagination Pagination  `json:"pagination"`
}

// ListTasks returns duration stats grouped by task name, the analog of
// ListEndpoints for background tasks.
func (c *Client) ListTasks(ctx context.Context, projectID string, req ListTasksRequest) (*ListTasksResponse, error) {
	path := "/api/tasks/grouped?projectId=" + url.QueryEscape(projectID)
	var resp ListTasksResponse
	if err := c.do(ctx, http.MethodPost, path, req, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// TaskDetailStats matches the upstream models.TaskDetailStats — aggregate
// stats for one task name. Unlike the Duration fields elsewhere these are
// float64 milliseconds, and Throughput is runs per minute.
type TaskDetailStats struct {
	Count          int64   `json:"count"`
	AvgDuration    float64 `json:"avgDuration"`
	MedianDuration float64 `json:"medianDuration"`
	P95Duration    float64 `json:"p95Duration"`
	P99Duration    float64 `json:"p99Duration"`
	Throughput     float64 `json:"throughput"`
}

// ListTaskRunsRequest is the body for POST /api/tasks (runs across all tasks)
// and POST /api/tasks/task (one task's runs). OrderBy is "recorded_at" or
// "duration". The all-tasks endpoint ignores SortDirection (always DESC).
type ListTaskRunsRequest struct {
	TimeRange     TimeRange        `json:"-"`
	Pagination    PaginationParams `json:"pagination"`
	OrderBy       string           `json:"orderBy,omitempty"`
	SortDirection string           `json:"sortDirection,omitempty"`
}

// MarshalJSON expands TimeRange into top-level fromDate/toDate.
func (r ListTaskRunsRequest) MarshalJSON() ([]byte, error) {
	type alias ListTaskRunsRequest
	wire := struct {
		FromDate time.Time `json:"fromDate"`
		ToDate   time.Time `json:"toDate"`
		alias
	}{r.TimeRange.From, r.TimeRange.To, alias(r)}
	return jsonMarshalNoHTMLEscape(wire)
}

// ListTaskRunsResponse holds individual task runs. Stats is only present when
// the request was scoped to one task name.
type ListTaskRunsResponse struct {
	Data       []Task           `json:"data"`
	Stats      *TaskDetailStats `json:"stats,omitempty"`
	Pagination Pagination       `json:"pagination"`
}

// ListTaskRuns returns individual task runs. With taskName empty it lists runs
// across all tasks (POST /api/tasks); with a task name it lists that task's
// runs plus aggregate stats (POST /api/tasks/task). Each run's id + recordedAt
// feed the by-id GetTask lookup.
func (c *Client) ListTaskRuns(ctx context.Context, projectID, taskName string, req ListTaskRunsRequest) (*ListTaskRunsResponse, error) {
	path := "/api/tasks?projectId=" + url.QueryEscape(projectID)
	if taskName != "" {
		path = "/api/tasks/task?projectId=" + url.QueryEscape(projectID) + "&task=" + url.QueryEscape(taskName)
	}
	var resp ListTaskRunsResponse
	if err := c.do(ctx, http.MethodPost, path, req, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}
