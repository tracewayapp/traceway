// Package seed builds realistic Traceway telemetry for one eval scenario and
// sends it through the real ingest endpoints (OTLP/HTTP and native
// /api/report). The DSL owns consistency: a request's duration is the sum of
// its children plus explicit gaps, percentiles come from the generated
// requests, and deploys are service.version changes on the resource.
package seed

import (
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"math"
	"math/rand/v2"
	"sort"
	"time"
)

// ProjectKind selects how a project's telemetry arrives.
type ProjectKind int

const (
	// OTel is a backend project fed over OTLP (framework opentelemetry).
	OTel ProjectKind = iota
	// Browser is a frontend project fed by a Traceway browser SDK.
	Browser
	// Mobile is a mobile project fed by a Traceway mobile SDK.
	Mobile
)

// Scenario is the state of one seed run.
type Scenario struct {
	CaseID string
	Now    time.Time
	RNG    *rand.Rand
	Sink   Sink

	projects []*Project
	services []*Service
	exposed  map[string]any
	lazy     map[string]func() any
	strings  []string
	flushed  bool
	stats    Stats
	// SpanBudget caps the spans sent with full detail (default 20000).
	SpanBudget int
	afterFlush []func() error
	// rows maps each endpoint row's id to the request that holds its span.
	rows map[string]*Request
}

// Stats summarizes what a seed sent.
type Stats struct {
	Requests      int           `json:"requests"`
	Spans         int           `json:"spans"`
	DetailedReqs  int           `json:"detailedRequests"`
	Logs          int           `json:"logs"`
	MetricPoints  int           `json:"metricPoints"`
	Exceptions    int           `json:"exceptions"`
	NativeFrames  int           `json:"nativeFrames"`
	IngestElapsed time.Duration `json:"ingestElapsed"`
}

// NewScenario starts a scenario anchored at now (truncated to the minute)
// with an RNG seeded from the case id, so a case always draws the same data.
func NewScenario(caseID string, now time.Time, sink Sink) *Scenario {
	sum := sha256.Sum256([]byte(caseID))
	seed1 := binary.BigEndian.Uint64(sum[:8])
	seed2 := binary.BigEndian.Uint64(sum[8:16])
	return &Scenario{
		CaseID:     caseID,
		Now:        now.UTC().Truncate(time.Minute),
		RNG:        rand.New(rand.NewPCG(seed1, seed2)),
		Sink:       sink,
		exposed:    map[string]any{},
		lazy:       map[string]func() any{},
		SpanBudget: 20000,
	}
}

// Stats returns what was sent.
func (s *Scenario) Stats() Stats { return s.stats }

// Strings returns every human-readable string the seed emitted (span names,
// attribute values, log bodies, stack traces, object names), for the leakage
// lint.
func (s *Scenario) Strings() []string { return s.strings }

func (s *Scenario) record(v ...string) {
	for _, x := range v {
		if x != "" {
			s.strings = append(s.strings, x)
		}
	}
}

// Project declares a Traceway project. framework defaults per kind.
func (s *Scenario) Project(name string, kind ProjectKind, framework ...string) *Project {
	fw := "opentelemetry"
	switch kind {
	case Browser:
		fw = "react"
	case Mobile:
		fw = "android"
	}
	if len(framework) > 0 {
		fw = framework[0]
	}
	p := &Project{s: s, Name: name, Kind: kind, Framework: fw}
	id, token, err := s.Sink.CreateProject(name, fw)
	if err != nil {
		panic(fmt.Errorf("create project %s: %w", name, err))
	}
	p.ID, p.token = id, token
	s.projects = append(s.projects, p)
	s.record(name)
	return p
}

// Projects returns the declared projects in order.
func (s *Scenario) Projects() []*Project { return s.projects }

