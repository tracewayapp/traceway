package clientmetrics

import (
	"strings"
	"testing"
)

// The same table as js-client packages/core/src/metrics/metrics.test.ts.
func TestNormalizePath(t *testing.T) {
	for input, want := range map[string]string{
		"/":                  "/",
		"":                   "/",
		"/orders/8841":       "/orders/:id",
		"/orders/8841/":      "/orders/:id",
		"/orders/8841?x=1#y": "/orders/:id",
		"/u/3f2b8c1e-9a4d-4c7e-8b1a-2d3e4f5a6b7c/edit": "/u/:id/edit",
		"/commit/9fceb02d0ae598e9":                     "/commit/:id",
		"/reset/dGhpcyBpcyBhIHRva2VuIHZhbHVl0":         "/reset/:id",
		"/products/blue-suede-shoes":                   "/products/blue-suede-shoes",
		"/deadbeef":                                    "/deadbeef",
		"/a/b/c/d/e/f/g/h":                             "/a/b/c/d/e/f/*",
	} {
		if got := NormalizePath(input); got != want {
			t.Errorf("NormalizePath(%q) = %q, want %q", input, got, want)
		}
	}
	long := NormalizePath("/" + strings.Repeat("segment-that-is-long/", 8))
	if len(long) > MaxDimensionLength || !strings.HasSuffix(long, "/*") {
		t.Errorf("long path became %q", long)
	}
}
