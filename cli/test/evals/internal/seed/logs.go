package seed

import (
	"sort"
	"time"

	collogspb "go.opentelemetry.io/proto/otlp/collector/logs/v1"
	commonpb "go.opentelemetry.io/proto/otlp/common/v1"
	logspb "go.opentelemetry.io/proto/otlp/logs/v1"
)

type logRecord struct {
	at       time.Duration // offset within the owning span
	abs      time.Time     // absolute time, for logs outside a request
	host     string
	severity int
	body     string
	attrs    map[string]any
	scope    string
}

type logOut struct {
	rec     logRecord
	at      time.Time
	traceID [16]byte
	spanID  [8]byte
	host    string
	version string
	noTrace bool
}

// Log emits a log line from the service that is not tied to a request (a
// startup message, a scheduler line, a pool warning without a trace).
func (svc *Service) Log(at time.Time, severity int, body string, attrs ...KV) {
	svc.LogOn(svc.Hosts[0], at, severity, body, attrs...)
}

// LogOn is Log from a specific instance.
func (svc *Service) LogOn(host string, at time.Time, severity int, body string, attrs ...KV) {
	svc.logs = append(svc.logs, logRecord{abs: at, host: host, severity: severity, body: body, attrs: kvMap(attrs)})
	svc.s.record(body)
	for _, kv := range attrs {
		if s, ok := kv.V.(string); ok {
			svc.s.record(s)
		}
	}
}

func logScope(svc *Service) string {
	switch svc.Stack.Language {
	case "python":
		return "opentelemetry.sdk._logs"
	case "nodejs":
		return "@opentelemetry/winston-transport"
	case "java":
		return "io.opentelemetry.logback-appender-1.0"
	case "dotnet":
		return "Microsoft.Extensions.Logging"
	case "php":
		return "io.opentelemetry.contrib.php.monolog"
	default:
		return "go.opentelemetry.io/contrib/bridges/otelslog"
	}
}

// logsRequest renders a service's logs as one OTLP request per resource.
func logsRequest(svc *Service, outs []logOut) *collogspb.ExportLogsServiceRequest {
	type key struct{ version, host string }
	groups := map[key][]*logspb.LogRecord{}
	var keys []key
	for _, o := range outs {
		k := key{o.version, o.host}
		if _, ok := groups[k]; !ok {
			keys = append(keys, k)
		}
		lr := &logspb.LogRecord{
			TimeUnixNano:         nanos(o.at),
			ObservedTimeUnixNano: nanos(o.at.Add(3 * time.Millisecond)),
			SeverityNumber:       logspb.SeverityNumber(o.rec.severity),
			SeverityText:         SeverityText(o.rec.severity),
			Body:                 &commonpb.AnyValue{Value: &commonpb.AnyValue_StringValue{StringValue: o.rec.body}},
			Attributes:           attrList(o.rec.attrs),
		}
		if !o.noTrace {
			lr.TraceId = append([]byte{}, o.traceID[:]...)
			lr.SpanId = append([]byte{}, o.spanID[:]...)
			lr.Flags = 1
		}
		groups[k] = append(groups[k], lr)
	}
	sort.Slice(keys, func(i, j int) bool {
		if keys[i].version != keys[j].version {
			return keys[i].version < keys[j].version
		}
		return keys[i].host < keys[j].host
	})
	req := &collogspb.ExportLogsServiceRequest{}
	for _, k := range keys {
		recs := groups[k]
		sort.SliceStable(recs, func(i, j int) bool { return recs[i].TimeUnixNano < recs[j].TimeUnixNano })
		req.ResourceLogs = append(req.ResourceLogs, &logspb.ResourceLogs{
			Resource: svc.resource(k.version, k.host),
			ScopeLogs: []*logspb.ScopeLogs{{
				Scope:      &commonpb.InstrumentationScope{Name: logScope(svc)},
				LogRecords: recs,
			}},
		})
	}
	return req
}
