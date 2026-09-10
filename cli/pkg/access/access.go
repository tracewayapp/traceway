// Package access is the telemetry boundary of the Traceway CLI. Every command
// and every MCP tool reads telemetry through the interfaces here, and every
// provider (Traceway first, others as adapters) implements them, so nothing
// above this package speaks to a provider API.
//
// A Source is one configured provider instance. It answers one or more
// domains by implementing the matching interface: LogsAccess, ExceptionAccess,
// EndpointsAccess, MetricsAccess, and the Traceway-specific SessionsAccess,
// TasksAccess, AITracesAccess and TracesAccess. Capabilities says what a
// source cannot do so callers degrade honestly instead of guessing.
//
// The record and query types mirror the MCP tool inputs and outputs: those
// shapes are the contract, and the Traceway API is their reference
// implementation. Records carry a Provenance naming the source they came
// from; it is set only when more than one source answered.
package access

import (
	"context"
	"time"
)

// Domain is a family of telemetry a source can answer.
type Domain string

const (
	DomainLogs       Domain = "logs"
	DomainExceptions Domain = "exceptions"
	DomainEndpoints  Domain = "endpoints"
	DomainMetrics    Domain = "metrics"
	DomainSessions   Domain = "sessions"
	DomainTasks      Domain = "tasks"
	DomainAITraces   Domain = "ai_traces"
	DomainTraces     Domain = "traces"
)

// Domains lists every domain in display order.
var Domains = []Domain{DomainLogs, DomainExceptions, DomainEndpoints, DomainMetrics, DomainSessions, DomainTasks, DomainAITraces, DomainTraces}

// Source is one configured provider instance.
type Source interface {
	// Name is the profile-level name, such as "traceway" or "dd-prod".
	Name() string
	// Provider is the adapter key the source was opened with.
	Provider() string
	Capabilities() Capabilities
}

// Capabilities declares what a source can answer within the domains it
// implements. A false value means the corresponding fields come back empty
// or the corresponding method returns ErrUnsupported.
type Capabilities struct {
	// Percentiles: EndpointStats carry real p50/p95/p99 quantiles.
	Percentiles bool `json:"percentiles"`
	// Spans: request and task detail carry a span waterfall.
	Spans bool `json:"spans"`
	// Search: free-text filters on list queries are honored.
	Search bool `json:"search"`
	// Archive: exception groups can be archived and unarchived.
	Archive bool `json:"archive"`
	// SlowEndpoints: operator slow-endpoint allowances exist.
	SlowEndpoints bool `json:"slowEndpoints"`
}

type LogsAccess interface {
	Source
	QueryLogs(ctx context.Context, q LogQuery) (*LogPage, error)
}

type ExceptionAccess interface {
	Source
	ListExceptions(ctx context.Context, q ExceptionQuery) (*ExceptionPage, error)
	GetException(ctx context.Context, l ExceptionLookup) (*ExceptionDetail, error)
	GetOccurrence(ctx context.Context, l Lookup) (*OccurrenceDetail, error)
	ArchiveExceptions(ctx context.Context, projectID string, hashes []string) error
	UnarchiveExceptions(ctx context.Context, projectID string, hashes []string) error
}

type EndpointsAccess interface {
	Source
	ListEndpoints(ctx context.Context, q EndpointQuery) (*EndpointPage, error)
	GetRequest(ctx context.Context, l Lookup) (*RequestDetail, error)
	EndpointChart(ctx context.Context, q EndpointChartQuery) (*EndpointChart, error)
	SlowEndpoint(ctx context.Context, projectID, endpoint string) (*SlowEndpoint, error)
}

type MetricsAccess interface {
	Source
	QueryMetrics(ctx context.Context, q MetricQuery) (*MetricResult, error)
	DiscoverMetrics(ctx context.Context, q MetricDiscovery) ([]MetricInfo, error)
}

type SessionsAccess interface {
	Source
	GetSession(ctx context.Context, l Lookup) (*SessionDetail, error)
}

type TasksAccess interface {
	Source
	GetTask(ctx context.Context, l Lookup) (*TaskDetail, error)
}

type AITracesAccess interface {
	Source
	GetAITrace(ctx context.Context, l Lookup) (*AITraceDetail, error)
}

// TracesAccess resolves distributed traces, which span every project the
// caller can see, so its lookup carries no project.
type TracesAccess interface {
	Source
	GetTrace(ctx context.Context, id string, at time.Time) (*Trace, error)
}

// Implements reports whether a source answers a domain.
func Implements(s Source, domain Domain) bool {
	switch domain {
	case DomainLogs:
		_, ok := s.(LogsAccess)
		return ok
	case DomainExceptions:
		_, ok := s.(ExceptionAccess)
		return ok
	case DomainEndpoints:
		_, ok := s.(EndpointsAccess)
		return ok
	case DomainMetrics:
		_, ok := s.(MetricsAccess)
		return ok
	case DomainSessions:
		_, ok := s.(SessionsAccess)
		return ok
	case DomainTasks:
		_, ok := s.(TasksAccess)
		return ok
	case DomainAITraces:
		_, ok := s.(AITracesAccess)
		return ok
	case DomainTraces:
		_, ok := s.(TracesAccess)
		return ok
	}
	return false
}

// ImplementedDomains lists the domains a source answers, in display order.
func ImplementedDomains(s Source) []Domain {
	var out []Domain
	for _, d := range Domains {
		if Implements(s, d) {
			out = append(out, d)
		}
	}
	return out
}
