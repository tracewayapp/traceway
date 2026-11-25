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
		"/u/3f2b8c1e-9a4d-4c7e-8b1a-2d3e4f5a6b7c/edit":               "/u/:id/edit",
		"/commit/9fceb02d0ae598e9":                                   "/commit/:id",
		"/reset/dGhpcyBpcyBhIHRva2VuIHZhbHVl0":                       "/reset/:id",
		"/products/blue-suede-shoes":                                 "/products/blue-suede-shoes",
		"/deadbeef":                                                  "/deadbeef",
		"/a/b/c/d/e/f/g/h":                                           "/a/b/c/d/e/f/*",
		"/search/hello%20world":                                      "/search/:id",
		"/endpoints/POST%20%2Ftasks%2F%7BtaskId%7D/9fceb02d0ae598e9": "/endpoints/:id/:id",
		"/caf%C3%A9/menu":                                            "/caf%C3%A9/menu",
		"/:lang?/about":                                              "/:lang?/about",
		"/:locale(en|fr)?/products/:slug":                            "/:locale(en|fr)?/products/:slug",
		"/users/:id?":                                                "/users/:id",
		"/orders/8841?/next":                                         "/orders/:id",
		// Route templates as real routers write them (Nuxt, Next.js, TanStack) pass unchanged.
		"/products/:slug()": "/products/:slug()",
		"/docs/:slug(.*)*":  "/docs/:slug(.*)*",
		"/docs/:slug*":      "/docs/:slug*",
		"/posts/$postId":    "/posts/$postId",
		"/posts/[id]":       "/posts/[id]",
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
