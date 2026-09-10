package access

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"
)

// Bound is a source together with the domains its profile entry lets it
// answer; an empty list means every domain the source implements.
type Bound struct {
	Source  Source
	Domains []Domain
}

func (b Bound) answers(domain Domain) bool {
	if !Implements(b.Source, domain) {
		return false
	}
	return len(b.Domains) == 0 || slices.Contains(b.Domains, domain)
}

// Resolver picks the sources that answer a domain, in the order they were
// bound, which is their priority.
type Resolver struct {
	bound []Bound
}

func NewResolver(bound ...Bound) *Resolver {
	return &Resolver{bound: bound}
}

func (r *Resolver) Bound() []Bound {
	return append([]Bound(nil), r.bound...)
}

// Sources returns the sources answering a domain. A non-empty name narrows
// the answer to that one source and fails when it is unknown or does not
// answer the domain.
func (r *Resolver) Sources(domain Domain, name string) ([]Source, error) {
	var out []Source
	for _, b := range r.bound {
		if name != "" && b.Source.Name() != name {
			continue
		}
		if b.answers(domain) {
			out = append(out, b.Source)
		}
	}
	if len(out) > 0 {
		return out, nil
	}
	if name != "" {
		for _, b := range r.bound {
			if b.Source.Name() == name {
				return nil, fmt.Errorf("source %q does not answer %s", name, domain)
			}
		}
		return nil, fmt.Errorf("%w %q (configured: %s)", ErrUnknownSource, name, strings.Join(r.names(), ", "))
	}
	return nil, fmt.Errorf("%w: %s", ErrNoSource, domain)
}

func (r *Resolver) names() []string {
	names := make([]string, 0, len(r.bound))
	for _, b := range r.bound {
		names = append(names, b.Source.Name())
	}
	return names
}

func sourcesAs[T any](r *Resolver, domain Domain, name string) ([]T, error) {
	sources, err := r.Sources(domain, name)
	if err != nil {
		return nil, err
	}
	out := make([]T, 0, len(sources))
	for _, s := range sources {
		out = append(out, s.(T))
	}
	return out, nil
}

func Logs(r *Resolver, name string) ([]LogsAccess, error) {
	return sourcesAs[LogsAccess](r, DomainLogs, name)
}

func Exceptions(r *Resolver, name string) ([]ExceptionAccess, error) {
	return sourcesAs[ExceptionAccess](r, DomainExceptions, name)
}

func Endpoints(r *Resolver, name string) ([]EndpointsAccess, error) {
	return sourcesAs[EndpointsAccess](r, DomainEndpoints, name)
}

func Metrics(r *Resolver, name string) ([]MetricsAccess, error) {
	return sourcesAs[MetricsAccess](r, DomainMetrics, name)
}

func Sessions(r *Resolver, name string) ([]SessionsAccess, error) {
	return sourcesAs[SessionsAccess](r, DomainSessions, name)
}

func Tasks(r *Resolver, name string) ([]TasksAccess, error) {
	return sourcesAs[TasksAccess](r, DomainTasks, name)
}

func AITraces(r *Resolver, name string) ([]AITracesAccess, error) {
	return sourcesAs[AITracesAccess](r, DomainAITraces, name)
}

func Traces(r *Resolver, name string) ([]TracesAccess, error) {
	return sourcesAs[TracesAccess](r, DomainTraces, name)
}

// Fan-out. A single source is called directly and its answer returned as is.
// With several, every source is asked, records are tagged with their source,
// merged, and the sources that failed are reported alongside; the call as a
// whole fails only when no source answered.

type answer[R any] struct {
	source string
	value  R
}

func fanOut[S Source, R any](sources []S, call func(S) (R, error)) ([]answer[R], []*SourceError, error) {
	var answers []answer[R]
	var failed []*SourceError
	for _, s := range sources {
		value, err := call(s)
		if err != nil {
			failed = append(failed, &SourceError{Source: s.Name(), Err: err})
			continue
		}
		answers = append(answers, answer[R]{source: s.Name(), value: value})
	}
	if len(answers) == 0 && len(failed) > 0 {
		return nil, failed, failed[0]
	}
	return answers, failed, nil
}

type tagged interface {
	SetSource(name string)
}

func tagAll[T any, P interface {
	*T
	tagged
}](source string, records []T) {
	for i := range records {
		P(&records[i]).SetSource(source)
	}
}

