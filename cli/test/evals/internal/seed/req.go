package seed

import (
	"fmt"
	"math"
	"strings"
	"time"
)

// Req builds one request's span tree. Work is laid out on a cursor: every
// child starts where the previous one ended (plus any Gap), and a span's
// duration is where its cursor stopped, so a parent always covers its
// children exactly.
type Req struct {
	s           *Scenario
	req         *Request
	cur         *Span
	cursor      time.Duration
	status      int
	forceDetail bool
	svc         *Service      // service owning cur (nil = the request's)
	base        time.Duration // offset of cur's start from the request's start
}

// Request returns the request being built.
func (r *Req) Request() *Request { return r.req }

// At is the absolute time of the cursor.
func (r *Req) At() time.Time { return r.req.Start.Add(r.base + r.cursor) }

// Start is the request's start time.
func (r *Req) Start() time.Time { return r.req.Start }

// Progress is how far through the traffic window the request falls (0..1).
func (r *Req) Progress() float64 { return r.req.Progress }

// Host is the instance serving the request.
func (r *Req) Host() string { return r.req.Host }

// Version is the service.version live when the request started.
func (r *Req) Version() string { return r.req.Version }

// Float returns a uniform [0,1) draw.
func (r *Req) Float() float64 { return r.s.RNG.Float64() }

// Chance reports true with probability p.
func (r *Req) Chance(p float64) bool { return r.s.RNG.Float64() < p }

// IntN returns a uniform int in [0,n).
func (r *Req) IntN(n int) int { return r.s.RNG.IntN(n) }

// Ms returns a uniform duration between lo and hi milliseconds.
func (r *Req) Ms(lo, hi float64) time.Duration {
	return time.Duration((lo + (hi-lo)*r.s.RNG.Float64()) * float64(time.Millisecond))
}

// LogNormal draws a duration with the given median and p99.
func (r *Req) LogNormal(median, p99 time.Duration) time.Duration {
	mu := math.Log(float64(median))
	sigma := (math.Log(float64(p99)) - mu) / 2.326
	return time.Duration(math.Exp(mu + sigma*r.s.RNG.NormFloat64()))
}

// Growth interpolates from lo at the window start to hi at its end, with
// ±10% noise, as an integer.
func (r *Req) Growth(lo, hi float64) int {
	v := lo + (hi-lo)*r.req.Progress
	v *= 0.9 + 0.2*r.s.RNG.Float64()
	return int(math.Round(math.Max(0, v)))
}

// Lerp interpolates from lo to hi by window progress, without noise.
func (r *Req) Lerp(lo, hi float64) float64 { return lo + (hi-lo)*r.req.Progress }

// Status sets the response status.
func (r *Req) Status(code int) { r.status = code }

// KeepDetail forces this request's full span tree to be sent.
func (r *Req) KeepDetail() { r.forceDetail = true }

// Attr sets an attribute on the current span.
func (r *Req) Attr(k string, v any) {
	r.cur.Attrs[k] = v
	if s, ok := v.(string); ok {
		r.s.record(s)
	}
}

// RootAttr sets an attribute on the request's root span.
func (r *Req) RootAttr(k string, v any) {
	r.req.Root.Attrs[k] = v
	if s, ok := v.(string); ok {
		r.s.record(s)
	}
}

// Gap advances the cursor without a span: time the request spent waiting.
func (r *Req) Gap(d time.Duration) { r.cursor += d }

func (r *Req) owner() *Service {
	if r.svc != nil {
		return r.svc
	}
	return r.req.e.svc
}

func (r *Req) child(name string, kind int, scope string, d time.Duration, attrs map[string]any) *Span {
	sp := &Span{Name: name, Kind: kind, Start: r.cursor, Dur: d, Attrs: attrs, Scope: scope}
	fillRandom(r.s, sp.SpanID[:])
	r.cur.Children = append(r.cur.Children, sp)
	r.cursor += d
	r.s.record(name)
	for _, v := range attrs {
		if s, ok := v.(string); ok {
			r.s.record(s)
		}
	}
	return sp
}

// Compute records in-process work as an INTERNAL span.
func (r *Req) Compute(name string, d time.Duration, attrs ...KV) *Span {
	return r.child(name, KindInternal, r.owner().Stack.ServerScope, d, kvMap(attrs))
}

