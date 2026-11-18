package client

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"strconv"
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
