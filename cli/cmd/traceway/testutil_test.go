package main

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// runCmd executes the given args against a fresh root command, with stdin/out/err
// captured into buffers. It returns the buffers and the error from Execute().
//
// Each test should also call t.Setenv("XDG_CONFIG_HOME", t.TempDir()) so that
// config writes are isolated.
func runCmd(t *testing.T, stdin string, args ...string) (stdout, stderr *bytes.Buffer, err error) {
	t.Helper()
	stdout = &bytes.Buffer{}
	stderr = &bytes.Buffer{}

	cmd := newRootCmd()
	cmd.SetIn(strings.NewReader(stdin))
	cmd.SetOut(stdout)
	cmd.SetErr(stderr)
	cmd.SetArgs(args)
	err = cmd.Execute()
	return
}

// readAll consumes a bytes.Buffer entirely; useful when a test needs the
// trailing bytes after Execute returned.
func readAll(t *testing.T, r io.Reader) string { //nolint:unused
	t.Helper()
	b, err := io.ReadAll(r)
	if err != nil {
		t.Fatalf("readAll: %v", err)
	}
	return string(b)
}

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