// Expose records a value for seed.json (graders and the oracle read it; the
// agent never sees it). A func() any value is resolved at Flush.
func (s *Scenario) Expose(key string, v any) {
	if f, ok := v.(func() any); ok {
		s.lazy[key] = f
		return
	}
	s.exposed[key] = v
}

// Exposed returns the resolved exposed values.
func (s *Scenario) Exposed() map[string]any {
	out := map[string]any{}
	for k, v := range s.exposed {
		out[k] = v
	}
	return out
}

// AfterFlush schedules work that needs the telemetry in place (for example
// objects that reference an ingested exception).
func (s *Scenario) AfterFlush(f func() error) { s.afterFlush = append(s.afterFlush, f) }

// Ago returns Now minus d.
func (s *Scenario) Ago(d time.Duration) time.Time { return s.Now.Add(-d) }

// Project is one Traceway project.
type Project struct {
	s         *Scenario
	Name      string
	Kind      ProjectKind
	Framework string
	ID        string
	token     string

	native []nativeFrame
}

// Service declares an OTel-instrumented service reporting to the project.
func (p *Project) Service(name string, stack Stack) *Service {
	svc := &Service{s: p.s, p: p, Name: name, Stack: stack, Env: "production", Hosts: []string{name + "-" + hostSuffix(p.s, name)}}
	p.s.services = append(p.s.services, svc)
	p.s.record(name)
	return svc
}

func hostSuffix(s *Scenario, name string) string {
	h := sha256.Sum256([]byte(s.CaseID + name))
	return fmt.Sprintf("%x-%x", h[0:3], h[3:5])
}

// Service is one deployable component.
type Service struct {
	s       *Scenario
	p       *Project
	Name    string
	Stack   Stack
	Env     string
	Hosts   []string
	deploys []Deploy
	// HostWeight biases request placement per host (default uniform).
	HostWeight map[string]float64

	endpoints   []*Endpoint
	tasks       []*Endpoint
	metrics     []*metricSeries
	logs        []logRecord
	defaultHost bool
}

// Deploy is a release: service.version changes at At.
type Deploy struct {
	At      time.Time
	Version string
}

// Project returns the owning project.
func (svc *Service) Project() *Project { return svc.p }

// Instances replaces the host list (multi-instance service).
func (svc *Service) Instances(hosts ...string) *Service {
	svc.Hosts = hosts
	svc.s.record(hosts...)
	return svc
}

// Versions sets the release history: the first version applies from the
// start, each later one from its deploy time.
func (svc *Service) Versions(initial string, deploys ...Deploy) *Service {
	svc.deploys = append([]Deploy{{At: time.Time{}, Version: initial}}, deploys...)
	sort.SliceStable(svc.deploys, func(i, j int) bool { return svc.deploys[i].At.Before(svc.deploys[j].At) })
	return svc
}

// VersionAt returns the service.version live at t.
func (svc *Service) VersionAt(t time.Time) string {
	if len(svc.deploys) == 0 {
		return "1.0.0"
	}
	v := svc.deploys[0].Version
	for _, d := range svc.deploys[1:] {
		if !t.Before(d.At) {
			v = d.Version
		}
	}
	return v
}

// Endpoint declares an HTTP route served by the service.
func (svc *Service) Endpoint(method, route string) *Endpoint {
	e := &Endpoint{svc: svc, Method: method, Route: route, policy: DetailPolicy{Recent: 6 * time.Hour, Rate: 0.03, Max: 60}}
	svc.endpoints = append(svc.endpoints, e)
	svc.s.record(method + " " + route)
	return e
}

// Task declares a background task (a CONSUMER span root).
func (svc *Service) Task(name string) *Endpoint {
	e := &Endpoint{svc: svc, Task: true, Route: name, policy: DetailPolicy{Recent: 6 * time.Hour, Rate: 0.05, Max: 40}}
	svc.tasks = append(svc.tasks, e)
	svc.s.record(name)
	return e
}

