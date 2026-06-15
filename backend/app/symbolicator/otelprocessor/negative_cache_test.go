package otelprocessor

import (
	"context"
	"testing"

	"go.opentelemetry.io/collector/pdata/pcommon"
)

func TestJSNegativeCacheShortCircuit(t *testing.T) {
	p := newTestProcessor(t, func(cfg *Config) {
		cfg.LocalSourceMaps.Path = t.TempDir()
	})

	resolveOnce := func() {
		attrs := pcommon.NewMap()
		putStructuredFrame(attrs)
		p.processRecord(context.Background(), attrs, pcommon.NewMap())
	}

	resolveOnce()
	first := p.cache.Stats()
	if first.Misses == 0 {
		t.Fatalf("expected the first resolve to miss the cache and fetch the bundle: %+v", first)
	}

	resolveOnce()
	second := p.cache.Stats()
	if second.Misses != first.Misses {
		t.Errorf("missing bundle re-fetched on the second resolve (misses %d -> %d): negative-cache short-circuit not applied", first.Misses, second.Misses)
	}
	if second.NegativeHits == 0 {
		t.Errorf("expected a negative-cache hit on the second resolve: %+v", second)
	}
}
