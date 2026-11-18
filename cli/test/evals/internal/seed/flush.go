package seed

import (
	"fmt"
	"sort"
	"time"

	coltracepb "go.opentelemetry.io/proto/otlp/collector/trace/v1"
	"google.golang.org/protobuf/proto"
)

const (
	pathTraces  = "/api/otel/v1/traces"
	pathMetrics = "/api/otel/v1/metrics"
	pathLogs    = "/api/otel/v1/logs"
	pathReport  = "/api/report"
	protoType   = "application/x-protobuf"
	// spansPerRequest keeps each OTLP body far below the 10MB ingest cap.
	spansPerRequest = 1500
)

// Flush generates every request, resolves lazy exposes, enforces the span
// budget and sends everything: traces, logs, metrics, native SDK frames.
func (s *Scenario) Flush() error {
	if s.flushed {
		return fmt.Errorf("scenario flushed twice")
	}
	s.flushed = true
	started := time.Now()

	for _, svc := range s.services {
		applyDefaults(svc)
	}
	var all []*Request
	for _, svc := range s.services {
		for _, e := range append(append([]*Endpoint{}, svc.endpoints...), svc.tasks...) {
			if len(e.shapes) == 0 {
				continue // only reached through Call
			}
			all = append(all, e.Requests()...)
		}
	}
	listed := s.listedRequests(all)
	keys := make([]string, 0, len(s.lazy))
	for k := range s.lazy {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		s.exposed[k] = s.lazy[k]()
	}
	s.enforceBudget(all)
	for _, r := range listed {
		if !r.Detail {
			r.Detail, r.listed = true, true
		}
	}
	sort.SliceStable(all, func(i, j int) bool { return all[i].Start.Before(all[j].Start) })

	batches := map[*Project]*spanBatch{}
	logs := map[*Service][]logOut{}
	for _, r := range all {
		s.stats.Requests++
		if r.Detail {
			s.stats.DetailedReqs++
		}
		s.emit(r, batches, logs)
		for p, b := range batches {
			if b.count >= spansPerRequest {
				if err := s.sendTraces(p, b); err != nil {
					return err
				}
				delete(batches, p)
			}
		}
	}
	projects := make([]*Project, 0, len(batches))
	for p := range batches {
		projects = append(projects, p)
	}
	sort.Slice(projects, func(i, j int) bool { return projects[i].Name < projects[j].Name })
	for _, p := range projects {
		if err := s.sendTraces(p, batches[p]); err != nil {
			return err
		}
	}

	for _, svc := range s.services {
		outs := logs[svc]
		for _, l := range svc.logs {
			outs = append(outs, logOut{rec: l, at: l.abs, host: l.host, version: svc.VersionAt(l.abs), noTrace: true})
		}
		if len(outs) == 0 {
			continue
		}
		sort.SliceStable(outs, func(i, j int) bool { return outs[i].at.Before(outs[j].at) })
		for i := 0; i < len(outs); i += 2000 {
			j := min(i+2000, len(outs))
			body, err := proto.Marshal(logsRequest(svc, outs[i:j]))
			if err != nil {
				return err
			}
			if _, err := s.Sink.Ingest(svc.p.token, pathLogs, protoType, body, true); err != nil {
				return fmt.Errorf("logs for %s: %w", svc.Name, err)
			}
			s.stats.Logs += j - i
		}
	}

	for _, svc := range s.services {
		for _, req := range s.metricRequests(svc) {
			body, err := proto.Marshal(req)
			if err != nil {
				return err
			}
			if _, err := s.Sink.Ingest(svc.p.token, pathMetrics, protoType, body, true); err != nil {
				return fmt.Errorf("metrics for %s: %w", svc.Name, err)
			}
		}
	}

	recordings := 0
	for _, p := range s.projects {
		for _, f := range p.native {
			recordings += len(f.recordings)
		}
		bodies, err := p.nativeBodies()
		if err != nil {
			return err
		}
		for _, body := range bodies {
			if _, err := s.Sink.Ingest(p.token, pathReport, "application/json", body, true); err != nil {
				return fmt.Errorf("native report for %s: %w", p.Name, err)
			}
			s.stats.NativeFrames++
		}
	}

	if recordings > 0 && !s.Sink.Dry() {
		// Session recordings are uploaded by a worker pool and inserted in
		// batches every ~2s; wait so occurrences show their actions.
		time.Sleep(3 * time.Second)
	}
	for _, f := range s.afterFlush {
		if err := f(); err != nil {
			return err
		}
	}
	s.stats.IngestElapsed = time.Since(started)
	return nil
}

func (s *Scenario) sendTraces(p *Project, b *spanBatch) error {
	body, err := proto.Marshal(b.request())
	if err != nil {
		return err
	}
	resp, err := s.Sink.Ingest(p.token, pathTraces, protoType, body, true)
	if err != nil {
		return fmt.Errorf("traces for %s: %w", p.Name, err)
	}
	if len(resp) > 0 {
		var out coltracepb.ExportTraceServiceResponse
		if proto.Unmarshal(resp, &out) == nil && out.GetPartialSuccess().GetRejectedSpans() > 0 {
			return fmt.Errorf("traces for %s: backend rejected %d spans: %s", p.Name, out.GetPartialSuccess().GetRejectedSpans(), out.GetPartialSuccess().GetErrorMessage())
		}
	}
	return nil
}

func spanCount(sp *Span) int {
	n := 1
	for _, c := range sp.Children {
		n += spanCount(c)
	}
	return n
}

func serverSpans(sp *Span) int {
	n := 0
	if sp.Kind == KindServer {
		n = 1
	}
	for _, c := range sp.Children {
		n += serverSpans(c)
	}
	return n
}

