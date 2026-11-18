package seed

import (
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"math"
	"sort"
	"time"

	colmetricspb "go.opentelemetry.io/proto/otlp/collector/metrics/v1"
	commonpb "go.opentelemetry.io/proto/otlp/common/v1"
	metricspb "go.opentelemetry.io/proto/otlp/metrics/v1"
)

// Series is a value over time. Noise inside a Series is a deterministic
// function of (series, t), so the same scenario always draws the same curve.
type Series func(t time.Time) float64

// noise returns a deterministic [0,1) value for a label and a time.
func noise(label string, t time.Time) float64 {
	var b [8]byte
	binary.BigEndian.PutUint64(b[:], uint64(t.Unix()))
	sum := sha256.Sum256(append([]byte(label), b[:]...))
	return float64(binary.BigEndian.Uint64(sum[:8])>>11) / float64(1<<53)
}

// Flat is a value wandering between lo and hi.
func Flat(lo, hi float64) Series {
	return func(t time.Time) float64 {
		// Smooth-ish: blend the current and previous 5-minute noise.
		a := noise("flat", t.Truncate(5*time.Minute))
		b := noise("flat", t.Truncate(5*time.Minute).Add(5*time.Minute))
		f := float64(t.Sub(t.Truncate(5*time.Minute))) / float64(5*time.Minute)
		return lo + (hi-lo)*(a*(1-f)+b*f)
	}
}

// Const is a constant value.
func Const(v float64) Series { return func(time.Time) float64 { return v } }

// Step switches from before to after at at.
func Step(at time.Time, before, after Series) Series {
	return func(t time.Time) float64 {
		if t.Before(at) {
			return before(t)
		}
		return after(t)
	}
}

// Ramp interpolates linearly from `from` at start to `to` at end, plus a
// little noise (±3% of the span).
func Ramp(from, to float64, start, end time.Time) Series {
	return func(t time.Time) float64 {
		p := progress(t, start, end)
		return from + (to-from)*p + (to-from)*0.03*(noise("ramp", t.Truncate(time.Minute))-0.5)
	}
}

// Scale multiplies a series by f(t).
func Scale(s Series, f func(t time.Time) float64) Series {
	return func(t time.Time) float64 { return s(t) * f(t) }
}

// Plus adds series.
func Plus(ss ...Series) Series {
	return func(t time.Time) float64 {
		v := 0.0
		for _, s := range ss {
			v += s(t)
		}
		return v
	}
}

// Clamp bounds a series to [lo, hi].
func Clamp(s Series, lo, hi float64) Series {
	return func(t time.Time) float64 { return math.Max(lo, math.Min(hi, s(t))) }
}

// Periodic is base, raised to peak for width at every multiple of period.
func Periodic(period, width time.Duration, base, peak Series) Series {
	return func(t time.Time) float64 {
		off := t.Sub(t.Truncate(period))
		if off < width {
			return peak(t)
		}
		return base(t)
	}
}

// Sawtooth climbs from lo to hi over period, then drops back (a leak that
// is cleared by a restart).
func Sawtooth(period time.Duration, lo, hi float64) Series {
	return func(t time.Time) float64 {
		f := float64(t.Sub(t.Truncate(period))) / float64(period)
		return lo + (hi-lo)*f + (hi-lo)*0.02*(noise("saw", t.Truncate(time.Minute))-0.5)
	}
}

// DiurnalSeries modulates a series with the same day/night cycle as Diurnal.
func DiurnalSeries(s Series, depth float64) Series {
	return func(t time.Time) float64 {
		h := float64(t.Hour()) + float64(t.Minute())/60
		return s(t) * (1 + depth*math.Cos((h-14)/24*2*math.Pi))
	}
}

type metricKind int

const (
	gaugeKind metricKind = iota
	counterKind
	histogramKind
)

