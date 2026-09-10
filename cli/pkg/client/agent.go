package client

import (
	"context"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

// Attempt is one fix-agent run on an issue as the backend reports it.
type Attempt struct {
	ID          string     `json:"id"`
	Number      int        `json:"number"`
	SubjectKind string     `json:"subjectKind"`
	SubjectRef  string     `json:"subjectRef"`
	Status      string     `json:"status"`
	Executor    string     `json:"executor"`
	Agent       string     `json:"agent"`
	Model       string     `json:"model"`
	FixBranch   string     `json:"fixBranch"`
	CostUSD     float64    `json:"costUsd"`
	Turns       int        `json:"turns"`
	Error       string     `json:"error"`
	CreatedAt   time.Time  `json:"createdAt"`
	StartedAt   *time.Time `json:"startedAt"`
	FinishedAt  *time.Time `json:"finishedAt"`
}

// AttemptLink is an external artifact of an attempt: its origin, a thread,
// a pull request.
type AttemptLink struct {
	Provider    string `json:"provider"`
	Kind        string `json:"kind"`
	ExternalRef string `json:"externalRef"`
	URL         string `json:"url"`
}

// AttemptEvent is one entry of an attempt's event stream.
type AttemptEvent struct {
	Seq       int64          `json:"seq"`
	Kind      string         `json:"kind"`
	Payload   map[string]any `json:"payload"`
	CreatedAt time.Time      `json:"createdAt"`
}

type AttemptsPage struct {
	Data       []Attempt  `json:"data"`
	Pagination Pagination `json:"pagination"`
}

type AttemptDetail struct {
	Attempt Attempt       `json:"attempt"`
	Links   []AttemptLink `json:"links"`
}

type attemptEventsResponse struct {
	Events []AttemptEvent `json:"events"`
	Status string         `json:"status"`
}

// AttemptReport is what a CI executor reports into an attempt: the run's
// outcome, the branch and pull request it pushed when it fixed the issue,
// and the report text.
type AttemptReport struct {
	Hash           string `json:"hash"`
	Status         string `json:"status"`
	Branch         string `json:"branch,omitempty"`
	PullRequestURL string `json:"pullRequestUrl,omitempty"`
	Report         string `json:"report"`
}

type attemptsListRequest struct {
	Status     string           `json:"status"`
	Pagination PaginationParams `json:"pagination"`
}

// ListAttempts lists the project's attempts, newest first, optionally of
// one status.
func (c *Client) ListAttempts(ctx context.Context, projectID string, status string, page PaginationParams) (*AttemptsPage, error) {
	var out AttemptsPage
	path := "/api/agent/attempts/list?projectId=" + url.QueryEscape(projectID)
	if err := c.do(ctx, http.MethodPost, path, attemptsListRequest{Status: status, Pagination: page}, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// GetAttempt returns one attempt with its links.
func (c *Client) GetAttempt(ctx context.Context, projectID string, id string) (*AttemptDetail, error) {
	var out AttemptDetail
	path := "/api/agent/attempts/" + url.PathEscape(id) + "?projectId=" + url.QueryEscape(projectID)
	if err := c.do(ctx, http.MethodGet, path, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// AttemptEvents returns the attempt's events after a sequence number.
func (c *Client) AttemptEvents(ctx context.Context, projectID string, id string, after int64) ([]AttemptEvent, string, error) {
	var out attemptEventsResponse
	path := "/api/agent/attempts/" + url.PathEscape(id) + "/events?projectId=" + url.QueryEscape(projectID) + "&after=" + strconv.FormatInt(after, 10)
	if err := c.do(ctx, http.MethodGet, path, nil, &out); err != nil {
		return nil, "", err
	}
	return out.Events, out.Status, nil
}

// ReportAttempt records a CI run's outcome as an attempt on the issue.
func (c *Client) ReportAttempt(ctx context.Context, projectID string, report AttemptReport) (*Attempt, error) {
	var out struct {
		Attempt Attempt `json:"attempt"`
	}
	path := "/api/agent/attempts/report?projectId=" + url.QueryEscape(projectID)
	if err := c.do(ctx, http.MethodPost, path, report, &out); err != nil {
		return nil, err
	}
	return &out.Attempt, nil
}
