//go:build write

package main

import (
	"net/http"
	"strings"
	"testing"
)

func TestTokensCreate_confirmsThenCreatesAReadToken(t *testing.T) {
	rec := &recorded{}
	srv := newTestServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rec.add(r)
		if r.Method == http.MethodPost && r.URL.Query().Get("projectId") == "" {
			t.Errorf("token creation must name the project: %s", r.URL)
		}
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"id":"t-1","name":"Embed","prefix":"twp_abcdefgh","token":"twp_secret","scope":"traceway:read","expiresAt":null}`))
	}))
	defer srv.Close()
	seedSessionFor(t, srv.URL)
	t.Cleanup(func() { flagYes = false })

	if _, _, err := runCmd(t, "", "tokens", "create", "--name", "Embed", "--output", "json"); err == nil || len(rec.calls) != 0 {
		t.Fatalf("a non-TTY create without --yes must be refused before any request, calls = %v", rec.calls)
	}
	for _, args := range [][]string{{"tokens", "create", "--yes"}, {"tokens", "create", "--name", "x", "--expires-in-days", "0", "--yes"}, {"tokens", "create", "--name", "x", "--expires-in-days", "4000", "--yes"}} {
		if _, _, err := runCmd(t, "", args...); err == nil || len(rec.calls) != 0 {
			t.Fatalf("%v must be refused before any request, calls = %v", args, rec.calls)
		}
	}
	stdout, stderr, err := runCmd(t, "", "tokens", "create", "--name", "Embed", "--yes", "--output", "json")
	if err != nil {
		t.Fatalf("create: %v %s", err, stderr.String())
	}
	if len(rec.calls) != 1 || rec.calls[0] != "POST /api/personal-access-tokens" || rec.body["scope"] != "read" || rec.body["name"] != "Embed" || rec.body["expiresInDays"] != nil {
		t.Errorf("calls = %v, body = %v", rec.calls, rec.body)
	}
	if !strings.Contains(stdout.String(), "twp_secret") {
		t.Errorf("stdout = %s", stdout.String())
	}
}
