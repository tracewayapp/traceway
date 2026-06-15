package otelprocessor

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tracewayapp/traceway/backend/app/symbolicator/dart"
	"go.opentelemetry.io/collector/pdata/pcommon"
)

const dartSeedDir = "../../../../benchmarks/processor/seeds/dart"

func TestSymbolicateDartTrace(t *testing.T) {
	elf, err := os.ReadFile(filepath.Join(dartSeedDir, "app.debug.elf"))
	if err != nil {
		t.Skipf("dart seed not available (%v)", err)
	}
	rawBytes, err := os.ReadFile(filepath.Join(dartSeedDir, "trace.txt"))
	if err != nil {
		t.Skipf("dart seed trace not available (%v)", err)
	}
	raw := string(rawBytes)

	tr := dart.ParseTrace(raw)
	if tr.BuildID == "" || tr.Arch == "" {
		t.Fatalf("seed trace missing build_id/arch: %+v", tr)
	}
	store := t.TempDir()
	symbolsName := dart.NormalizeDebugID(tr.BuildID) + "-" + dart.NormalizeArch(tr.Arch) + ".symbols"
	if err := os.WriteFile(filepath.Join(store, symbolsName), elf, 0o644); err != nil {
		t.Fatal(err)
	}

	p := newTestProcessor(t, func(c *Config) {
		c.SourceMapStoreKey = fileStoreKey
		c.LocalSourceMaps = LocalSourceMapsConfig{Path: store}
	})

	attrs := pcommon.NewMap()
	attrs.PutStr(p.cfg.ExceptionTypeAttributeKey, "Bad state")
	attrs.PutStr(p.cfg.ExceptionMessageAttributeKey, "boom from level3")
	attrs.PutStr(p.cfg.StackTraceAttributeKey, raw)

	p.processRecord(context.Background(), attrs, pcommon.NewMap())

	got := strAttr(attrs, p.cfg.StackTraceAttributeKey)
	if strings.Contains(got, "_kDartIsolateSnapshotInstructions") {
		t.Errorf("expected all frames resolved, output still has the instruction symbol:\n%s", got)
	}
	if !strings.Contains(got, "crash.dart") {
		t.Errorf("expected resolved dart frames (crash.dart), got:\n%s", got)
	}
	if !strings.Contains(got, "level3 (file:///private/tmp/dartfix/crash.dart:3:3)") {
		t.Errorf("expected golden frame 0 'level3 (...crash.dart:3:3)', got:\n%s", got)
	}
	if v, ok := attrs.Get(p.cfg.SymbolicatorFailureAttributeKey); !ok || v.Bool() {
		t.Errorf("expected symbolicator.failed=false, got %v", v)
	}

	attrs2 := pcommon.NewMap()
	attrs2.PutStr(p.cfg.StackTraceAttributeKey, raw)
	p.processRecord(context.Background(), attrs2, pcommon.NewMap())
	if got2 := strAttr(attrs2, p.cfg.StackTraceAttributeKey); strings.Contains(got2, "_kDartIsolateSnapshotInstructions") {
		t.Errorf("warm-cache resolve regressed:\n%s", got2)
	}
}

func TestSymbolicateDartTraceMissingSymbols(t *testing.T) {
	rawBytes, err := os.ReadFile(filepath.Join(dartSeedDir, "trace.txt"))
	if err != nil {
		t.Skipf("dart seed trace not available (%v)", err)
	}
	raw := string(rawBytes)

	p := newTestProcessor(t, func(c *Config) {
		c.SourceMapStoreKey = fileStoreKey
		c.LocalSourceMaps = LocalSourceMapsConfig{Path: t.TempDir()}
	})

	attrs := pcommon.NewMap()
	attrs.PutStr(p.cfg.StackTraceAttributeKey, raw)
	p.processRecord(context.Background(), attrs, pcommon.NewMap())

	got := strAttr(attrs, p.cfg.StackTraceAttributeKey)
	if !strings.Contains(got, "_kDartIsolateSnapshotInstructions+") {
		t.Errorf("expected stable unresolved offset frames, got:\n%s", got)
	}
	if strings.Contains(got, "build_id:") || strings.Contains(got, "abs ") {
		t.Errorf("volatile header leaked into output:\n%s", got)
	}
	if v, ok := attrs.Get(p.cfg.SymbolicatorFailureAttributeKey); !ok || !v.Bool() {
		t.Errorf("expected symbolicator.failed=true for missing symbols")
	}
}
