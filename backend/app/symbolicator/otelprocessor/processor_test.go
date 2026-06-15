package otelprocessor

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.uber.org/zap"
)

func stageStore(t *testing.T, subdir string) string {
	t.Helper()
	dir := t.TempDir()
	target := dir
	if subdir != "" {
		target = filepath.Join(dir, subdir)
		if err := os.MkdirAll(target, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	source, err := os.ReadFile("testdata/minified.js")
	if err != nil {
		t.Fatal(err)
	}
	source = append(source, []byte("\n//# sourceMappingURL=minified.js.map\n")...)
	if err := os.WriteFile(filepath.Join(target, "minified.js"), source, 0o644); err != nil {
		t.Fatal(err)
	}
	sourceMap, err := os.ReadFile("testdata/minified.js.map")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(target, "minified.js.map"), sourceMap, 0o644); err != nil {
		t.Fatal(err)
	}
	return dir
}

func newTestProcessor(t *testing.T, mutate func(*Config)) *symbolicatorProcessor {
	t.Helper()
	cfg := createDefaultConfig().(*Config)
	if mutate != nil {
		mutate(cfg)
	}
	if err := cfg.Validate(); err != nil {
		t.Fatal(err)
	}
	store, err := newStore(cfg)
	if err != nil {
		t.Fatal(err)
	}
	cache, err := newCache(cfg)
	if err != nil {
		t.Fatal(err)
	}
	return &symbolicatorProcessor{cfg: cfg, store: store, cache: cache, logger: zap.NewNop()}
}

func putStructuredFrame(attrs pcommon.Map) {
	attrs.PutStr("exception.type", "Error")
	attrs.PutStr("exception.message", "boom")
	attrs.PutStr("exception.stacktrace", "Error: boom\n    at t (https://cdn.example.com/assets/minified.js:1:11)")
	urls := attrs.PutEmptySlice("exception.structured_stacktrace.urls")
	urls.AppendEmpty().SetStr("https://cdn.example.com/assets/minified.js")
	functions := attrs.PutEmptySlice("exception.structured_stacktrace.functions")
	functions.AppendEmpty().SetStr("t")
	lines := attrs.PutEmptySlice("exception.structured_stacktrace.lines")
	lines.AppendEmpty().SetInt(1)
	columns := attrs.PutEmptySlice("exception.structured_stacktrace.columns")
	columns.AppendEmpty().SetInt(11)
}

func TestProcessTracesStructuredRoute(t *testing.T) {
	storeDir := stageStore(t, "")
	p := newTestProcessor(t, func(cfg *Config) {
		cfg.LocalSourceMaps.Path = storeDir
	})

	td := ptrace.NewTraces()
	rs := td.ResourceSpans().AppendEmpty()
	rs.Resource().Attributes().PutStr("telemetry.sdk.language", "webjs")
	span := rs.ScopeSpans().AppendEmpty().Spans().AppendEmpty()
	span.SetName("exception")
	putStructuredFrame(span.Attributes())

	if _, err := p.processTraces(context.Background(), td); err != nil {
		t.Fatal(err)
	}

	attrs := span.Attributes()
	want := "Error: boom\n    at abcd(tests/fixtures/simple/original.js:2:10)"
	if got := strAttr(attrs, "exception.stacktrace"); got != want {
		t.Errorf("stacktrace:\n got %q\nwant %q", got, want)
	}
	if got := strAttr(attrs, "exception.stacktrace.original"); got == "" {
		t.Error("expected original stacktrace preserved")
	}
	if v, ok := attrs.Get("exception.symbolicator.failed"); !ok || v.Bool() {
		t.Errorf("expected failed=false, got %v (present=%v)", v, ok)
	}
	if got := strAttr(attrs, "exception.symbolicator.parsing_method"); got != parsingMethodStructured {
		t.Errorf("parsing method: got %q", got)
	}

	urls, _ := sliceAttr(attrs, "exception.structured_stacktrace.urls")
	if urls.Len() != 1 || urls.At(0).Str() != "tests/fixtures/simple/original.js" {
		t.Errorf("urls not rewritten: %v", urls.AsRaw())
	}
	functions, _ := sliceAttr(attrs, "exception.structured_stacktrace.functions")
	if functions.At(0).Str() != "abcd" {
		t.Errorf("function not resolved from bundle scope analysis: %v", functions.AsRaw())
	}
	lines, _ := sliceAttr(attrs, "exception.structured_stacktrace.lines")
	columns, _ := sliceAttr(attrs, "exception.structured_stacktrace.columns")
	if lines.At(0).Int() != 2 || columns.At(0).Int() != 10 {
		t.Errorf("position not rewritten: %d:%d", lines.At(0).Int(), columns.At(0).Int())
	}
	origUrls, _ := sliceAttr(attrs, "exception.structured_stacktrace.urls.original")
	if origUrls.Len() != 1 || origUrls.At(0).Str() != "https://cdn.example.com/assets/minified.js" {
		t.Errorf("original urls not preserved: %v", origUrls.AsRaw())
	}
}

func TestProcessTracesParsedRouteOnSpanEvent(t *testing.T) {
	storeDir := stageStore(t, "")
	p := newTestProcessor(t, func(cfg *Config) {
		cfg.LocalSourceMaps.Path = storeDir
	})

	td := ptrace.NewTraces()
	rs := td.ResourceSpans().AppendEmpty()
	span := rs.ScopeSpans().AppendEmpty().Spans().AppendEmpty()
	event := span.Events().AppendEmpty()
	event.SetName("exception")
	event.Attributes().PutStr("exception.stacktrace", "Error: boom\n    at t (https://cdn.example.com/minified.js:1:11)")

	if _, err := p.processTraces(context.Background(), td); err != nil {
		t.Fatal(err)
	}

	attrs := event.Attributes()
	if got := strAttr(attrs, "exception.stacktrace"); got != "    at abcd(tests/fixtures/simple/original.js:2:10)" {
		t.Errorf("stacktrace: got %q", got)
	}
	if got := strAttr(attrs, "exception.symbolicator.parsing_method"); got != parsingMethodParsed {
		t.Errorf("parsing method: got %q", got)
	}
}

func TestProcessLogs(t *testing.T) {
	storeDir := stageStore(t, "")
	p := newTestProcessor(t, func(cfg *Config) {
		cfg.LocalSourceMaps.Path = storeDir
	})

	ld := plog.NewLogs()
	rl := ld.ResourceLogs().AppendEmpty()
	record := rl.ScopeLogs().AppendEmpty().LogRecords().AppendEmpty()
	putStructuredFrame(record.Attributes())

	if _, err := p.processLogs(context.Background(), ld); err != nil {
		t.Fatal(err)
	}

	if got := strAttr(record.Attributes(), "exception.stacktrace"); got != "Error: boom\n    at abcd(tests/fixtures/simple/original.js:2:10)" {
		t.Errorf("log stacktrace: got %q", got)
	}
}

func TestBuildUUIDPrefix(t *testing.T) {
	storeDir := stageStore(t, "build-uuid-1")
	p := newTestProcessor(t, func(cfg *Config) {
		cfg.LocalSourceMaps.Path = storeDir
	})

	td := ptrace.NewTraces()
	rs := td.ResourceSpans().AppendEmpty()
	rs.Resource().Attributes().PutStr("app.debug.source_map_uuid", "build-uuid-1")
	span := rs.ScopeSpans().AppendEmpty().Spans().AppendEmpty()
	putStructuredFrame(span.Attributes())

	if _, err := p.processTraces(context.Background(), td); err != nil {
		t.Fatal(err)
	}

	if got := strAttr(span.Attributes(), "exception.stacktrace"); got != "Error: boom\n    at abcd(tests/fixtures/simple/original.js:2:10)" {
		t.Errorf("uuid-prefixed lookup failed: %q", got)
	}
}

func TestMissingArtifactMarksFailed(t *testing.T) {
	p := newTestProcessor(t, func(cfg *Config) {
		cfg.LocalSourceMaps.Path = t.TempDir()
	})

	td := ptrace.NewTraces()
	span := td.ResourceSpans().AppendEmpty().ScopeSpans().AppendEmpty().Spans().AppendEmpty()
	putStructuredFrame(span.Attributes())

	if _, err := p.processTraces(context.Background(), td); err != nil {
		t.Fatal(err)
	}

	attrs := span.Attributes()
	if v, ok := attrs.Get("exception.symbolicator.failed"); !ok || !v.Bool() {
		t.Error("expected failed=true for missing artifacts")
	}
	if strAttr(attrs, "exception.symbolicator.error") == "" {
		t.Error("expected error attribute set")
	}
	columns, _ := sliceAttr(attrs, "exception.structured_stacktrace.columns")
	if columns.At(0).Int() != -1 {
		t.Errorf("expected -1 column sentinel, got %d", columns.At(0).Int())
	}
}

func TestMismatchedArraysPreserveOriginals(t *testing.T) {
	p := newTestProcessor(t, func(cfg *Config) {
		cfg.LocalSourceMaps.Path = t.TempDir()
	})

	td := ptrace.NewTraces()
	span := td.ResourceSpans().AppendEmpty().ScopeSpans().AppendEmpty().Spans().AppendEmpty()
	attrs := span.Attributes()
	putStructuredFrame(attrs)
	attrs.PutEmptySlice("exception.structured_stacktrace.columns")

	if _, err := p.processTraces(context.Background(), td); err != nil {
		t.Fatal(err)
	}

	if v, ok := attrs.Get("exception.symbolicator.failed"); !ok || !v.Bool() {
		t.Error("expected failed=true for mismatched arrays")
	}
	urls, _ := sliceAttr(attrs, "exception.structured_stacktrace.urls")
	if urls.Len() != 1 {
		t.Error("mismatched input must not destroy the original arrays")
	}
}

func TestAllowedLanguagesGate(t *testing.T) {
	storeDir := stageStore(t, "")
	p := newTestProcessor(t, func(cfg *Config) {
		cfg.LocalSourceMaps.Path = storeDir
		cfg.AllowedLanguages = []string{"webjs"}
	})

	td := ptrace.NewTraces()
	rs := td.ResourceSpans().AppendEmpty()
	rs.Resource().Attributes().PutStr("telemetry.sdk.language", "python")
	span := rs.ScopeSpans().AppendEmpty().Spans().AppendEmpty()
	putStructuredFrame(span.Attributes())

	if _, err := p.processTraces(context.Background(), td); err != nil {
		t.Fatal(err)
	}

	if _, ok := span.Attributes().Get("exception.symbolicator.failed"); ok {
		t.Error("expected record skipped for disallowed language")
	}
}

func TestDiskCacheServesSecondProcessor(t *testing.T) {
	storeDir := stageStore(t, "")
	cacheDir := t.TempDir()

	first := newTestProcessor(t, func(cfg *Config) {
		cfg.LocalSourceMaps.Path = storeDir
		cfg.CacheDir = cacheDir
	})
	td := ptrace.NewTraces()
	span := td.ResourceSpans().AppendEmpty().ScopeSpans().AppendEmpty().Spans().AppendEmpty()
	putStructuredFrame(span.Attributes())
	if _, err := first.processTraces(context.Background(), td); err != nil {
		t.Fatal(err)
	}

	entries, err := os.ReadDir(cacheDir)
	if err != nil || len(entries) != 1 {
		t.Fatalf("expected one .tw cache file, got %d (err=%v)", len(entries), err)
	}

	second := newTestProcessor(t, func(cfg *Config) {
		cfg.LocalSourceMaps.Path = t.TempDir()
		cfg.CacheDir = cacheDir
	})
	td2 := ptrace.NewTraces()
	span2 := td2.ResourceSpans().AppendEmpty().ScopeSpans().AppendEmpty().Spans().AppendEmpty()
	putStructuredFrame(span2.Attributes())
	if _, err := second.processTraces(context.Background(), td2); err != nil {
		t.Fatal(err)
	}

	if got := strAttr(span2.Attributes(), "exception.stacktrace"); got != "Error: boom\n    at abcd(tests/fixtures/simple/original.js:2:10)" {
		t.Errorf("expected resolution served from the .tw disk cache without store access, got %q", got)
	}
}

func TestConfigValidate(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	if err := cfg.Validate(); err != nil {
		t.Errorf("default config must validate: %v", err)
	}
	cfg.CacheMaxDiskPct = 101
	if err := cfg.Validate(); err == nil {
		t.Error("expected error for pct > 100")
	}
	cfg.CacheMaxDiskPct = 0
	cfg.SourceMapStoreKey = "redis_store"
	if err := cfg.Validate(); err == nil {
		t.Error("expected error for unknown store")
	}
}