type metricSeries struct {
	name, unit, desc string
	kind             metricKind
	attrs            map[string]any
	host             string
	value            Series // gauge value; counter rate per second; histogram mean
	count            Series // histogram observations per minute
	window           time.Duration
	scope            string
	// cumTotal and cumPrev carry a counter's running total across the
	// chunks metricRequests renders it in; histPrev carries the end of a
	// histogram's last delta window the same way, and histCarry the part of
	// an observation its rounded counts have not reported yet.
	cumTotal  float64
	cumPrev   time.Time
	histPrev  time.Time
	histCarry float64
}

// MetricStep controls sampling: points every Recent inside the last day,
// every Old before that.
var MetricStep = struct{ Recent, Old time.Duration }{time.Minute, 10 * time.Minute}

// Gauge adds a gauge series on the service's first host.
func (svc *Service) Gauge(name, unit string, attrs map[string]any, v Series) {
	svc.GaugeOn(svc.Hosts[0], name, unit, attrs, v)
}

// GaugeOn adds a gauge series for one instance. A ratio (unit "1") stays
// within [0, 1] after the jitter.
func (svc *Service) GaugeOn(host, name, unit string, attrs map[string]any, v Series) {
	label := fmt.Sprint(host, name, attrs)
	jittered := func(t time.Time) float64 { return v(t) * (1 + 0.03*(noise(label, t)-0.5)) }
	if unit == "1" {
		jittered = Clamp(jittered, 0, 1)
	}
	svc.metrics = append(svc.metrics, &metricSeries{name: name, unit: unit, kind: gaugeKind, attrs: attrs, host: host, value: jittered, scope: metricScope(name)})
	svc.s.record(name)
}

// Counter adds a monotonic cumulative sum whose rate per second is v.
func (svc *Service) Counter(name, unit string, attrs map[string]any, ratePerSec Series) {
	svc.metrics = append(svc.metrics, &metricSeries{name: name, unit: unit, kind: counterKind, attrs: attrs, host: svc.Hosts[0], value: ratePerSec, scope: metricScope(name)})
	svc.s.record(name)
}

// Histogram adds a delta histogram: mean value and observations per minute.
// The backend stores it as <name>.avg and <name>.count.
func (svc *Service) Histogram(name, unit string, attrs map[string]any, mean, perMinute Series) {
	svc.HistogramOn(svc.Hosts[0], name, unit, attrs, mean, perMinute)
}

// HistogramOn is Histogram for one instance.
func (svc *Service) HistogramOn(host, name, unit string, attrs map[string]any, mean, perMinute Series) {
	svc.metrics = append(svc.metrics, &metricSeries{name: name, unit: unit, kind: histogramKind, attrs: attrs, host: host, value: mean, count: perMinute, scope: metricScope(name)})
	svc.s.record(name)
}

// MetricsWindow sets how far back the service's metric series go (default:
// the scenario's longest traffic window, at least 24h).
func (svc *Service) metricsWindow() time.Duration {
	w := 24 * time.Hour
	for _, e := range append(append([]*Endpoint{}, svc.endpoints...), svc.tasks...) {
		if e.window > w {
			w = e.window
		}
	}
	return w
}

func metricScope(name string) string {
	switch {
	case len(name) >= 7 && name[:7] == "system.":
		return "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver"
	case len(name) > 4 && name[:4] == "jvm.":
		return "io.opentelemetry.runtime-telemetry-java17"
	case len(name) > 3 && name[:3] == "go.":
		return "go.opentelemetry.io/contrib/instrumentation/runtime"
	case len(name) > 10 && name[:10] == "db.client.":
		return "io.opentelemetry.hikaricp-3.0"
	case len(name) > 12 && name[:12] == "http.server.":
		return "go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	default:
		return "app"
	}
}

// Host is a convenience over one instance's host metrics.
type Host struct {
	svc  *Service
	Name string
}

// Host returns the first instance's host metrics.
func (svc *Service) Host() *Host { return &Host{svc: svc, Name: svc.Hosts[0]} }

// HostNamed returns one instance's host metrics.
func (svc *Service) HostNamed(name string) *Host { return &Host{svc: svc, Name: name} }

