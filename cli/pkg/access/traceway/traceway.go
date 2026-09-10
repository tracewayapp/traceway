// Package traceway is the access.Source over a Traceway instance. It answers
// every domain by delegating to pkg/client, and is registered under the
// provider key "traceway" so a profile can bind a second instance by URL and
// token; the profile's own instance is bound implicitly by the CLI.
package traceway

import (
	"context"
	"errors"
	"time"

	"github.com/tracewayapp/traceway/cli/pkg/access"
	"github.com/tracewayapp/traceway/cli/pkg/client"
)

const (
	Provider = "traceway"
	// DefaultName is the name of the profile's own instance, bound
	// implicitly ahead of every configured source.
	DefaultName = "traceway"
)

func init() {
	access.Register(Provider, func(cfg access.Config) (access.Source, error) {
		url, token := cfg.Settings["url"], cfg.Settings["token"]
		if url == "" || token == "" {
			return nil, errors.New("settings url and token are required")
		}
		return New(cfg.Name, client.New(url, client.WithJWT(token))), nil
	})
}

type Source struct {
	name   string
	client *client.Client
}

func New(name string, c *client.Client) *Source {
	return &Source{name: name, client: c}
}

func (s *Source) Name() string     { return s.name }
func (s *Source) Provider() string { return Provider }

func (s *Source) Capabilities() access.Capabilities {
	return access.Capabilities{Percentiles: true, Spans: true, Search: true, Archive: true, SlowEndpoints: true}
}

func (s *Source) QueryLogs(ctx context.Context, q access.LogQuery) (*access.LogPage, error) {
	return s.client.QueryLogs(ctx, q.ProjectID, client.QueryLogsRequest{
		TimeRange:     q.Window,
		Pagination:    q.Page,
		Search:        q.Search,
		SearchType:    q.SearchType,
		MinSeverity:   q.MinSeverity,
		ServiceName:   q.ServiceName,
		TraceId:       q.TraceID,
		OrderBy:       "timestamp",
		SortDirection: q.SortDirection,
	})
}

func (s *Source) ListExceptions(ctx context.Context, q access.ExceptionQuery) (*access.ExceptionPage, error) {
	return s.client.ListExceptions(ctx, q.ProjectID, client.ListExceptionsRequest{
		TimeRange:       q.Window,
		Pagination:      q.Page,
		Search:          q.Search,
		SearchType:      q.SearchType,
		OrderBy:         q.OrderBy,
		IncludeArchived: q.IncludeArchived,
	})
}

func (s *Source) GetException(ctx context.Context, l access.ExceptionLookup) (*access.ExceptionDetail, error) {
	return s.client.GetException(ctx, l.ProjectID, l.Hash, l.Page)
}

func (s *Source) GetOccurrence(ctx context.Context, l access.Lookup) (*access.OccurrenceDetail, error) {
	return s.client.GetExceptionById(ctx, l.ProjectID, l.ID, l.At)
}

func (s *Source) ArchiveExceptions(ctx context.Context, projectID string, hashes []string) error {
	return s.client.ArchiveExceptions(ctx, projectID, hashes)
}

func (s *Source) UnarchiveExceptions(ctx context.Context, projectID string, hashes []string) error {
	return s.client.UnarchiveExceptions(ctx, projectID, hashes)
}

func (s *Source) ListEndpoints(ctx context.Context, q access.EndpointQuery) (*access.EndpointPage, error) {
	return s.client.ListEndpoints(ctx, q.ProjectID, client.ListEndpointsRequest{
		TimeRange:     q.Window,
		Pagination:    q.Page,
		Search:        q.Search,
		OrderBy:       q.OrderBy,
		SortDirection: q.SortDirection,
	})
}

func (s *Source) GetRequest(ctx context.Context, l access.Lookup) (*access.RequestDetail, error) {
	return s.client.GetEndpoint(ctx, l.ProjectID, l.ID, l.At)
}

func (s *Source) EndpointChart(ctx context.Context, q access.EndpointChartQuery) (*access.EndpointChart, error) {
	return s.client.GetEndpointChart(ctx, q.ProjectID, client.EndpointChartRequest{
		TimeRange:       q.Window,
		MetricType:      q.MetricType,
		IntervalMinutes: q.IntervalMinutes,
	})
}

func (s *Source) SlowEndpoint(ctx context.Context, projectID, endpoint string) (*access.SlowEndpoint, error) {
	return s.client.GetSlowEndpoint(ctx, projectID, endpoint)
}

func (s *Source) QueryMetrics(ctx context.Context, q access.MetricQuery) (*access.MetricResult, error) {
	return s.client.QueryMetrics(ctx, q.ProjectID, client.QueryMetricsRequest{
		TimeRange:       q.Window,
		IntervalMinutes: q.IntervalMinutes,
		Queries:         q.Queries,
	})
}

func (s *Source) DiscoverMetrics(ctx context.Context, q access.MetricDiscovery) ([]access.MetricInfo, error) {
	resp, err := s.client.DiscoverMetrics(ctx, q.ProjectID, q.Window)
	if err != nil {
		return nil, err
	}
	return resp.Metrics, nil
}

func (s *Source) GetSession(ctx context.Context, l access.Lookup) (*access.SessionDetail, error) {
	return s.client.GetSession(ctx, l.ProjectID, l.ID, l.At)
}

func (s *Source) GetTask(ctx context.Context, l access.Lookup) (*access.TaskDetail, error) {
	return s.client.GetTask(ctx, l.ProjectID, l.ID, l.At)
}

func (s *Source) GetAITrace(ctx context.Context, l access.Lookup) (*access.AITraceDetail, error) {
	return s.client.GetAiTrace(ctx, l.ProjectID, l.ID, l.At)
}

func (s *Source) GetTrace(ctx context.Context, id string, at time.Time) (*access.Trace, error) {
	return s.client.GetDistributedTrace(ctx, id, at)
}

var (
	_ access.LogsAccess      = (*Source)(nil)
	_ access.ExceptionAccess = (*Source)(nil)
	_ access.EndpointsAccess = (*Source)(nil)
	_ access.MetricsAccess   = (*Source)(nil)
	_ access.SessionsAccess  = (*Source)(nil)
	_ access.TasksAccess     = (*Source)(nil)
	_ access.AITracesAccess  = (*Source)(nil)
	_ access.TracesAccess    = (*Source)(nil)
)
