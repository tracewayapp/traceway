//go:build oxc && cgo

package symbolicator

import (
	"testing"

	"github.com/tracewayapp/traceway/backend/app/symbolicator/scopes"
	"github.com/tracewayapp/traceway/backend/app/symbolicator/sourcemap"
)

func TestOxcGojaLookupEquivalence(t *testing.T) {
	original := scopes.ActiveParser()
	defer func() { _ = scopes.SetParser(original) }()

	for _, tc := range parityCases {
		if tc.minifiedPath == nil {
			continue
		}
		t.Run(tc.name, func(t *testing.T) {
			mapBytes := mustRead(t, fixture(t, tc.mapPath...))
			bundle := mustRead(t, fixture(t, tc.minifiedPath...))

			if err := scopes.SetParser("goja"); err != nil {
				t.Fatal(err)
			}
			gojaResolver, err := NewResolver(mapBytes, bundle)
			if err != nil {
				t.Fatalf("NewResolver(goja): %v", err)
			}

			if err := scopes.SetParser("oxc"); err != nil {
				t.Fatal(err)
			}
			oxcResolver, err := NewResolver(mapBytes, bundle)
			if err != nil {
				t.Fatalf("NewResolver(oxc): %v", err)
			}

			parsed, err := sourcemap.Parse(mapBytes)
			if err != nil {
				t.Fatalf("parsing source map: %v", err)
			}

			mismatches := 0
			for _, token := range parsed.Tokens {
				gFrame, gOk := gojaResolver.Lookup(token.GenLine, token.GenCol)
				oFrame, oOk := oxcResolver.Lookup(token.GenLine, token.GenCol)
				if gOk != oOk || gFrame != oFrame {
					mismatches++
					if mismatches <= 10 {
						t.Errorf("lookup(%d,%d): goja=(%+v,%v) oxc=(%+v,%v)", token.GenLine, token.GenCol, gFrame, gOk, oFrame, oOk)
					}
				}
			}
			if mismatches > 10 {
				t.Errorf("%d total mismatches", mismatches)
			}
		})
	}
}
