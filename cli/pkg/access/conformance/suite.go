// Package conformance is the contract every access.Source must satisfy. An
// adapter's test opens its source against recorded fixtures and calls
// RunSuite with the ids those fixtures contain; the suite checks each domain
// the source implements the same way, so adapters cannot drift apart.
package conformance

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/tracewayapp/traceway/cli/pkg/access"
)

// Fixtures names records the source under test can answer with. Ids for
// domains the source does not implement may be left empty.
type Fixtures struct {
	ProjectID     string
	Window        access.Window
	ExceptionHash string
	OccurrenceID  string
	RequestID     string
	EndpointName  string
	MetricName    string
	TaskID        string
	AITraceID     string
	SessionID     string
	TraceID       string
	// At is the timestamp the by-id records carry.
	At time.Time
	// UnknownHash and UnknownID are well-formed refs that exist nowhere.
	UnknownHash string
	UnknownID   string
}

func RunSuite(t *testing.T, source access.Source, fx Fixtures) {
	t.Helper()
	ctx := context.Background()
	if source.Name() == "" || source.Provider() == "" {
		t.Fatalf("source must have a name and a provider, got %q/%q", source.Name(), source.Provider())
	}
	caps := source.Capabilities()

	if logs, ok := source.(access.LogsAccess); ok {
		t.Run("logs", func(t *testing.T) {
			page, err := logs.QueryLogs(ctx, access.LogQuery{ProjectID: fx.ProjectID, Window: fx.Window, Page: access.PageRequest{Page: 1, PageSize: 20}})
			if err != nil {
				t.Fatalf("QueryLogs: %v", err)
			}
			if len(page.Data) == 0 {
				t.Fatal("QueryLogs returned no records inside the fixture window")
			}
			for _, record := range page.Data {
				if record.Timestamp.Before(fx.Window.From) || record.Timestamp.After(fx.Window.To) {
					t.Errorf("record %s at %s is outside the window", record.Id, record.Timestamp)
				}
				assertUntagged(t, record.Provenance)
			}
		})
	}

	if exceptions, ok := source.(access.ExceptionAccess); ok {
		t.Run("exceptions", func(t *testing.T) {
			page, err := exceptions.ListExceptions(ctx, access.ExceptionQuery{ProjectID: fx.ProjectID, Window: fx.Window, Page: access.PageRequest{Page: 1, PageSize: 20}, OrderBy: "lastSeen"})
			if err != nil {
				t.Fatalf("ListExceptions: %v", err)
			}
			if !containsGroup(page.Data, fx.ExceptionHash) {
				t.Fatalf("ListExceptions did not return group %s", fx.ExceptionHash)
			}
			for _, group := range page.Data {
				assertUntagged(t, group.Provenance)
			}

			detail, err := exceptions.GetException(ctx, access.ExceptionLookup{ProjectID: fx.ProjectID, Hash: fx.ExceptionHash, Page: access.PageRequest{Page: 1, PageSize: 20}})
			if err != nil {
				t.Fatalf("GetException: %v", err)
			}
			if detail.Group == nil || detail.Group.ExceptionHash != fx.ExceptionHash {
				t.Fatalf("GetException returned group %+v", detail.Group)
			}
			if len(detail.Occurrences) == 0 {
				t.Fatal("GetException returned no occurrences")
			}
			if _, err := exceptions.GetException(ctx, access.ExceptionLookup{ProjectID: fx.ProjectID, Hash: fx.UnknownHash, Page: access.PageRequest{Page: 1, PageSize: 20}}); !errors.Is(err, access.ErrNotFound) {
				t.Errorf("GetException(unknown) = %v, want ErrNotFound", err)
			}

			occurrence, err := exceptions.GetOccurrence(ctx, access.Lookup{ProjectID: fx.ProjectID, ID: fx.OccurrenceID, At: fx.At})
			if err != nil {
				t.Fatalf("GetOccurrence: %v", err)
			}
			if occurrence.Exception == nil || occurrence.Exception.Id.String() != fx.OccurrenceID {
				t.Fatalf("GetOccurrence returned %+v", occurrence.Exception)
			}
			if _, err := exceptions.GetOccurrence(ctx, access.Lookup{ProjectID: fx.ProjectID, ID: fx.UnknownID, At: fx.At}); !errors.Is(err, access.ErrNotFound) {
				t.Errorf("GetOccurrence(unknown) = %v, want ErrNotFound", err)
			}

			err = exceptions.ArchiveExceptions(ctx, fx.ProjectID, []string{fx.ExceptionHash})
			if caps.Archive && err != nil {
				t.Errorf("ArchiveExceptions: %v", err)
			}
			if !caps.Archive && !errors.Is(err, access.ErrUnsupported) {
				t.Errorf("ArchiveExceptions without the capability = %v, want ErrUnsupported", err)
			}
			if caps.Archive {
				if err := exceptions.UnarchiveExceptions(ctx, fx.ProjectID, []string{fx.ExceptionHash}); err != nil {
					t.Errorf("UnarchiveExceptions: %v", err)
				}
			}
		})
	}

	if endpoints, ok := source.(access.EndpointsAccess); ok {
		t.Run("endpoints", func(t *testing.T) {
			page, err := endpoints.ListEndpoints(ctx, access.EndpointQuery{ProjectID: fx.ProjectID, Window: fx.Window, Page: access.PageRequest{Page: 1, PageSize: 20}, OrderBy: "impact", SortDirection: "desc"})
			if err != nil {
				t.Fatalf("ListEndpoints: %v", err)
			}
			var found bool
			for _, stats := range page.Data {
				assertUntagged(t, stats.Provenance)
				if stats.Endpoint != fx.EndpointName {
					continue
				}
				found = true
				if caps.Percentiles && (stats.P50Duration == 0 || stats.P95Duration == 0 || stats.P99Duration == 0) {
					t.Errorf("Percentiles declared but %s carries p50=%s p95=%s p99=%s", stats.Endpoint, stats.P50Duration, stats.P95Duration, stats.P99Duration)
				}
			}
			if !found {
				t.Fatalf("ListEndpoints did not return %s", fx.EndpointName)
			}

			request, err := endpoints.GetRequest(ctx, access.Lookup{ProjectID: fx.ProjectID, ID: fx.RequestID, At: fx.At})
			if err != nil {
				t.Fatalf("GetRequest: %v", err)
			}
			if request.Endpoint == nil || request.Endpoint.Id.String() != fx.RequestID {
				t.Fatalf("GetRequest returned %+v", request.Endpoint)
			}
			if caps.Spans && request.HasSpans && len(request.Spans) == 0 {
				t.Error("Spans declared and HasSpans set, but no spans returned")
			}
			if _, err := endpoints.GetRequest(ctx, access.Lookup{ProjectID: fx.ProjectID, ID: fx.UnknownID, At: fx.At}); !errors.Is(err, access.ErrNotFound) {
				t.Errorf("GetRequest(unknown) = %v, want ErrNotFound", err)
			}

			chart, err := endpoints.EndpointChart(ctx, access.EndpointChartQuery{ProjectID: fx.ProjectID, Window: fx.Window, MetricType: "p95"})
			if err != nil {
				t.Fatalf("EndpointChart: %v", err)
			}
			if len(chart.Endpoints) == 0 {
				t.Error("EndpointChart returned no endpoints")
			}

			slow, err := endpoints.SlowEndpoint(ctx, fx.ProjectID, fx.EndpointName)
			if caps.SlowEndpoints && (err != nil || slow == nil) {
				t.Errorf("SlowEndpoint = %v, %v", slow, err)
			}
			if !caps.SlowEndpoints && !errors.Is(err, access.ErrUnsupported) {
				t.Errorf("SlowEndpoint without the capability = %v, want ErrUnsupported", err)
			}
		})
	}

	if metrics, ok := source.(access.MetricsAccess); ok {
		t.Run("metrics", func(t *testing.T) {
			result, err := metrics.QueryMetrics(ctx, access.MetricQuery{ProjectID: fx.ProjectID, Window: fx.Window, Queries: []access.MetricQueryItem{{Name: fx.MetricName, Aggregation: "avg"}}})
			if err != nil {
				t.Fatalf("QueryMetrics: %v", err)
			}
			if len(result.Results) == 0 || result.Results[0].Name != fx.MetricName {
				t.Fatalf("QueryMetrics returned %+v", result.Results)
			}
			for _, series := range result.Results {
				assertUntagged(t, series.Provenance)
			}
			infos, err := metrics.DiscoverMetrics(ctx, access.MetricDiscovery{ProjectID: fx.ProjectID, Window: fx.Window})
			if err != nil {
				t.Fatalf("DiscoverMetrics: %v", err)
			}
			var found bool
			for _, info := range infos {
				found = found || info.Name == fx.MetricName
			}
			if !found {
				t.Errorf("DiscoverMetrics did not list %s", fx.MetricName)
			}
		})
	}

	if tasks, ok := source.(access.TasksAccess); ok {
		t.Run("tasks", func(t *testing.T) {
			task, err := tasks.GetTask(ctx, access.Lookup{ProjectID: fx.ProjectID, ID: fx.TaskID, At: fx.At})
			if err != nil {
				t.Fatalf("GetTask: %v", err)
			}
			if task.Task == nil || task.Task.Id.String() != fx.TaskID {
				t.Fatalf("GetTask returned %+v", task.Task)
			}
			if _, err := tasks.GetTask(ctx, access.Lookup{ProjectID: fx.ProjectID, ID: fx.UnknownID, At: fx.At}); !errors.Is(err, access.ErrNotFound) {
				t.Errorf("GetTask(unknown) = %v, want ErrNotFound", err)
			}
		})
	}

	if aiTraces, ok := source.(access.AITracesAccess); ok {
		t.Run("ai_traces", func(t *testing.T) {
			trace, err := aiTraces.GetAITrace(ctx, access.Lookup{ProjectID: fx.ProjectID, ID: fx.AITraceID, At: fx.At})
			if err != nil {
				t.Fatalf("GetAITrace: %v", err)
			}
			if trace.AiTrace == nil || trace.AiTrace.Id.String() != fx.AITraceID {
				t.Fatalf("GetAITrace returned %+v", trace.AiTrace)
			}
		})
	}

	if sessions, ok := source.(access.SessionsAccess); ok {
		t.Run("sessions", func(t *testing.T) {
			session, err := sessions.GetSession(ctx, access.Lookup{ProjectID: fx.ProjectID, ID: fx.SessionID, At: fx.At})
			if err != nil {
				t.Fatalf("GetSession: %v", err)
			}
			if session.Session == nil || session.Session.Id.String() != fx.SessionID {
				t.Fatalf("GetSession returned %+v", session.Session)
			}
		})
	}

	if traces, ok := source.(access.TracesAccess); ok {
		t.Run("traces", func(t *testing.T) {
			trace, err := traces.GetTrace(ctx, fx.TraceID, fx.At)
			if err != nil {
				t.Fatalf("GetTrace: %v", err)
			}
			if trace.DistributedTraceId != fx.TraceID || len(trace.Nodes) == 0 {
				t.Fatalf("GetTrace returned %+v", trace)
			}
			if _, err := traces.GetTrace(ctx, fx.UnknownID, fx.At); !errors.Is(err, access.ErrNotFound) {
				t.Errorf("GetTrace(unknown) = %v, want ErrNotFound", err)
			}
		})
	}
}

// assertUntagged checks that an adapter leaves Source empty: the fan-out
// tags records, and only when more than one source answered.
func assertUntagged(t *testing.T, p access.Provenance) {
	t.Helper()
	if p.Source != "" {
		t.Errorf("adapter pre-tagged a record with source %q", p.Source)
	}
}

func containsGroup(groups []access.ExceptionGroup, hash string) bool {
	for _, group := range groups {
		if group.ExceptionHash == hash {
			return true
		}
	}
	return false
}