// Endpoint is an HTTP route (or a task, when Task is set).
type Endpoint struct {
	svc     *Service
	Method  string
	Route   string
	Task    bool
	shapes  []Shape
	window  time.Duration
	handler func(r *Req)
	policy  DetailPolicy
	reqs    []*Request
	gen     bool
	// Status picks the response status; nil means 200.
	status func(r *Req) int
	// SkipAccessLog disables the per-request access log line.
	SkipAccessLog bool
}

// Name returns "METHOD /route" (or the task name).
func (e *Endpoint) Name() string {
	if e.Task {
		return e.Route
	}
	return e.Method + " " + e.Route
}

// Service returns the serving service.
func (e *Endpoint) Service() *Service { return e.svc }

// Traffic sets the arrival process: rate shapes multiplied together over a
// window (Over, default 24h).
func (e *Endpoint) Traffic(shapes ...Shape) *Endpoint {
	for _, sh := range shapes {
		if o, ok := sh.(over); ok {
			e.window = time.Duration(o)
			continue
		}
		e.shapes = append(e.shapes, sh)
	}
	return e
}

// Handler sets the per-request span builder.
func (e *Endpoint) Handler(f func(r *Req)) *Endpoint { e.handler = f; return e }

// Detail tunes which requests are sent with their full span tree. Requests
// that carry logs or exceptions, or are exposed, always are.
func (e *Endpoint) Detail(p DetailPolicy) *Endpoint { e.policy = p; return e }

// DetailPolicy selects requests to send with full span trees: every request
// in the last Recent window with probability Rate, at most Max of them.
type DetailPolicy struct {
	Recent time.Duration
	Rate   float64
	Max    int
}

// Shape is a factor on the arrival rate.
type Shape interface {
	factor(s *Scenario, t time.Time, start, end time.Time) float64
}

type daily float64

func (d daily) factor(*Scenario, time.Time, time.Time, time.Time) float64 { return float64(d) }

// Daily sets the base rate in requests per day.
func Daily(n int) Shape { return daily(n) }

type diurnal struct{}

func (diurnal) factor(_ *Scenario, t, _, _ time.Time) float64 {
	h := float64(t.Hour()) + float64(t.Minute())/60
	// Peak around 14:00 UTC, trough around 02:00; mean ~1.
	return 1 + 0.6*math.Cos((h-14)/24*2*math.Pi)
}

// Diurnal applies a day/night cycle (peak mid-afternoon UTC).
func Diurnal() Shape { return diurnal{} }

type growth struct{ from, to float64 }

func (g growth) factor(_ *Scenario, t, start, end time.Time) float64 {
	return g.from + (g.to-g.from)*progress(t, start, end)
}

// GrowRate multiplies the rate from `from` at the window start to `to` at its end.
func GrowRate(from, to float64) Shape { return growth{from, to} }

type window struct {
	from, to time.Time
	mult     float64
}

func (w window) factor(_ *Scenario, t, _, _ time.Time) float64 {
	if !t.Before(w.from) && t.Before(w.to) {
		return w.mult
	}
	return 1
}

// Burst multiplies the rate inside [from, to).
func Burst(from, to time.Time, mult float64) Shape { return window{from, to, mult} }

type funcShape func(t time.Time) float64

func (f funcShape) factor(_ *Scenario, t, _, _ time.Time) float64 { return f(t) }

// RateFunc is an arbitrary rate factor.
func RateFunc(f func(t time.Time) float64) Shape { return funcShape(f) }

type over time.Duration

func (over) factor(*Scenario, time.Time, time.Time, time.Time) float64 { return 1 }

// Over sets the traffic window ending now.
func Over(d time.Duration) Shape { return over(d) }

func progress(t, start, end time.Time) float64 {
	if !end.After(start) {
		return 1
	}
	p := float64(t.Sub(start)) / float64(end.Sub(start))
	return math.Max(0, math.Min(1, p))
}