// CPU emits system.cpu.utilization (OTel hostmetrics) split into user,
// system, wait, steal and idle, with busy = user+system. Optional wait and
// steal series default to small flat values.
func (h *Host) CPU(busy Series, extra ...CPUExtra) {
	wait, steal := Flat(0.002, 0.01), Flat(0, 0.004)
	for _, e := range extra {
		if e.Wait != nil {
			wait = e.Wait
		}
		if e.Steal != nil {
			steal = e.Steal
		}
	}
	clampFrac := func(s Series) Series { return Clamp(s, 0, 1) }
	busy, wait, steal = clampFrac(busy), clampFrac(wait), clampFrac(steal)
	user := Scale(busy, func(time.Time) float64 { return 0.78 })
	sys := Scale(busy, func(time.Time) float64 { return 0.22 })
	idle := func(t time.Time) float64 { return math.Max(0, 1-busy(t)-wait(t)-steal(t)) }
	for state, s := range map[string]Series{"user": user, "system": sys, "wait": wait, "steal": steal, "idle": idle} {
		h.svc.GaugeOn(h.Name, "system.cpu.utilization", "1", map[string]any{"state": state, "cpu": "cpu0"}, s)
	}
}

// CPUExtra adds iowait or steal to CPU.
type CPUExtra struct{ Wait, Steal Series }

// Memory emits system.memory.utilization{state=used|free}.
func (h *Host) Memory(used Series) {
	used = Clamp(used, 0, 1)
	h.svc.GaugeOn(h.Name, "system.memory.utilization", "1", map[string]any{"state": "used"}, used)
	h.svc.GaugeOn(h.Name, "system.memory.utilization", "1", map[string]any{"state": "free"}, func(t time.Time) float64 { return 1 - used(t) })
}

// Disk emits the hostmetrics disk series for one device: system.disk.io
// (cumulative bytes, by direction) and system.disk.pending_operations (the
// queue depth gauge).
func (h *Host) Disk(device string, readBps, writeBps, pending Series) {
	h.svc.metrics = append(h.svc.metrics,
		&metricSeries{name: "system.disk.io", unit: "By", kind: counterKind, attrs: map[string]any{"device": device, "direction": "read"}, host: h.Name, value: readBps, scope: metricScope("system.")},
		&metricSeries{name: "system.disk.io", unit: "By", kind: counterKind, attrs: map[string]any{"device": device, "direction": "write"}, host: h.Name, value: writeBps, scope: metricScope("system.")},
	)
	h.svc.GaugeOn(h.Name, "system.disk.pending_operations", "{operations}", map[string]any{"device": device}, pending)
}

// Filesystem emits system.filesystem.utilization for a mountpoint.
func (h *Host) Filesystem(mount string, used Series) {
	h.svc.GaugeOn(h.Name, "system.filesystem.utilization", "1", map[string]any{"mountpoint": mount, "state": "used"}, Clamp(used, 0, 1))
}