// Span records an INTERNAL span whose children are built by f; its duration
// is whatever f lays out.
func (r *Req) Span(name string, f func(r *Req), attrs ...KV) *Span {
	sp := &Span{Name: name, Kind: KindInternal, Start: r.cursor, Attrs: kvMap(attrs), Scope: r.owner().Stack.ServerScope}
	fillRandom(r.s, sp.SpanID[:])
	r.cur.Children = append(r.cur.Children, sp)
	r.s.record(name)
	sub := &Req{s: r.s, req: r.req, cur: sp, cursor: 0, svc: r.svc, base: r.base + sp.Start}
	f(sub)
	sp.Dur = sub.cursor
	r.cursor += sp.Dur
	r.forceDetail = r.forceDetail || sub.forceDetail
	if sub.status != 0 {
		r.status = sub.status
	}
	return sp
}

// Parallel runs branches concurrently from the current cursor; the cursor
// then moves to the end of the longest branch.
func (r *Req) Parallel(branches ...func(r *Req)) {
	start := r.cursor
	longest := start
	for _, b := range branches {
		sub := &Req{s: r.s, req: r.req, cur: r.cur, cursor: start, svc: r.svc, base: r.base}
		b(sub)
		if sub.cursor > longest {
			longest = sub.cursor
		}
		r.forceDetail = r.forceDetail || sub.forceDetail
		if sub.status != 0 {
			r.status = sub.status
		}
	}
	r.cursor = longest
}

// KV is an attribute pair.
type KV struct {
	K string
	V any
}

// A is shorthand for a KV.
func A(k string, v any) KV { return KV{k, v} }

func kvMap(kvs []KV) map[string]any {
	m := map[string]any{}
	for _, kv := range kvs {
		m[kv.K] = kv.V
	}
	return m
}

// DB records a database CLIENT span for query. The span name follows the
// semantic conventions ("{operation} {collection}").
func (r *Req) DB(query string, d time.Duration, attrs ...KV) *Span {
	st := r.owner().Stack
	op, coll := sqlParts(query)
	name := op
	if coll != "" {
		name = op + " " + coll
	}
	m := map[string]any{
		"db.system.name":    st.DBSystem,
		"db.query.text":     query,
		"db.namespace":      r.owner().dbName(),
		"db.operation.name": op,
		"server.address":    r.owner().dbHost(),
		"server.port":       5432,
	}
	if st.DBSystem == "mysql" {
		m["server.port"] = 3306
	}
	if coll != "" {
		m["db.collection.name"] = coll
	}
	for _, kv := range attrs {
		m[kv.K] = kv.V
	}
	return r.child(name, KindClient, st.DBScope, d, m)
}

// Rows is the db.response.returned_rows attribute.
func Rows(n int) KV { return KV{"db.response.returned_rows", n} }

func sqlParts(q string) (op, coll string) {
	fields := strings.Fields(q)
	if len(fields) == 0 {
		return "QUERY", ""
	}
	op = strings.ToUpper(fields[0])
	for i, f := range fields {
		u := strings.ToUpper(f)
		if (u == "FROM" || u == "INTO" || (u == "UPDATE" && i == 0)) && i+1 < len(fields) {
			coll = strings.Trim(fields[i+1], "(),;")
			break
		}
	}
	return op, strings.ToLower(coll)
}

func (svc *Service) dbName() string {
	return strings.ReplaceAll(strings.TrimSuffix(strings.TrimSuffix(svc.Name, "-api"), "-service"), "-", "_")
}

func (svc *Service) dbHost() string {
	return strings.ReplaceAll(svc.dbName(), "_", "-") + "-db.internal"
}

// Cache records a cache CLIENT span (redis).
func (r *Req) Cache(cmd, key string, d time.Duration, attrs ...KV) *Span {
	m := map[string]any{
		"db.system.name":    "redis",
		"db.operation.name": cmd,
		"db.query.text":     cmd + " " + key,
		"server.address":    "cache.internal",
		"server.port":       6379,
	}
	for _, kv := range attrs {
		m[kv.K] = kv.V
	}
	return r.child(cmd, KindClient, r.owner().Stack.CacheScope, d, m)
}