func mergedPagination(page PageRequest, pages []Pagination) Pagination {
	merged := Pagination{Page: page.Page, PageSize: page.PageSize}
	for _, p := range pages {
		merged.Total += p.Total
		merged.TotalPages = max(merged.TotalPages, p.TotalPages)
	}
	return merged
}

func descending(direction string) bool {
	return !strings.EqualFold(direction, "asc")
}

func directed(c int, desc bool) int {
	if desc {
		return -c
	}
	return c
}

// QueryLogs asks every source and merges the records by timestamp in the
// query's sort direction.
func QueryLogs(ctx context.Context, sources []LogsAccess, q LogQuery) (*LogPage, []*SourceError, error) {
	if len(sources) == 1 {
		page, err := sources[0].QueryLogs(ctx, q)
		return page, nil, err
	}
	answers, failed, err := fanOut(sources, func(s LogsAccess) (*LogPage, error) { return s.QueryLogs(ctx, q) })
	if err != nil {
		return nil, failed, err
	}
	merged := &LogPage{Data: []LogRecord{}}
	var pages []Pagination
	for _, a := range answers {
		tagAll[LogRecord](a.source, a.value.Data)
		merged.Data = append(merged.Data, a.value.Data...)
		pages = append(pages, a.value.Pagination)
	}
	desc := descending(q.SortDirection)
	slices.SortStableFunc(merged.Data, func(a, b LogRecord) int {
		return directed(a.Timestamp.Compare(b.Timestamp), desc)
	})
	merged.Pagination = mergedPagination(q.Page, pages)
	return merged, failed, nil
}

// ListExceptions asks every source and merges the groups by the query's
// order field, descending like the Traceway API.
func ListExceptions(ctx context.Context, sources []ExceptionAccess, q ExceptionQuery) (*ExceptionPage, []*SourceError, error) {
	if len(sources) == 1 {
		page, err := sources[0].ListExceptions(ctx, q)
		return page, nil, err
	}
	answers, failed, err := fanOut(sources, func(s ExceptionAccess) (*ExceptionPage, error) { return s.ListExceptions(ctx, q) })
	if err != nil {
		return nil, failed, err
	}
	merged := &ExceptionPage{Data: []ExceptionGroup{}}
	var pages []Pagination
	for _, a := range answers {
		tagAll[ExceptionGroup](a.source, a.value.Data)
		merged.Data = append(merged.Data, a.value.Data...)
		pages = append(pages, a.value.Pagination)
	}
	slices.SortStableFunc(merged.Data, func(a, b ExceptionGroup) int {
		switch q.OrderBy {
		case "firstSeen":
			return b.FirstSeen.Compare(a.FirstSeen)
		case "count":
			return cmp.Compare(b.Count, a.Count)
		default:
			return b.LastSeen.Compare(a.LastSeen)
		}
	})
	merged.Pagination = mergedPagination(q.Page, pages)
	return merged, failed, nil
}

// ListEndpoints asks every source and merges the rows by the query's order
// field and direction.
func ListEndpoints(ctx context.Context, sources []EndpointsAccess, q EndpointQuery) (*EndpointPage, []*SourceError, error) {
	if len(sources) == 1 {
		page, err := sources[0].ListEndpoints(ctx, q)
		return page, nil, err
	}
	answers, failed, err := fanOut(sources, func(s EndpointsAccess) (*EndpointPage, error) { return s.ListEndpoints(ctx, q) })
	if err != nil {
		return nil, failed, err
	}
	merged := &EndpointPage{Data: []EndpointStats{}}
	var pages []Pagination
	for _, a := range answers {
		tagAll[EndpointStats](a.source, a.value.Data)
		merged.Data = append(merged.Data, a.value.Data...)
		pages = append(pages, a.value.Pagination)
	}
	desc := descending(q.SortDirection)
	slices.SortStableFunc(merged.Data, func(a, b EndpointStats) int {
		switch q.OrderBy {
		case "count":
			return directed(cmp.Compare(a.Count, b.Count), desc)
		case "p95":
			return directed(cmp.Compare(a.P95Duration, b.P95Duration), desc)
		case "lastSeen":
			return directed(a.LastSeen.Compare(b.LastSeen), desc)
		default:
			return directed(cmp.Compare(a.Impact, b.Impact), desc)
		}
	})
	merged.Pagination = mergedPagination(q.Page, pages)
	return merged, failed, nil
}

