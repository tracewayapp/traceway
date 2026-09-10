package access

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
)

// fakeSource answers logs and endpoints with canned pages, and fails on
// request when a test wants a broken source.
type fakeSource struct {
	name string
	logs []LogRecord
	rows []EndpointStats
	fail error
	got  []string
}

func (f *fakeSource) Name() string               { return f.name }
func (f *fakeSource) Provider() string           { return "fake" }
func (f *fakeSource) Capabilities() Capabilities { return Capabilities{} }

func (f *fakeSource) QueryLogs(_ context.Context, q LogQuery) (*LogPage, error) {
	f.got = append(f.got, "logs")
	if f.fail != nil {
		return nil, f.fail
	}
	return &LogPage{Data: append([]LogRecord(nil), f.logs...), Pagination: Pagination{Page: q.Page.Page, PageSize: q.Page.PageSize, Total: int64(len(f.logs)), TotalPages: 1}}, nil
}

func (f *fakeSource) ListEndpoints(_ context.Context, q EndpointQuery) (*EndpointPage, error) {
	f.got = append(f.got, "endpoints")
	if f.fail != nil {
		return nil, f.fail
	}
	return &EndpointPage{Data: append([]EndpointStats(nil), f.rows...), Pagination: Pagination{Total: int64(len(f.rows)), TotalPages: 2}}, nil
}

func (f *fakeSource) GetRequest(_ context.Context, l Lookup) (*RequestDetail, error) {
	f.got = append(f.got, "request:"+l.ID)
	if f.fail != nil {
		return nil, f.fail
	}
	for _, row := range f.rows {
		if row.Endpoint == l.ID {
			return &RequestDetail{Endpoint: &Request{Endpoint: row.Endpoint}}, nil
		}
	}
	return nil, ErrNotFound
}

func (f *fakeSource) EndpointChart(context.Context, EndpointChartQuery) (*EndpointChart, error) {
	return nil, ErrUnsupported
}

func (f *fakeSource) SlowEndpoint(context.Context, string, string) (*SlowEndpoint, error) {
	return nil, ErrUnsupported
}

func at(minute int) time.Time { return time.Date(2026, 6, 1, 12, minute, 0, 0, time.UTC) }

func logAt(minute int) LogRecord {
	return LogRecord{Id: uuid.New(), Timestamp: at(minute), Body: at(minute).Format("15:04")}
}

func TestResolverPicksSourcesByDomainAndBinding(t *testing.T) {
	primary := &fakeSource{name: "traceway"}
	logsOnly := &fakeSource{name: "dd-logs"}
	r := NewResolver(Bound{Source: primary}, Bound{Source: logsOnly, Domains: []Domain{DomainLogs}})

	logs, err := Logs(r, "")
	if err != nil || len(logs) != 2 || logs[0].Name() != "traceway" || logs[1].Name() != "dd-logs" {
		t.Fatalf("Logs = %v, %v", logs, err)
	}
	endpoints, err := Endpoints(r, "")
	if err != nil || len(endpoints) != 1 || endpoints[0].Name() != "traceway" {
		t.Fatalf("Endpoints = %v, %v (binding must exclude dd-logs)", endpoints, err)
	}
	if _, err := Metrics(r, ""); !errors.Is(err, ErrNoSource) {
		t.Fatalf("Metrics = %v, want ErrNoSource", err)
	}

	named, err := Logs(r, "dd-logs")
	if err != nil || len(named) != 1 || named[0].Name() != "dd-logs" {
		t.Fatalf("Logs(dd-logs) = %v, %v", named, err)
	}
	if _, err := Endpoints(r, "dd-logs"); err == nil || !strings.Contains(err.Error(), "does not answer endpoints") {
		t.Fatalf("Endpoints(dd-logs) = %v", err)
	}
	if _, err := Logs(r, "nope"); !errors.Is(err, ErrUnknownSource) || !strings.Contains(err.Error(), "traceway, dd-logs") {
		t.Fatalf("Logs(nope) = %v", err)
	}
}

func TestQueryLogsSingleSourcePassesThroughUntagged(t *testing.T) {
	only := &fakeSource{name: "traceway", logs: []LogRecord{logAt(5)}}
	page, failed, err := QueryLogs(context.Background(), []LogsAccess{only}, LogQuery{})
	if err != nil || len(failed) != 0 {
		t.Fatalf("QueryLogs = %v, %v", failed, err)
	}
	if page.Data[0].Source != "" {
		t.Fatalf("single source tagged the record with %q", page.Data[0].Source)
	}
}

func TestQueryLogsMergesByTimeAndTagsSources(t *testing.T) {
	a := &fakeSource{name: "traceway", logs: []LogRecord{logAt(10), logAt(2)}}
	b := &fakeSource{name: "dd-prod", logs: []LogRecord{logAt(7)}}
	page, failed, err := QueryLogs(context.Background(), []LogsAccess{a, b}, LogQuery{SortDirection: "desc", Page: PageRequest{Page: 1, PageSize: 20}})
	if err != nil || len(failed) != 0 {
		t.Fatalf("QueryLogs = %v, %v", failed, err)
	}
	var order []string
	for _, record := range page.Data {
		order = append(order, record.Body+"@"+record.Source)
	}
	if got := strings.Join(order, " "); got != "12:10@traceway 12:07@dd-prod 12:02@traceway" {
		t.Fatalf("merged order = %s", got)
	}
	if page.Pagination.Total != 3 || page.Pagination.Page != 1 || page.Pagination.PageSize != 20 {
		t.Fatalf("pagination = %+v", page.Pagination)
	}

	ascending, _, _ := QueryLogs(context.Background(), []LogsAccess{a, b}, LogQuery{SortDirection: "asc"})
	if ascending.Data[0].Body != "12:02" {
		t.Fatalf("ascending merge starts with %s", ascending.Data[0].Body)
	}
}

