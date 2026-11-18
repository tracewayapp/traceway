package seed

import (
	"fmt"
	"sort"
	"time"

	"github.com/google/uuid"
	coltracepb "go.opentelemetry.io/proto/otlp/collector/trace/v1"
	commonpb "go.opentelemetry.io/proto/otlp/common/v1"
	resourcepb "go.opentelemetry.io/proto/otlp/resource/v1"
	tracepb "go.opentelemetry.io/proto/otlp/trace/v1"
)

// EntityID is the id the backend gives the endpoint/task/exception row
// promoted from a span: UUIDv5(projectId, traceId || spanId).
func EntityID(projectID string, traceID [16]byte, spanID [8]byte) string {
	pid, err := uuid.Parse(projectID)
	if err != nil {
		return ""
	}
	var identity [24]byte
	copy(identity[:16], traceID[:])
	copy(identity[16:], spanID[:])
	return uuid.NewSHA1(pid, identity[:]).String()
}

// resourceKey groups spans that share one OTLP Resource.
type resourceKey struct {
	svc     *Service
	version string
	host    string
}

// spanBatch accumulates the spans of one project, grouped by resource.
type spanBatch struct {
	project *Project
	groups  map[resourceKey]map[string][]*tracepb.Span // scope -> spans
	count   int
}

func newSpanBatch(p *Project) *spanBatch {
	return &spanBatch{project: p, groups: map[resourceKey]map[string][]*tracepb.Span{}}
}

func (b *spanBatch) add(k resourceKey, scope string, sp *tracepb.Span) {
	g := b.groups[k]
	if g == nil {
		g = map[string][]*tracepb.Span{}
		b.groups[k] = g
	}
	g[scope] = append(g[scope], sp)
	b.count++
}

func (b *spanBatch) request() *coltracepb.ExportTraceServiceRequest {
	keys := make([]resourceKey, 0, len(b.groups))
	for k := range b.groups {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		if keys[i].svc.Name != keys[j].svc.Name {
			return keys[i].svc.Name < keys[j].svc.Name
		}
		if keys[i].version != keys[j].version {
			return keys[i].version < keys[j].version
		}
		return keys[i].host < keys[j].host
	})
	req := &coltracepb.ExportTraceServiceRequest{}
	for _, k := range keys {
		rs := &tracepb.ResourceSpans{Resource: k.svc.resource(k.version, k.host)}
		scopes := make([]string, 0, len(b.groups[k]))
		for sc := range b.groups[k] {
			scopes = append(scopes, sc)
		}
		sort.Strings(scopes)
		for _, sc := range scopes {
			rs.ScopeSpans = append(rs.ScopeSpans, &tracepb.ScopeSpans{
				Scope: &commonpb.InstrumentationScope{Name: sc, Version: scopeVersion(sc)},
				Spans: b.groups[k][sc],
			})
		}
		req.ResourceSpans = append(req.ResourceSpans, rs)
	}
	return req
}

func scopeVersion(scope string) string {
	switch {
	case len(scope) > 0 && scope[0] == '@':
		return "0.57.0"
	case len(scope) > 3 && scope[:3] == "io.":
		return "2.12.0"
	default:
		return "0.63.0"
	}
}

// resource builds the OTLP Resource of a service instance.
func (svc *Service) resource(version, host string) *resourcepb.Resource {
	attrs := []*commonpb.KeyValue{
		str("service.name", svc.Name),
		str("service.version", version),
		str("deployment.environment.name", svc.Env),
		str("host.name", host),
		str("telemetry.sdk.language", svc.Stack.Language),
		str("telemetry.sdk.name", "opentelemetry"),
		str("telemetry.sdk.version", sdkVersion(svc.Stack.Language)),
	}
	return &resourcepb.Resource{Attributes: attrs}
}

func sdkVersion(lang string) string {
	switch lang {
	case "go":
		return "1.38.0"
	case "python":
		return "1.37.0"
	case "nodejs":
		return "2.1.0"
	case "java":
		return "1.55.0"
	case "dotnet":
		return "1.13.1"
	default:
		return "1.0.0"
	}
}

func str(k, v string) *commonpb.KeyValue {
	return &commonpb.KeyValue{Key: k, Value: &commonpb.AnyValue{Value: &commonpb.AnyValue_StringValue{StringValue: v}}}
}

// attrList converts a map to OTLP attributes. Integers become intValue, as the
// backend reads numeric semantic-convention attributes only from intValue.
func attrList(m map[string]any) []*commonpb.KeyValue {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	out := make([]*commonpb.KeyValue, 0, len(keys))
	for _, k := range keys {
		out = append(out, &commonpb.KeyValue{Key: k, Value: anyValue(m[k])})
	}
	return out
}