// How many requests from the top of a request listing get their full span
// tree. Every trailing window keeps its listedSlowest slowest (the default
// order) and its listedOthers fastest and newest; every UTC day, for
// --from/--to bisection, its listedPerDay slowest. Verify checks half of the
// trailing ones, which leaves room for the requests that age out of a window
// between the seed and the agent's query.
const (
	listedSlowest = 10
	listedOthers  = 4
	listedPerDay  = 3
)

var listingWindows = []time.Duration{time.Hour, 6 * time.Hour, 24 * time.Hour, 3 * 24 * time.Hour, 7 * 24 * time.Hour, 30 * 24 * time.Hour}

// listingDrift is where a listing's window may end relative to s.Now: before
// it in a scenario that plays out in real time (its telemetry runs ahead of
// the clock), after it by up to a run's length. The hour is taken at each
// end, since a few minutes move the top of a busy hour; longer windows barely
// move.
var listingDrift = []time.Duration{-10 * time.Minute, -5 * time.Minute, 0, 5 * time.Minute, 10 * time.Minute, 20 * time.Minute}

type listedRow struct {
	r     *Request
	start time.Time
	dur   time.Duration
}

// listedRequests returns the requests a request listing (endpoints requests,
// list_endpoint_requests) shows first, which Flush sends with their full span
// tree. The random detail sample almost never holds the slowest requests, so
// an agent that opened one used to find no spans at all. Flush adds them after
// enforceBudget, without an access log line, and this reads no random
// numbers, so the sample and everything else generated stay the same.
func (s *Scenario) listedRequests(all []*Request) []*Request {
	type key struct {
		p    *Project
		name string
	}
	groups := map[key][]listedRow{}
	// Only endpoints with traffic of their own: one reached only through
	// Call is seen in its caller's waterfall, and covering its listing would
	// send most of the callers' trees.
	listable := map[key]bool{}
	for _, svc := range s.services {
		for _, e := range svc.endpoints {
			if len(e.shapes) > 0 {
				listable[key{svc.p, e.Name()}] = true
			}
		}
	}
	s.rows = map[string]*Request{}
	for _, r := range all {
		var walk func(sp *Span, svc *Service, abs time.Time)
		walk = func(sp *Span, svc *Service, abs time.Time) {
			if sp.Service != nil {
				svc = sp.Service
			}
			start := abs.Add(sp.Start)
			if sp.Kind == KindServer {
				k := key{svc.p, sp.Name}
				groups[k] = append(groups[k], listedRow{r, start, sp.Dur})
				s.rows[EntityID(svc.p.ID, r.TraceID, sp.SpanID)] = r
			}
			for _, c := range sp.Children {
				walk(c, svc, start)
			}
		}
		walk(r.Root, r.e.svc, r.Start)
	}
	slowest := func(a, b listedRow) bool { return a.dur > b.dur }
	fastest := func(a, b listedRow) bool { return a.dur < b.dur }
	newest := func(a, b listedRow) bool { return a.start.After(b.start) }
	picked := map[*Request]bool{}
	var listed []*Request
	keep := func(rows []listedRow, from, to time.Time, n int, less func(a, b listedRow) bool) {
		var in []listedRow
		for _, x := range rows {
			if !x.start.Before(from) && x.start.Before(to) {
				in = append(in, x)
			}
		}
		sort.SliceStable(in, func(i, j int) bool { return less(in[i], in[j]) })
		for _, x := range in[:min(n, len(in))] {
			if !picked[x.r] {
				picked[x.r] = true
				listed = append(listed, x.r)
			}
		}
	}
	for k, rows := range groups {
		if !listable[k] {
			continue
		}
		for _, d := range listingWindows {
			ends := []time.Duration{0}
			if d == time.Hour {
				ends = listingDrift
			}
			for _, off := range ends {
				to := s.Now.Add(off)
				keep(rows, to.Add(-d), to, listedSlowest, slowest)
				keep(rows, to.Add(-d), to, listedOthers, fastest)
				keep(rows, to.Add(-d), to, listedOthers, newest)
			}
		}
		earliest := rows[0].start
		for _, x := range rows {
			if x.start.Before(earliest) {
				earliest = x.start
			}
		}
		for day := earliest.UTC().Truncate(24 * time.Hour); day.Before(s.Now); day = day.Add(24 * time.Hour) {
			keep(rows, day, day.Add(24*time.Hour), listedPerDay, slowest)
		}
	}
	return listed
}

// enforceBudget keeps the detailed span volume under SpanBudget by dropping
// sampled (not forced) detail, oldest first.
func (s *Scenario) enforceBudget(all []*Request) {
	total := 0
	for _, r := range all {
		total += s.sentSpans(r)
	}
	if total <= s.SpanBudget {
		return
	}
	sampled := make([]*Request, 0)
	for _, r := range all {
		if r.Detail && !r.forced {
			sampled = append(sampled, r)
		}
	}
	sort.SliceStable(sampled, func(i, j int) bool { return sampled[i].Start.Before(sampled[j].Start) })
	for _, r := range sampled {
		if total <= s.SpanBudget {
			return
		}
		before := s.sentSpans(r)
		r.Detail = false
		total -= before - s.sentSpans(r)
	}
}

func (s *Scenario) sentSpans(r *Request) int {
	if r.Detail {
		return spanCount(r.Root)
	}
	return 1 + serverSpans(r.Root) - boolInt(r.Root.Kind == KindServer)
}

func boolInt(b bool) int {
	if b {
		return 1
	}
	return 0
}