// Request is one generated request (or task run).
type Request struct {
	e         *Endpoint
	Index     int
	Start     time.Time
	Host      string
	Version   string
	TraceID   [16]byte
	SpanID    [8]byte
	Root      *Span
	Status    int
	Detail    bool
	forced    bool
	listed    bool // detail only because a listing shows it (listedRequests): no access log line
	Progress  float64
	ParentRef *Span // for downstream SERVER spans: the CLIENT span calling it
}

// Duration is the request's total duration.
func (r *Request) Duration() time.Duration { return r.Root.Dur }

// TraceHex is the 32-hex trace id.
func (r *Request) TraceHex() string { return fmt.Sprintf("%x", r.TraceID) }

// SpanHex is the 16-hex root span id.
func (r *Request) SpanHex() string { return fmt.Sprintf("%x", r.SpanID) }

// Endpoint returns the request's endpoint.
func (r *Request) Endpoint() *Endpoint { return r.e }

// KeepDetail forces the full span tree to be sent.
func (r *Request) KeepDetail() { r.forced = true; r.Detail = true }

// Ref is what seed.json carries for a request.
func (r *Request) Ref() map[string]any {
	return map[string]any{
		"id":         EntityID(r.e.svc.p.ID, r.TraceID, r.SpanID),
		"recordedAt": r.Start.UTC().Format(time.RFC3339Nano),
		"traceId":    r.TraceHex(),
		"spanId":     r.SpanHex(),
		"durationMs": float64(r.Duration()) / float64(time.Millisecond),
		"endpoint":   r.e.Name(),
		"service":    r.e.svc.Name,
		"host":       r.Host,
		"version":    r.Version,
		"project":    r.e.svc.p.Name,
	}
}

// Span is one span in a request tree. Start is the offset from the
// request's start.
type Span struct {
	Name      string
	Kind      int
	Start     time.Duration
	Dur       time.Duration
	Attrs     map[string]any
	Status    int
	StatusMsg string
	Events    []Event
	Children  []*Span
	Scope     string
	SpanID    [8]byte
	// Service is set when the span belongs to another service (a
	// downstream SERVER span and its subtree).
	Service *Service
	Host    string
	logs    []logRecord
}

// Event is a span event.
type Event struct {
	Name  string
	At    time.Duration
	Attrs map[string]any
}

// Requests generates (once) and returns the endpoint's requests in time order.
func (e *Endpoint) Requests() []*Request {
	if e.gen {
		return e.reqs
	}
	e.gen = true
	if e.handler == nil {
		panic(fmt.Errorf("endpoint %s has no handler", e.Name()))
	}
	s := e.svc.s
	win := e.window
	if win == 0 {
		win = 24 * time.Hour
	}
	end := s.Now.Add(-time.Minute)
	start := end.Add(-win)
	base := 0.0
	var mods []Shape
	for _, sh := range e.shapes {
		if d, ok := sh.(daily); ok {
			base += float64(d)
			continue
		}
		mods = append(mods, sh)
	}
	if base == 0 {
		panic(fmt.Errorf("endpoint %s: traffic needs a Daily(n) base rate", e.Name()))
	}
	// Walk the window minute by minute; expected arrivals per minute is the
	// base rate times every shape factor. Fractional arrivals carry over so
	// low rates still produce requests (deterministic dithering).
	carry := 0.0
	idx := 0
	for t := start; t.Before(end); t = t.Add(time.Minute) {
		lambda := base / 1440
		for _, sh := range mods {
			lambda *= sh.factor(s, t, start, end)
		}
		carry += lambda
		n := int(carry)
		carry -= float64(n)
		for k := 0; k < n; k++ {
			at := t.Add(time.Duration(s.RNG.Int64N(int64(time.Minute))))
			r := e.newRequest(idx, at, progress(at, start, end))
			idx++
			e.reqs = append(e.reqs, r)
		}
	}
	sort.SliceStable(e.reqs, func(i, j int) bool { return e.reqs[i].Start.Before(e.reqs[j].Start) })
	e.selectDetail()
	return e.reqs
}

