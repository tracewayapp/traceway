package client

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"
)

func (c *Client) ListDashboards(ctx context.Context, projectID string) (json.RawMessage, error) {
	var out json.RawMessage
	err := c.do(ctx, http.MethodGet, "/api/dashboards"+projectQuery(projectID), nil, &out)
	return out, err
}

func (c *Client) DashboardLibrary(ctx context.Context) (json.RawMessage, error) {
	var out json.RawMessage
	err := c.do(ctx, http.MethodGet, "/api/dashboards/library", nil, &out)
	return out, err
}

func (c *Client) ExportDashboard(ctx context.Context, projectID string, id int) (json.RawMessage, error) {
	var out json.RawMessage
	err := c.do(ctx, http.MethodGet, "/api/dashboards/"+strconv.Itoa(id)+"/export"+projectQuery(projectID), nil, &out)
	return out, err
}

func (c *Client) ImportDashboards(ctx context.Context, projectID string, data json.RawMessage, mode string) (json.RawMessage, error) {
	var out json.RawMessage
	body := struct {
		Data json.RawMessage `json:"data"`
		Mode string          `json:"mode"`
	}{data, mode}
	err := c.do(ctx, http.MethodPost, "/api/dashboards/import"+projectQuery(projectID), body, &out)
	return out, err
}

// DashboardEmbedMaxDays is the longest a share link may live. A link cannot be
// revoked on its own, so the server bounds it.
const DashboardEmbedMaxDays = 90

const dashboardEmbedMaxTagFilters = 20

var dashboardEmbedTimeframe = regexp.MustCompile(`^(last)?[1-9][0-9]*[mhd]$`)

// DashboardEmbedOptions is the view a share link signs and how long it lasts.
// The zero value is the server's default: the last 42 hours, for five minutes.
type DashboardEmbedOptions struct {
	Timeframe     string
	From, To      time.Time
	TagFilters    map[string]string
	ExpiresInDays int
	ExpiresAt     time.Time
	Theme         string
}

// Validate refuses what the server would answer 422 for, before a write is sent.
func (o DashboardEmbedOptions) Validate() error {
	fixed := !o.From.IsZero() || !o.To.IsZero()
	switch {
	case o.Timeframe != "" && fixed:
		return errors.New("use a relative timeframe or an explicit from and to range, not both")
	case o.Timeframe != "" && !dashboardEmbedTimeframe.MatchString(o.Timeframe):
		return errors.New("timeframe must be a positive whole number followed by m, h or d, such as 30m, 42h or 7d")
	case o.From.IsZero() != o.To.IsZero():
		return errors.New("a fixed range needs both from and to")
	case fixed && o.To.Before(o.From):
		return errors.New("to must not be before from")
	case o.ExpiresInDays != 0 && !o.ExpiresAt.IsZero():
		return errors.New("use an expiry in days or an expiry timestamp, not both")
	case o.ExpiresInDays < 0 || o.ExpiresInDays > DashboardEmbedMaxDays:
		return fmt.Errorf("the expiry must be between 1 and %d days", DashboardEmbedMaxDays)
	case o.Theme != "" && o.Theme != "light" && o.Theme != "dark":
		return errors.New("theme must be light or dark")
	case len(o.TagFilters) > dashboardEmbedMaxTagFilters:
		return fmt.Errorf("at most %d tag filters can be applied", dashboardEmbedMaxTagFilters)
	}
	for tag, value := range o.TagFilters {
		if strings.TrimSpace(tag) == "" || value == "" {
			return errors.New("a tag filter needs a non-empty tag and value")
		}
	}
	return nil
}

// CreateDashboardEmbed issues a share link for a dashboard applied to the project.
// The response carries the signed token, so treat it as a credential. It gains a
// "url" field: the returned embedPath resolved against origin, the address the
// viewer's browser reaches Traceway at.
func (c *Client) CreateDashboardEmbed(ctx context.Context, projectID string, id int, opts DashboardEmbedOptions, origin string) (json.RawMessage, error) {
	q := url.Values{"projectId": {projectID}}
	if opts.Timeframe != "" {
		q.Set("timeframe", opts.Timeframe)
	}
	if opts.Theme != "" {
		q.Set("theme", opts.Theme)
	}
	body := map[string]any{}
	if !opts.From.IsZero() {
		body["from"] = opts.From.UTC().Format(time.RFC3339Nano)
		body["to"] = opts.To.UTC().Format(time.RFC3339Nano)
	}
	if len(opts.TagFilters) > 0 {
		body["tagFilters"] = opts.TagFilters
	}
	if opts.ExpiresInDays != 0 {
		body["expiresInDays"] = opts.ExpiresInDays
	}
	if !opts.ExpiresAt.IsZero() {
		body["expiresAt"] = opts.ExpiresAt.UTC().Format(time.RFC3339)
	}
	var out map[string]any
	if err := c.do(ctx, http.MethodPost, "/api/dashboards/"+strconv.Itoa(id)+"/embed-token?"+q.Encode(), body, &out); err != nil {
		return nil, err
	}
	if path, ok := out["embedPath"].(string); ok && origin != "" {
		out["url"] = strings.TrimRight(origin, "/") + path
	}
	return json.Marshal(out)
}

func (c *Client) DiscoverMetrics(ctx context.Context, projectID string, tr TimeRange, name, key string) (json.RawMessage, error) {
	path := "/api/metrics/discover"
	q := url.Values{"projectId": {projectID}, "from": {tr.From.Format(time.RFC3339Nano)}, "to": {tr.To.Format(time.RFC3339Nano)}}
	if name != "" {
		path += "/tags"
		q.Set("name", name)
		q.Set("key", key)
	}
	var out json.RawMessage
	err := c.do(ctx, http.MethodGet, path+"?"+q.Encode(), nil, &out)
	return out, err
}
