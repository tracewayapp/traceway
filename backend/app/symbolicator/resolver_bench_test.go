package symbolicator

import (
	"testing"

	"github.com/tracewayapp/traceway/backend/app/symbolicator/scopes"
)

func BenchmarkNewResolver(b *testing.B) {
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
				if _, err := NewResolver(mapBytes, bundle); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

func BenchmarkOpenTW(b *testing.B) {
	mapBytes := mustRead(b, fixture(b, "sourcemapcache", "preact.module.js.map"))
	bundle := mustRead(b, fixture(b, "sourcemapcache", "preact.module.js"))
	resolver, err := NewResolver(mapBytes, bundle)
	if err != nil {
		b.Fatal(err)
	}
	tw := resolver.MarshalTW()

	b.SetBytes(int64(len(tw)))
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		if _, err := OpenTW(tw); err != nil {
			b.Fatal(err)
		}
	}
}
