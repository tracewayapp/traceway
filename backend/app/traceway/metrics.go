package traceway

import (
	"context"

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