func anyValue(v any) *commonpb.AnyValue {
	switch t := v.(type) {
	case string:
		return &commonpb.AnyValue{Value: &commonpb.AnyValue_StringValue{StringValue: t}}
	case int:
		return &commonpb.AnyValue{Value: &commonpb.AnyValue_IntValue{IntValue: int64(t)}}
	case int64:
		return &commonpb.AnyValue{Value: &commonpb.AnyValue_IntValue{IntValue: t}}
	case float64:
		return &commonpb.AnyValue{Value: &commonpb.AnyValue_DoubleValue{DoubleValue: t}}
	case bool:
		return &commonpb.AnyValue{Value: &commonpb.AnyValue_BoolValue{BoolValue: t}}
	case []string:
		vals := make([]*commonpb.AnyValue, 0, len(t))
		for _, s := range t {
			vals = append(vals, anyValue(s))
		}
		return &commonpb.AnyValue{Value: &commonpb.AnyValue_ArrayValue{ArrayValue: &commonpb.ArrayValue{Values: vals}}}
	default:
		return &commonpb.AnyValue{Value: &commonpb.AnyValue_StringValue{StringValue: fmt.Sprint(t)}}
	}
}

func nanos(t time.Time) uint64 { return uint64(t.UnixNano()) }

// emit walks a request tree and adds its spans to per-project batches. A
// non-detailed request sends only its root and every downstream SERVER span,
// so endpoint rows and percentiles stay complete while the span volume stays
// bounded.
func (s *Scenario) emit(r *Request, batches map[*Project]*spanBatch, logs map[*Service][]logOut) {
	rootSvc := r.e.svc
	var walk func(sp *Span, parent *Span, svc *Service, host string, abs time.Time, depthDetail bool)
	walk = func(sp *Span, parent *Span, svc *Service, host string, abs time.Time, detail bool) {
		if sp.Service != nil {
			svc = sp.Service
		}
		if sp.Host != "" {
			host = sp.Host
		}
		start := abs.Add(sp.Start)
		send := detail || parent == nil || sp.Kind == KindServer
		if send {
			pb := &tracepb.Span{
				TraceId:           r.TraceID[:],
				SpanId:            sp.SpanID[:],
				Name:              sp.Name,
				Kind:              tracepb.Span_SpanKind(sp.Kind),
				StartTimeUnixNano: nanos(start),
				EndTimeUnixNano:   nanos(start.Add(sp.Dur)),
				Attributes:        attrList(sp.Attrs),
				Status:            &tracepb.Status{Code: tracepb.Status_StatusCode(sp.Status), Message: sp.StatusMsg},
			}
			if parent != nil {
				pb.ParentSpanId = parent.SpanID[:]
			}
			for _, ev := range sp.Events {
				pb.Events = append(pb.Events, &tracepb.Span_Event{
					TimeUnixNano: nanos(start.Add(ev.At)),
					Name:         ev.Name,
					Attributes:   attrList(ev.Attrs),
				})
			}
			b := batches[svc.p]
			if b == nil {
				b = newSpanBatch(svc.p)
				batches[svc.p] = b
			}
			b.add(resourceKey{svc: svc, version: svc.VersionAt(r.Start), host: host}, sp.Scope, pb)
			s.stats.Spans++
			if len(sp.Events) > 0 {
				s.stats.Exceptions += len(sp.Events)
			}
		}
		if detail {
			for _, l := range sp.logs {
				logs[svc] = append(logs[svc], logOut{rec: l, at: start.Add(l.at), traceID: r.TraceID, spanID: sp.SpanID, host: host, version: svc.VersionAt(r.Start)})
			}
		}
		for _, c := range sp.Children {
			walk(c, sp, svc, host, start, detail)
		}
	}
	walk(r.Root, nil, rootSvc, r.Host, r.Start, r.Detail)
	if r.Detail && !r.listed && !r.e.Task && !r.e.SkipAccessLog {
		// The access log line a request logger writes on completion,
		// correlated with the trace like any OTel log bridge does.
		end := r.Start.Add(r.Root.Dur)
		path, _ := r.Root.Attrs["url.path"].(string)
		body := fmt.Sprintf("%s %s %d %dms", r.e.Method, path, r.Status, r.Root.Dur.Milliseconds())
		logs[rootSvc] = append(logs[rootSvc], logOut{
			rec: logRecord{severity: SevInfo, body: body, attrs: map[string]any{
				"http.request.method": r.e.Method, "url.path": path, "http.route": r.e.Route,
				"http.response.status_code": r.Status, "duration_ms": r.Root.Dur.Milliseconds(),
			}},
			at: end, traceID: r.TraceID, spanID: r.SpanID, host: r.Host, version: r.Version,
		})
	}
}