func TestFanOutReportsFailedSourcesWithoutFailingTheCall(t *testing.T) {
	healthy := &fakeSource{name: "traceway", logs: []LogRecord{logAt(1)}}
	broken := &fakeSource{name: "dd-prod", fail: errors.New("boom")}
	page, failed, err := QueryLogs(context.Background(), []LogsAccess{healthy, broken}, LogQuery{})
	if err != nil {
		t.Fatalf("one healthy source must answer: %v", err)
	}
	if len(page.Data) != 1 || len(failed) != 1 || failed[0].Source != "dd-prod" || !strings.Contains(failed[0].Error(), "boom") {
		t.Fatalf("page=%d failed=%v", len(page.Data), failed)
	}

	alsoBroken := &fakeSource{name: "traceway", fail: errors.New("down")}
	_, failed, err = QueryLogs(context.Background(), []LogsAccess{alsoBroken, broken}, LogQuery{})
	if err == nil || len(failed) != 2 || !strings.Contains(err.Error(), "down") {
		t.Fatalf("all failing: err=%v failed=%v", err, failed)
	}
}

func TestListEndpointsMergesByOrderField(t *testing.T) {
	a := &fakeSource{name: "a", rows: []EndpointStats{{Endpoint: "x", Impact: 0.2, Count: 5}, {Endpoint: "y", Impact: 0.9, Count: 1}}}
	b := &fakeSource{name: "b", rows: []EndpointStats{{Endpoint: "z", Impact: 0.5, Count: 9}}}
	page, _, err := ListEndpoints(context.Background(), []EndpointsAccess{a, b}, EndpointQuery{OrderBy: "impact", SortDirection: "desc"})
	if err != nil {
		t.Fatal(err)
	}
	if names(page.Data) != "y@a z@b x@a" {
		t.Fatalf("impact desc = %s", names(page.Data))
	}
	page, _, _ = ListEndpoints(context.Background(), []EndpointsAccess{a, b}, EndpointQuery{OrderBy: "count", SortDirection: "asc"})
	if names(page.Data) != "y@a x@a z@b" {
		t.Fatalf("count asc = %s", names(page.Data))
	}
	if page.Pagination.TotalPages != 2 || page.Pagination.Total != 3 {
		t.Fatalf("pagination = %+v", page.Pagination)
	}
}

func names(rows []EndpointStats) string {
	var out []string
	for _, row := range rows {
		out = append(out, row.Endpoint+"@"+row.Source)
	}
	return strings.Join(out, " ")
}

func TestLookupsAskSourcesInOrderUntilOneHasTheRecord(t *testing.T) {
	a := &fakeSource{name: "a", rows: []EndpointStats{{Endpoint: "only-in-a"}}}
	b := &fakeSource{name: "b", rows: []EndpointStats{{Endpoint: "only-in-b"}}}
	detail, err := GetRequest(context.Background(), []EndpointsAccess{a, b}, Lookup{ID: "only-in-b"})
	if err != nil || detail.Endpoint.Endpoint != "only-in-b" {
		t.Fatalf("GetRequest = %+v, %v", detail, err)
	}
	if strings.Join(a.got, ",") != "request:only-in-b" || strings.Join(b.got, ",") != "request:only-in-b" {
		t.Fatalf("asked a=%v b=%v", a.got, b.got)
	}
	if _, err := GetRequest(context.Background(), []EndpointsAccess{a, b}, Lookup{ID: "nowhere"}); !errors.Is(err, ErrNotFound) {
		t.Fatalf("missing everywhere = %v", err)
	}

	broken := &fakeSource{name: "broken", fail: errors.New("timeout")}
	_, err = GetRequest(context.Background(), []EndpointsAccess{broken, b}, Lookup{ID: "only-in-b"})
	var sourceErr *SourceError
	if !errors.As(err, &sourceErr) || sourceErr.Source != "broken" {
		t.Fatalf("a real failure must stop the walk: %v", err)
	}
}

func TestRegistryOpensRegisteredProvidersOnly(t *testing.T) {
	Register("fake", func(cfg Config) (Source, error) {
		return &fakeSource{name: cfg.Name}, nil
	})
	if _, err := Open(Config{Name: "x", Provider: "nope"}); !errors.Is(err, ErrUnknownProvider) || !strings.Contains(err.Error(), "fake") {
		t.Fatalf("Open(nope) = %v", err)
	}
	source, err := Open(Config{Name: "x", Provider: "fake", Domains: []Domain{DomainLogs, DomainEndpoints}})
	if err != nil || source.Name() != "x" {
		t.Fatalf("Open(fake) = %v, %v", source, err)
	}
	if _, err := Open(Config{Name: "x", Provider: "fake", Domains: []Domain{DomainMetrics}}); err == nil || !strings.Contains(err.Error(), "does not answer metrics") {
		t.Fatalf("Open with an unanswered domain = %v", err)
	}
	if got := ImplementedDomains(source); len(got) != 2 || got[0] != DomainLogs || got[1] != DomainEndpoints {
		t.Fatalf("ImplementedDomains = %v", got)
	}
}
