package otelprocessor

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"go.opentelemetry.io/collector/pdata/pcommon"
)

const androidFixtureDir = "../android/fixtures"

func androidFixture(t *testing.T, parts ...string) []byte {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(append([]string{androidFixtureDir}, parts...)...))
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func TestSymbolicateAndroidR8(t *testing.T) {
	for _, mode := range []string{"memory", "disk"} {
		t.Run(mode, func(t *testing.T) {
			const proguardUUID = "6A8CB813-45F6-3652-AD33-778FD1EAB196"
			store := t.TempDir()
			if err := os.WriteFile(filepath.Join(store, proguardUUID+".txt"), androidFixture(t, "r8", "mapping.txt"), 0o644); err != nil {
				t.Fatal(err)
			}

			p := newTestProcessor(t, func(c *Config) {
				c.SourceMapStoreKey = fileStoreKey
				c.LocalSourceMaps = LocalSourceMapsConfig{Path: store}
				if mode == "disk" {
					c.CacheDir = t.TempDir()
					c.CacheMaxMB = 64
				}
			})
			if (p.cache.Dir() != "") != (mode == "disk") {
				t.Fatalf("cache mode mismatch for %q (dir=%q)", mode, p.cache.Dir())
			}

			attrs := pcommon.NewMap()
			attrs.PutStr(p.cfg.StackTraceAttributeKey, string(androidFixture(t, "r8", "obfuscated.txt")))
			resource := pcommon.NewMap()
			resource.PutStr(p.cfg.LanguageAttributeKey, "android")
			resource.PutStr(p.cfg.ProguardUUIDAttributeKey, proguardUUID)

			p.processRecord(context.Background(), attrs, resource)

			got := strAttr(attrs, p.cfg.StackTraceAttributeKey)
			if strings.Contains(got, "a.a.a(") {
				t.Errorf("trace still obfuscated:\n%s", got)
			}
			for _, want := range []string{"com.example.demo.Checkout.chargeCard", "Checkout.java"} {
				if !strings.Contains(got, want) {
					t.Errorf("missing %q in retraced output:\n%s", want, got)
				}
			}
			if v, ok := attrs.Get(p.cfg.SymbolicatorFailureAttributeKey); !ok || v.Bool() {
				t.Errorf("expected symbolicator.failed=false, got %v", v)
			}
		})
	}
}
