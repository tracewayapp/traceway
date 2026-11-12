package client

import (
	"net/http"
	"net/http/httptest"
	"strings"
)

// newTestServer serves handler to the CLI's own requests and answers anything
// else with 404. A listener on localhost is reachable by every process on the
// machine, and a request from one of them (a tool probing new ports with GET /)
// would otherwise land in a handler that fails on unexpected paths or records
// the last request it saw. Every CLI request carries the traceway-cli
// User-Agent (client.Client.do).
func newTestServer(handler http.Handler) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.UserAgent(), "traceway-cli/") {
			http.NotFound(w, r)
			return
		}
		handler.ServeHTTP(w, r)
	}))
}
