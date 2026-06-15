package sourcemap

import (
	"testing"

	"github.com/tracewayapp/traceway/backend/app/symbolicator/sourcemap/scopes"
)

func BenchmarkBuildTW(b *testing.B) {
	mapBytes := mustRead(b, fixture(b, "sourcemapcache", "preact.module.js.map"))
	bundle := mustRead(b, fixture(b, "sourcemapcache", "preact.module.js"))

	for _, parserName := range scopes.AvailableParsers() {
		b.Run("preact/"+parserName, func(b *testing.B) {
			original := scopes.ActiveParser()
			defer func() { _ = scopes.SetParser(original) }()
			if err := scopes.SetParser(parserName); err != nil {
				b.Skipf("parser %s not available in this build", parserName)
			}
			b.SetBytes(int64(len(mapBytes) + len(bundle)))
			b.ReportAllocs()
			b.ResetTimer()
			for b.Loop() {
				if _, err := BuildTW(mapBytes, bundle); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

func BenchmarkLookupTW(b *testing.B) {
	mapBytes := mustRead(b, fixture(b, "sourcemapcache", "preact.module.js.map"))
	bundle := mustRead(b, fixture(b, "sourcemapcache", "preact.module.js"))
	tw, err := BuildTW(mapBytes, bundle)
	if err != nil {
		b.Fatal(err)
	}

	b.SetBytes(int64(len(tw)))
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		if _, ok := LookupTW(tw, 0, 132); !ok {
			b.Fatal("expected a mapping")
		}
	}
}
