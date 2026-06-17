package otelprocessor

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"go.opentelemetry.io/collector/pdata/pcommon"
)

const iosFixtureDir = "../ios/fixtures/sample"

func TestSymbolicateIOSTrace(t *testing.T) {
	dsym, err := os.ReadFile(filepath.Join(iosFixtureDir, "sample.dsym"))
	if err != nil {
		t.Skipf("ios fixture not available (%v)", err)
	}
	raw, err := os.ReadFile(filepath.Join(iosFixtureDir, "trace.txt"))
	if err != nil {
		t.Skipf("ios fixture trace not available (%v)", err)
	}

	store := t.TempDir()
	if err := os.WriteFile(filepath.Join(store, "2dd71042118432be8f92dd4e3d3fe24a.dsym"), dsym, 0o644); err != nil {
		t.Fatal(err)
	}

	p := newTestProcessor(t, func(c *Config) {
		c.SourceMapStoreKey = fileStoreKey
		c.LocalSourceMaps = LocalSourceMapsConfig{Path: store}
	})

	attrs := pcommon.NewMap()
	attrs.PutStr(p.cfg.ExceptionTypeAttributeKey, "SampleError")
	attrs.PutStr(p.cfg.ExceptionMessageAttributeKey, "boom")
	attrs.PutStr(p.cfg.StackTraceAttributeKey, string(raw))

	p.processRecord(context.Background(), attrs, pcommon.NewMap())

	got := strAttr(attrs, p.cfg.StackTraceAttributeKey)
	for _, want := range []string{"leaf (sample.c:7:18)", "compute (sample.c:14)", "main (sample.c:18)"} {
		if !strings.Contains(got, want) {
			t.Errorf("expected resolved frame %q in:\n%s", want, got)
		}
	}
	if !strings.Contains(got, "UnknownLib+0x1234") {
		t.Errorf("expected unresolved system frame as image+offset in:\n%s", got)
	}
	if v, ok := attrs.Get(p.cfg.SymbolicatorFailureAttributeKey); !ok || v.Bool() {
		t.Errorf("expected symbolicator.failed=false, got %v", v)
	}
}

func TestSymbolicateHoneycombIOSTrace(t *testing.T) {
	dsym, err := os.ReadFile(filepath.Join(iosFixtureDir, "sample.dsym"))
	if err != nil {
		t.Skipf("ios fixture not available (%v)", err)
	}
	raw, err := os.ReadFile(filepath.Join(iosFixtureDir, "trace_honeycomb.txt"))
	if err != nil {
		t.Skipf("honeycomb ios fixture not available (%v)", err)
	}
	store := t.TempDir()
	if err := os.WriteFile(filepath.Join(store, "2dd71042118432be8f92dd4e3d3fe24a.dsym"), dsym, 0o644); err != nil {
		t.Fatal(err)
	}
	p := newTestProcessor(t, func(c *Config) {
		c.SourceMapStoreKey = fileStoreKey
		c.LocalSourceMaps = LocalSourceMapsConfig{Path: store}
	})

	attrs := pcommon.NewMap()
	attrs.PutStr(p.cfg.StackTraceAttributeKey, string(raw))
	resource := pcommon.NewMap()
	resource.PutStr(p.cfg.LanguageAttributeKey, "swift")

	p.processRecord(context.Background(), attrs, resource)

	got := strAttr(attrs, p.cfg.StackTraceAttributeKey)
	for _, want := range []string{"leaf (sample.c:7:18)", "compute (sample.c:14)", "main (sample.c:18)"} {
		if !strings.Contains(got, want) {
			t.Errorf("expected resolved frame %q in:\n%s", want, got)
		}
	}
	if v, ok := attrs.Get(p.cfg.SymbolicatorFailureAttributeKey); !ok || v.Bool() {
		t.Errorf("expected symbolicator.failed=false, got %v", v)
	}
}