// QueryMetrics asks every source and concatenates their series, tagged.
func QueryMetrics(ctx context.Context, sources []MetricsAccess, q MetricQuery) (*MetricResult, []*SourceError, error) {
	if len(sources) == 1 {
		result, err := sources[0].QueryMetrics(ctx, q)
		return result, nil, err
	}
	answers, failed, err := fanOut(sources, func(s MetricsAccess) (*MetricResult, error) { return s.QueryMetrics(ctx, q) })
	if err != nil {
		return nil, failed, err
	}
	merged := &MetricResult{Results: []MetricSeries{}}
	for _, a := range answers {
		tagAll[MetricSeries](a.source, a.value.Results)
		merged.Results = append(merged.Results, a.value.Results...)
		merged.IntervalMinutes = max(merged.IntervalMinutes, a.value.IntervalMinutes)
	}
	return merged, failed, nil
}

// DiscoverMetrics asks every source and concatenates the metric names, tagged.
func DiscoverMetrics(ctx context.Context, sources []MetricsAccess, q MetricDiscovery) ([]MetricInfo, []*SourceError, error) {
	if len(sources) == 1 {
		infos, err := sources[0].DiscoverMetrics(ctx, q)
		return infos, nil, err
	}
	answers, failed, err := fanOut(sources, func(s MetricsAccess) ([]MetricInfo, error) { return s.DiscoverMetrics(ctx, q) })
	if err != nil {
		return nil, failed, err
	}
	merged := []MetricInfo{}
	for _, a := range answers {
		tagAll[MetricInfo](a.source, a.value)
		merged = append(merged, a.value...)
	}
	return merged, failed, nil
}

// first asks the sources in priority order and returns the first answer that
// is not ErrNotFound; a record lives at one source, so other sources are only
// consulted when the one before did not have it.
func first[S Source, R any](sources []S, call func(S) (R, error)) (R, error) {
	var zero R
	lastErr := ErrNotFound
	for _, s := range sources {
		value, err := call(s)
		if err == nil {
			return value, nil
		}
		if !errors.Is(err, ErrNotFound) {
			return zero, &SourceError{Source: s.Name(), Err: err}
		}
		lastErr = err
	}
	return zero, lastErr
}

func GetException(ctx context.Context, sources []ExceptionAccess, l ExceptionLookup) (*ExceptionDetail, error) {
	if len(sources) == 1 {
		return sources[0].GetException(ctx, l)
	}
	return first(sources, func(s ExceptionAccess) (*ExceptionDetail, error) { return s.GetException(ctx, l) })
}

func GetOccurrence(ctx context.Context, sources []ExceptionAccess, l Lookup) (*OccurrenceDetail, error) {
	if len(sources) == 1 {
		return sources[0].GetOccurrence(ctx, l)
	}
	return first(sources, func(s ExceptionAccess) (*OccurrenceDetail, error) { return s.GetOccurrence(ctx, l) })
}

func GetRequest(ctx context.Context, sources []EndpointsAccess, l Lookup) (*RequestDetail, error) {
	if len(sources) == 1 {
		return sources[0].GetRequest(ctx, l)
	}
	return first(sources, func(s EndpointsAccess) (*RequestDetail, error) { return s.GetRequest(ctx, l) })
}

func GetTask(ctx context.Context, sources []TasksAccess, l Lookup) (*TaskDetail, error) {
	if len(sources) == 1 {
		return sources[0].GetTask(ctx, l)
	}
	return first(sources, func(s TasksAccess) (*TaskDetail, error) { return s.GetTask(ctx, l) })
}

func GetAITrace(ctx context.Context, sources []AITracesAccess, l Lookup) (*AITraceDetail, error) {
	if len(sources) == 1 {
		return sources[0].GetAITrace(ctx, l)
	}
	return first(sources, func(s AITracesAccess) (*AITraceDetail, error) { return s.GetAITrace(ctx, l) })
}

func GetSession(ctx context.Context, sources []SessionsAccess, l Lookup) (*SessionDetail, error) {
	if len(sources) == 1 {
		return sources[0].GetSession(ctx, l)
	}
	return first(sources, func(s SessionsAccess) (*SessionDetail, error) { return s.GetSession(ctx, l) })
}

func GetTrace(ctx context.Context, sources []TracesAccess, id string, at time.Time) (*Trace, error) {
	if len(sources) == 1 {
		return sources[0].GetTrace(ctx, id, at)
	}
	return first(sources, func(s TracesAccess) (*Trace, error) { return s.GetTrace(ctx, id, at) })
}