func (e *Endpoint) newRequest(idx int, at time.Time, prog float64) *Request {
	s := e.svc.s
	r := &Request{e: e, Index: idx, Start: at, Progress: prog}
	r.Host = e.svc.pickHost()
	r.Version = e.svc.VersionAt(at)
	fillRandom(s, r.TraceID[:])
	fillRandom(s, r.SpanID[:])
	kind := KindServer
	if e.Task {
		kind = KindConsumer
	}
	root := &Span{Name: e.Name(), Kind: kind, Scope: e.svc.Stack.ServerScope, SpanID: r.SpanID, Attrs: map[string]any{}, Host: r.Host}
	r.Root = root
	b := &Req{s: s, req: r, cur: root}
	if !e.Task {
		root.Attrs["http.request.method"] = e.Method
		root.Attrs["http.route"] = e.Route
		root.Attrs["url.path"] = concretePath(s, e.Route)
		root.Attrs["url.scheme"] = "https"
		root.Attrs["network.protocol.version"] = "1.1"
		root.Attrs["user_agent.original"] = pick(s, userAgents)
		root.Attrs["client.address"] = fmt.Sprintf("10.%d.%d.%d", 1+s.RNG.IntN(30), s.RNG.IntN(255), 1+s.RNG.IntN(250))
	} else {
		root.Attrs["messaging.operation.type"] = "process"
	}
	e.handler(b)
	b.finish()
	r.Status = 200
	if e.status != nil {
		r.Status = e.status(b)
	}
	if b.status != 0 {
		r.Status = b.status
	}
	if !e.Task {
		root.Attrs["http.response.status_code"] = r.Status
		if r.Status >= 500 {
			root.Status = StatusError
		}
	}
	if b.forceDetail {
		r.forced, r.Detail = true, true
	}
	return r
}

// Statuses sets a per-request status picker.
func (e *Endpoint) Statuses(f func(r *Req) int) *Endpoint { e.status = f; return e }

func (e *Endpoint) selectDetail() {
	s := e.svc.s
	cut := s.Now.Add(-e.policy.Recent)
	// Forced detail (logs, errors) never uses up the sample's cap.
	n := 0
	// Prefer the newest requests so the detail sample sits where the agent
	// looks first, but keep it random within the window.
	for i := len(e.reqs) - 1; i >= 0 && n < e.policy.Max; i-- {
		r := e.reqs[i]
		if r.Detail || r.Start.Before(cut) {
			continue
		}
		if s.RNG.Float64() < e.policy.Rate {
			r.Detail = true
			n++
		}
	}
}

func (svc *Service) pickHost() string {
	if len(svc.Hosts) == 1 {
		return svc.Hosts[0]
	}
	total := 0.0
	for _, h := range svc.Hosts {
		total += svc.weight(h)
	}
	x := svc.s.RNG.Float64() * total
	for _, h := range svc.Hosts {
		x -= svc.weight(h)
		if x <= 0 {
			return h
		}
	}
	return svc.Hosts[len(svc.Hosts)-1]
}

func (svc *Service) weight(h string) float64 {
	if w, ok := svc.HostWeight[h]; ok {
		return w
	}
	return 1
}

func fillRandom(s *Scenario, b []byte) {
	for i := 0; i < len(b); i += 8 {
		v := s.RNG.Uint64()
		for j := 0; j < 8 && i+j < len(b); j++ {
			b[i+j] = byte(v >> (8 * j))
		}
	}
	if allZero(b) {
		b[0] = 1
	}
}

func allZero(b []byte) bool {
	for _, x := range b {
		if x != 0 {
			return false
		}
	}
	return true
}