func (s *Scenario) metricRequests(svc *Service) []*colmetricspb.ExportMetricsServiceRequest {
	if len(svc.metrics) == 0 {
		return nil
	}
	end := s.Now.Add(-time.Minute)
	win := svc.metricsWindow()
	start := end.Add(-win)
	recentCut := end.Add(-24 * time.Hour)
	var times []time.Time
	for t := start.Truncate(MetricStep.Old); t.Before(recentCut); t = t.Add(MetricStep.Old) {
		if !t.Before(start) {
			times = append(times, t)
		}
	}
	for t := recentCut.Truncate(MetricStep.Recent); !t.After(end); t = t.Add(MetricStep.Recent) {
		times = append(times, t)
	}
	byHost := map[string][]*metricSeries{}
	var hosts []string
	for _, m := range svc.metrics {
		if _, ok := byHost[m.host]; !ok {
			hosts = append(hosts, m.host)
		}
		byHost[m.host] = append(byHost[m.host], m)
	}
	sort.Strings(hosts)
	var out []*colmetricspb.ExportMetricsServiceRequest
	const chunk = 400 // timestamps per request keeps each body small
	for i := 0; i < len(times); i += chunk {
		j := min(i+chunk, len(times))
		req := &colmetricspb.ExportMetricsServiceRequest{}
		for _, host := range hosts {
			rm := &metricspb.ResourceMetrics{Resource: svc.resource(svc.VersionAt(times[j-1]), host)}
			scopes := map[string]*metricspb.ScopeMetrics{}
			var scopeNames []string
			for _, m := range byHost[host] {
				sm := scopes[m.scope]
				if sm == nil {
					sm = &metricspb.ScopeMetrics{Scope: &commonpb.InstrumentationScope{Name: m.scope}}
					scopes[m.scope] = sm
					scopeNames = append(scopeNames, m.scope)
				}
				sm.Metrics = append(sm.Metrics, m.render(times[i:j], start))
				s.stats.MetricPoints += j - i
			}
			sort.Strings(scopeNames)
			for _, n := range scopeNames {
				rm.ScopeMetrics = append(rm.ScopeMetrics, scopes[n])
			}
			req.ResourceMetrics = append(req.ResourceMetrics, rm)
		}
		out = append(out, req)
	}
	return out
}

func (m *metricSeries) render(times []time.Time, origin time.Time) *metricspb.Metric {
	attrs := attrList(m.attrs)
	out := &metricspb.Metric{Name: m.name, Unit: m.unit}
	switch m.kind {
	case gaugeKind:
		g := &metricspb.Gauge{}
		for _, t := range times {
			g.DataPoints = append(g.DataPoints, &metricspb.NumberDataPoint{
				Attributes: attrs, TimeUnixNano: nanos(t),
				Value: &metricspb.NumberDataPoint_AsDouble{AsDouble: m.value(t)},
			})
		}
		out.Data = &metricspb.Metric_Gauge{Gauge: g}
	case counterKind:
		sum := &metricspb.Sum{IsMonotonic: true, AggregationTemporality: metricspb.AggregationTemporality_AGGREGATION_TEMPORALITY_CUMULATIVE}
		if m.cumPrev.IsZero() {
			m.cumPrev = origin
		}
		total, prev := m.cumTotal, m.cumPrev
		for _, t := range times {
			total += m.value(t) * t.Sub(prev).Seconds()
			prev = t
			sum.DataPoints = append(sum.DataPoints, &metricspb.NumberDataPoint{
				Attributes: attrs, StartTimeUnixNano: nanos(origin), TimeUnixNano: nanos(t),
				Value: &metricspb.NumberDataPoint_AsDouble{AsDouble: total},
			})
		}
		m.cumTotal, m.cumPrev = total, prev
		out.Data = &metricspb.Metric_Sum{Sum: sum}
	case histogramKind:
		// Each delta point covers the time since the previous one, which is
		// longer where points are sparser (MetricStep.Old), and its count
		// covers the same window.
		h := &metricspb.Histogram{AggregationTemporality: metricspb.AggregationTemporality_AGGREGATION_TEMPORALITY_DELTA}
		prev := m.histPrev
		if prev.IsZero() {
			step := time.Minute
			if len(times) > 1 {
				step = times[1].Sub(times[0])
			}
			prev = times[0].Add(-step)
		}
		carry := m.histCarry
		for _, t := range times {
			per := math.Max(0, m.count(t)*t.Sub(prev).Minutes()) + carry
			cnt := uint64(math.Max(0, math.Round(per)))
			carry = per - float64(cnt)
			mean := m.value(t)
			s := mean * float64(cnt)
			h.DataPoints = append(h.DataPoints, &metricspb.HistogramDataPoint{
				Attributes: attrs, StartTimeUnixNano: nanos(prev), TimeUnixNano: nanos(t),
				Count: cnt, Sum: &s,
			})
			prev = t
		}
		m.histPrev, m.histCarry = prev, carry
		out.Data = &metricspb.Metric_Histogram{Histogram: h}
	}
	return out
}
