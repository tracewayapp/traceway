package traceway

import (
	"context"
	"sync"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

func CaptureMetric(name string, value float64) {
	CaptureMetricWithTags(name, value, nil)
}

// Only the last value recorded per tag set within an export interval is sent.
func CaptureMetricWithTags(name string, value float64, tags map[string]string) {
	s := current.Load()
	if !s.enabled {
		return
	}
	if g := s.gauge(name); g != nil {
		g.Record(context.Background(), value, metric.WithAttributes(tagAttributes(tags)...))
	}
}

// CaptureMetricMax reports the largest value recorded per tag set within an
// export interval, so a spike between two exports is never lost to a later,
// smaller value.
func CaptureMetricMax(name string, value float64, tags map[string]string) {
	s := current.Load()
	if !s.enabled {
		return
	}
	if g := s.maxGauge(name); g != nil {
		g.record(value, attribute.NewSet(tagAttributes(tags)...))
	}
}

func CaptureCount(name string, n int64, tags map[string]string) {
	s := current.Load()
	if !s.enabled {
		return
	}
	if c := s.counter(name); c != nil {
		c.Add(context.Background(), n, metric.WithAttributes(tagAttributes(tags)...))
	}
}

func (s *state) gauge(name string) metric.Float64Gauge {
	if g, ok := s.gauges.Load(name); ok {
		return g.(metric.Float64Gauge)
	}
	g, err := s.meter.Float64Gauge(name)
	if err != nil {
		return nil
	}
	actual, _ := s.gauges.LoadOrStore(name, g)
	return actual.(metric.Float64Gauge)
}

type maxGauge struct {
	mu     sync.Mutex
	values map[attribute.Distinct]maxValue
}

type maxValue struct {
	set   attribute.Set
	value float64
}

func (s *state) maxGauge(name string) *maxGauge {
	if g, ok := s.maxGauges.Load(name); ok {
		return g.(*maxGauge)
	}
	g := &maxGauge{values: map[attribute.Distinct]maxValue{}}
	if actual, loaded := s.maxGauges.LoadOrStore(name, g); loaded {
		return actual.(*maxGauge)
	}
	if _, err := s.meter.Float64ObservableGauge(name, metric.WithFloat64Callback(g.observe)); err != nil {
		return nil
	}
	return g
}

func (g *maxGauge) record(value float64, set attribute.Set) {
	key := set.Equivalent()
	g.mu.Lock()
	defer g.mu.Unlock()
	if existing, ok := g.values[key]; !ok || value > existing.value {
		g.values[key] = maxValue{set: set, value: value}
	}
}

// Each collection starts a new window: a tag set with no value since the
// last one is not reported.
func (g *maxGauge) observe(_ context.Context, o metric.Float64Observer) error {
	g.mu.Lock()
	values := g.values
	g.values = map[attribute.Distinct]maxValue{}
	g.mu.Unlock()
	for _, v := range values {
		o.Observe(v.value, metric.WithAttributeSet(v.set))
	}
	return nil
}

func (s *state) counter(name string) metric.Int64Counter {
	if c, ok := s.counters.Load(name); ok {
		return c.(metric.Int64Counter)
	}
	c, err := s.meter.Int64Counter(name)
	if err != nil {
		return nil
	}
	actual, _ := s.counters.LoadOrStore(name, c)
	return actual.(metric.Int64Counter)
}

func tagAttributes(tags map[string]string) []attribute.KeyValue {
	if len(tags) == 0 {
		return nil
	}
	attrs := make([]attribute.KeyValue, 0, len(tags))
	for k, v := range tags {
		attrs = append(attrs, attribute.String(k, v))
	}
	return attrs
}