// HTTP records an outgoing HTTP CLIENT span. The span name is the method,
// as the semantic conventions specify for client spans.
func (r *Req) HTTP(method, url string, d time.Duration, status int, attrs ...KV) *Span {
	host, port := hostOf(url)
	m := map[string]any{
		"http.request.method":       method,
		"url.full":                  url,
		"server.address":            host,
		"server.port":               port,
		"http.response.status_code": status,
	}
	for _, kv := range attrs {
		m[kv.K] = kv.V
	}
	sp := r.child(method, KindClient, r.owner().Stack.HTTPClient, d, m)
	// Client spans count a 4xx as an error too (HTTP semantic conventions).
	if status >= 400 {
		sp.Status = StatusError
	}
	return sp
}

func hostOf(u string) (string, int) {
	rest := u
	port := 443
	if strings.HasPrefix(rest, "http://") {
		rest, port = strings.TrimPrefix(rest, "http://"), 80
	}
	rest = strings.TrimPrefix(rest, "https://")
	if i := strings.IndexAny(rest, "/?"); i >= 0 {
		rest = rest[:i]
	}
	if h, p, ok := strings.Cut(rest, ":"); ok {
		var n int
		fmt.Sscanf(p, "%d", &n)
		return h, n
	}
	return rest, port
}

// Call makes an HTTP call to another service's endpoint: a CLIENT span here
// and, under it, the downstream SERVER span built by the target's handler.
// rtt is the network round trip added around the server time.
func (r *Req) Call(target *Endpoint, rtt time.Duration) *Span {
	tsvc := target.svc
	url := fmt.Sprintf("http://%s.internal:8080%s", tsvc.Name, concretePath(r.s, target.Route))
	client := &Span{Name: target.Method, Kind: KindClient, Start: r.cursor, Scope: r.owner().Stack.HTTPClient,
		Attrs: map[string]any{"http.request.method": target.Method, "url.full": url, "server.address": tsvc.Name + ".internal", "server.port": 8080}}
	fillRandom(r.s, client.SpanID[:])
	r.cur.Children = append(r.cur.Children, client)

	server := &Span{Name: target.Name(), Kind: KindServer, Start: rtt / 2, Scope: tsvc.Stack.ServerScope, Service: tsvc, Host: tsvc.pickHost(),
		Attrs: map[string]any{"http.request.method": target.Method, "http.route": target.Route, "url.path": concretePath(r.s, target.Route), "url.scheme": "http"}}
	fillRandom(r.s, server.SpanID[:])
	client.Children = append(client.Children, server)
	sub := &Req{s: r.s, req: r.req, cur: server, svc: tsvc, base: r.base + client.Start + server.Start}
	if target.handler != nil {
		target.handler(sub)
	}
	server.Dur = sub.cursor
	status := 200
	if sub.status != 0 {
		status = sub.status
	}
	server.Attrs["http.response.status_code"] = status
	client.Attrs["http.response.status_code"] = status
	if status >= 500 {
		server.Status, client.Status = StatusError, StatusError
	}
	client.Dur = server.Dur + rtt
	r.cursor += client.Dur
	r.forceDetail = r.forceDetail || sub.forceDetail
	return client
}

// Log emits a log record correlated with the current span at the cursor.
// Logged requests are always sent in detail so the trace id resolves.
func (r *Req) Log(severity int, body string, attrs ...KV) {
	r.cur.logs = append(r.cur.logs, logRecord{at: r.cursor, severity: severity, body: body, attrs: kvMap(attrs)})
	r.forceDetail = true
	r.s.record(body)
	for _, kv := range attrs {
		if s, ok := kv.V.(string); ok {
			r.s.record(s)
		}
	}
}

// Exc is an exception to record on a span.
type Exc struct {
	Type    string
	Message string
	Stack   string
	// Escaped marks exception.escaped.
	Escaped bool
}

// Error records an exception event on the current span at the cursor and
// marks the span (and, for escaped errors, the request) as failed.
func (r *Req) Error(e Exc) {
	attrs := map[string]any{"exception.type": e.Type, "exception.message": e.Message}
	if e.Stack != "" {
		attrs["exception.stacktrace"] = e.Stack
	}
	if e.Escaped {
		attrs["exception.escaped"] = true
	}
	r.cur.Events = append(r.cur.Events, Event{Name: "exception", At: r.cursor, Attrs: attrs})
	r.cur.Status = StatusError
	r.cur.StatusMsg = e.Message
	r.forceDetail = true
	r.s.record(e.Type, e.Message, e.Stack)
}

func (r *Req) finish() {
	r.req.Root.Dur = r.cursor
	if r.req.Root.Dur <= 0 {
		r.req.Root.Dur = time.Millisecond
	}
}