// Slowest returns a lazy reference to the slowest request in the trailing
// window, for Expose.
func (e *Endpoint) Slowest(within time.Duration) func() any {
	return func() any {
		r := e.SlowestRequest(within)
		if r == nil {
			return nil
		}
		return r.Ref()
	}
}

// SlowestRequest returns the slowest request in the trailing window and
// forces it to be sent in detail.
func (e *Endpoint) SlowestRequest(within time.Duration) *Request {
	var best *Request
	cut := e.svc.s.Now.Add(-within)
	for _, r := range e.Requests() {
		if r.Start.Before(cut) {
			continue
		}
		if best == nil || r.Duration() > best.Duration() {
			best = r
		}
	}
	if best != nil {
		best.KeepDetail()
	}
	return best
}

// Percentile returns the p-quantile (0..1) of durations in the trailing window.
func (e *Endpoint) Percentile(p float64, within time.Duration) time.Duration {
	var ds []time.Duration
	cut := e.svc.s.Now.Add(-within)
	for _, r := range e.Requests() {
		if !r.Start.Before(cut) {
			ds = append(ds, r.Duration())
		}
	}
	return quantile(ds, p)
}

// PercentileBetween returns the p-quantile of durations started in [from, to).
func (e *Endpoint) PercentileBetween(p float64, from, to time.Time) time.Duration {
	var ds []time.Duration
	for _, r := range e.Requests() {
		if !r.Start.Before(from) && r.Start.Before(to) {
			ds = append(ds, r.Duration())
		}
	}
	return quantile(ds, p)
}

func quantile(ds []time.Duration, p float64) time.Duration {
	if len(ds) == 0 {
		return 0
	}
	sort.Slice(ds, func(i, j int) bool { return ds[i] < ds[j] })
	pos := p * float64(len(ds)-1)
	lo := int(math.Floor(pos))
	hi := int(math.Ceil(pos))
	frac := pos - float64(lo)
	return time.Duration(float64(ds[lo])*(1-frac) + float64(ds[hi])*frac)
}

// Pick returns a lazy reference to the first request matching f (newest
// first), forcing it to detail.
func (e *Endpoint) Pick(f func(r *Request) bool) func() any {
	return func() any {
		r := e.PickRequest(f)
		if r == nil {
			return nil
		}
		return r.Ref()
	}
}

// PickRequest returns the newest request matching f and forces its detail.
func (e *Endpoint) PickRequest(f func(r *Request) bool) *Request {
	reqs := e.Requests()
	for i := len(reqs) - 1; i >= 0; i-- {
		if f(reqs[i]) {
			reqs[i].KeepDetail()
			return reqs[i]
		}
	}
	return nil
}

// Count returns how many requests started in the trailing window.
func (e *Endpoint) Count(within time.Duration) int {
	cut := e.svc.s.Now.Add(-within)
	n := 0
	for _, r := range e.Requests() {
		if !r.Start.Before(cut) {
			n++
		}
	}
	return n
}

var userAgents = []string{
	"Mozilla/5.0 (Macintosh; Intel Mac OS X 14_6) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/18.1 Safari/605.1.15",
	"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/141.0.0.0 Safari/537.36",
	"Mozilla/5.0 (iPhone; CPU iPhone OS 18_1 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Mobile/15E148",
	"okhttp/4.12.0",
}

func pick[T any](s *Scenario, xs []T) T { return xs[s.RNG.IntN(len(xs))] }

// concretePath turns a route template into a plausible concrete path.
func concretePath(s *Scenario, route string) string {
	out := []byte{}
	for i := 0; i < len(route); i++ {
		if route[i] == ':' || route[i] == '{' {
			j := i + 1
			for j < len(route) && route[j] != '/' && route[j] != '}' {
				j++
			}
			if j < len(route) && route[j] == '}' {
				j++
			}
			out = append(out, []byte(fmt.Sprint(1000+s.RNG.IntN(90000)))...)
			i = j - 1
			continue
		}
		out = append(out, route[i])
	}
	return string(out)
}