func TestSymbolicateHoneycombIOSBinaryName(t *testing.T) {
	dsym, err := os.ReadFile(filepath.Join(iosFixtureDir, "sample.dsym"))
	if err != nil {
		t.Skipf("ios fixture not available (%v)", err)
	}
	store := t.TempDir()
	if err := os.WriteFile(filepath.Join(store, "2dd71042118432be8f92dd4e3d3fe24a.dsym"), dsym, 0o644); err != nil {
		t.Fatal(err)
	}
	p := newTestProcessor(t, func(c *Config) {
		c.SourceMapStoreKey = fileStoreKey
		c.LocalSourceMaps = LocalSourceMapsConfig{Path: store}
	})

	attrs := pcommon.NewMap()
	attrs.PutStr(p.cfg.StackTraceAttributeKey, "os: ios arch: arm64\n0   sample   0x100000460   sample + 1120\n2   sample   0x100000494   sample + 1172")
	resource := pcommon.NewMap()
	resource.PutStr(p.cfg.LanguageAttributeKey, "swift")
	resource.PutStr(p.cfg.IOSBuildUUIDAttributeKey, "2DD71042-1184-32BE-8F92-DD4E3D3FE24A")
	resource.PutStr(p.cfg.AppExecutableAttributeKey, "sample")

	p.processRecord(context.Background(), attrs, resource)

	got := strAttr(attrs, p.cfg.StackTraceAttributeKey)
	if !strings.Contains(got, "leaf (sample.c:7:18)") || !strings.Contains(got, "main (sample.c:18)") {
		t.Errorf("binary-name honeycomb trace not resolved:\n%s", got)
	}
}

func TestSymbolicateIOSByLanguageHeader(t *testing.T) {
	dsym, err := os.ReadFile(filepath.Join(iosFixtureDir, "sample.dsym"))
	if err != nil {
		t.Skipf("ios fixture not available (%v)", err)
	}
	raw, err := os.ReadFile(filepath.Join(iosFixtureDir, "trace.txt"))
	if err != nil {
		t.Skipf("ios fixture trace not available (%v)", err)
	}
	store := t.TempDir()
	if err := os.WriteFile(filepath.Join(store, "2dd71042118432be8f92dd4e3d3fe24a.dsym"), dsym, 0o644); err != nil {
		t.Fatal(err)
	}
	p := newTestProcessor(t, func(c *Config) {
		c.SourceMapStoreKey = fileStoreKey
		c.LocalSourceMaps = LocalSourceMapsConfig{Path: store}
	})

	attrs := pcommon.NewMap()
	attrs.PutStr(p.cfg.StackTraceAttributeKey, string(raw))
	resource := pcommon.NewMap()
	resource.PutStr(p.cfg.LanguageAttributeKey, "swift")

	p.processRecord(context.Background(), attrs, resource)

	got := strAttr(attrs, p.cfg.StackTraceAttributeKey)
	if !strings.Contains(got, "leaf (sample.c:7:18)") || !strings.Contains(got, "main (sample.c:18)") {
		t.Errorf("language-routed iOS trace not resolved:\n%s", got)
	}
}

func TestSymbolicateIOSByLanguageHeaderIOSValue(t *testing.T) {
	dsym, err := os.ReadFile(filepath.Join(iosFixtureDir, "sample.dsym"))
	if err != nil {
		t.Skipf("ios fixture not available (%v)", err)
	}
	raw, err := os.ReadFile(filepath.Join(iosFixtureDir, "trace.txt"))
	if err != nil {
		t.Skipf("ios fixture trace not available (%v)", err)
	}
	store := t.TempDir()
	if err := os.WriteFile(filepath.Join(store, "2dd71042118432be8f92dd4e3d3fe24a.dsym"), dsym, 0o644); err != nil {
		t.Fatal(err)
	}
	p := newTestProcessor(t, func(c *Config) {
		c.SourceMapStoreKey = fileStoreKey
		c.LocalSourceMaps = LocalSourceMapsConfig{Path: store}
	})

	attrs := pcommon.NewMap()
	attrs.PutStr(p.cfg.StackTraceAttributeKey, string(raw))
	resource := pcommon.NewMap()
	resource.PutStr(p.cfg.LanguageAttributeKey, "ios")

	p.processRecord(context.Background(), attrs, resource)

	if got := strAttr(attrs, p.cfg.StackTraceAttributeKey); !strings.Contains(got, "leaf (sample.c:7:18)") {
		t.Errorf("language=ios did not route to the iOS symbolicator:\n%s", got)
	}
}

func TestIOSLanguageLeavesSymbolicStackUntouched(t *testing.T) {
	p := newTestProcessor(t, func(c *Config) {
		c.SourceMapStoreKey = fileStoreKey
		c.LocalSourceMaps = LocalSourceMapsConfig{Path: t.TempDir()}
	})

	symbolic := "BoomError: kaboom\n  at App.run() (App.swift:12:5)"
	attrs := pcommon.NewMap()
	attrs.PutStr(p.cfg.StackTraceAttributeKey, symbolic)
	resource := pcommon.NewMap()
	resource.PutStr(p.cfg.LanguageAttributeKey, "ios")

	p.processRecord(context.Background(), attrs, resource)

	if got := strAttr(attrs, p.cfg.StackTraceAttributeKey); got != symbolic {
		t.Errorf("symbolic stack was modified:\n got: %q\nwant: %q", got, symbolic)
	}
	if _, ok := attrs.Get(p.cfg.SymbolicatorFailureAttributeKey); ok {
		t.Error("no-op iOS path should not set the failure flag")
	}
}

func TestSymbolicateIOSBenchmarkCorpus(t *testing.T) {
	dsym, err := os.ReadFile(filepath.Join(iosFixtureDir, "sample.dsym"))
	if err != nil {
		t.Skipf("ios fixture not available (%v)", err)
	}
	store := t.TempDir()
	synthetic := "00000000000000000000000000000001"
	if err := os.WriteFile(filepath.Join(store, synthetic+".dsym"), dsym, 0o644); err != nil {
		t.Fatal(err)
	}
	p := newTestProcessor(t, func(c *Config) {
		c.SourceMapStoreKey = fileStoreKey
		c.LocalSourceMaps = LocalSourceMapsConfig{Path: store}
	})

	attrs := pcommon.NewMap()
	attrs.PutStr(p.cfg.ExceptionTypeAttributeKey, "IOSError")
	attrs.PutStr(p.cfg.ExceptionMessageAttributeKey, "bench")
	attrs.PutStr(p.cfg.StackTraceAttributeKey,
		"IOSError: bench\nos: ios arch: arm64\n#00 "+synthetic+" 0x460 sample\n#01 "+synthetic+" 0x494 sample")
	resource := pcommon.NewMap()
	resource.PutStr(p.cfg.LanguageAttributeKey, "swift")

	p.processRecord(context.Background(), attrs, resource)

	got := strAttr(attrs, p.cfg.StackTraceAttributeKey)
	if !strings.Contains(got, "sample.c") {
		t.Errorf("expected resolved frames (drain OK marker sample.c):\n%s", got)
	}
	if strings.Contains(got, "sample+0x") {
		t.Errorf("unexpected unresolved frames (drain FAIL marker sample+0x):\n%s", got)
	}
}

func TestSymbolicateIOSTraceMissingSymbols(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join(iosFixtureDir, "trace.txt"))
	if err != nil {
		t.Skipf("ios fixture trace not available (%v)", err)
	}

	p := newTestProcessor(t, func(c *Config) {
		c.SourceMapStoreKey = fileStoreKey
		c.LocalSourceMaps = LocalSourceMapsConfig{Path: t.TempDir()}
	})

	attrs := pcommon.NewMap()
	attrs.PutStr(p.cfg.StackTraceAttributeKey, string(raw))
	p.processRecord(context.Background(), attrs, pcommon.NewMap())

	got := strAttr(attrs, p.cfg.StackTraceAttributeKey)
	if strings.Contains(got, "leaf (") {
		t.Errorf("did not expect symbolication without artifacts:\n%s", got)
	}
	if !strings.Contains(got, "sample+0x460") {
		t.Errorf("expected stable image+offset frames, got:\n%s", got)
	}
	if v, ok := attrs.Get(p.cfg.SymbolicatorFailureAttributeKey); !ok || !v.Bool() {
		t.Errorf("expected symbolicator.failed=true when no dSYM is available, got %v (present=%v)", v, ok)
	}
	if errMsg := strAttr(attrs, p.cfg.SymbolicatorErrorAttributeKey); errMsg == "" {
		t.Error("expected symbolicator error attribute to be set when symbols are missing")
	}
}
