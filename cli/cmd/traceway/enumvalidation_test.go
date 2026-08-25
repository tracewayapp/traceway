package main

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/tracewayapp/traceway/cli/internal/exitcode"
)

// TestEnumFlagValidation_acceptsAndRejects asserts that every enum-shaped flag
// across the read commands accepts each documented value and rejects unknown
// input with a usage_error envelope and exit code 2 — the same shape the
// missing-required-flag path produces.
func TestEnumFlagValidation_acceptsAndRejects(t *testing.T) {
	type validCase struct {
		name string
		args []string
	}
	type rejectCase struct {
		name            string
		args            []string
		expectFlagInMsg string
		expectAllowed   []string
	}

	emptyOK := func(_ http.ResponseWriter, _ *http.Request) {}

	tests := []struct {
		group     string
		emptyJSON string
		valid     []validCase
		reject    []rejectCase
	}{
		{
			group:     "exceptions list --search-type",
			emptyJSON: `{"data":[],"pagination":{}}`,
			valid: []validCase{
				{"text", []string{"exceptions", "list", "--search-type", "text", "--output", "json"}},
				{"regex", []string{"exceptions", "list", "--search-type", "regex", "--output", "json"}},
			},
			reject: []rejectCase{
				{
					name:            "bogus",
					args:            []string{"exceptions", "list", "--search-type", "bogus", "--output", "json"},
					expectFlagInMsg: "--search-type",
					expectAllowed:   []string{"text", "regex"},
				},
			},
		},
		{
			group:     "exceptions list --order-by",
			emptyJSON: `{"data":[],"pagination":{}}`,
			valid: []validCase{
				{"lastSeen", []string{"exceptions", "list", "--order-by", "lastSeen", "--output", "json"}},
				{"firstSeen", []string{"exceptions", "list", "--order-by", "firstSeen", "--output", "json"}},
				{"count", []string{"exceptions", "list", "--order-by", "count", "--output", "json"}},
			},
			reject: []rejectCase{
				{
					name:            "bogus",
					args:            []string{"exceptions", "list", "--order-by", "bogus", "--output", "json"},
					expectFlagInMsg: "--order-by",
					expectAllowed:   []string{"lastSeen", "firstSeen", "count"},
				},
			},
		},
		{
			group:     "logs query --search-type",
			emptyJSON: `{"data":[],"pagination":{}}`,
			valid: []validCase{
				{"body", []string{"logs", "query", "--search-type", "body", "--output", "json"}},
				{"attribute", []string{"logs", "query", "--search-type", "attribute", "--output", "json"}},
			},
			reject: []rejectCase{
				{
					name:            "bogus",
					args:            []string{"logs", "query", "--search-type", "bogus", "--output", "json"},
					expectFlagInMsg: "--search-type",
					expectAllowed:   []string{"body", "attribute"},
				},
			},
		},
		{
			group:     "logs query --sort-direction",
			emptyJSON: `{"data":[],"pagination":{}}`,
			valid: []validCase{
				{"asc", []string{"logs", "query", "--sort-direction", "asc", "--output", "json"}},
				{"desc", []string{"logs", "query", "--sort-direction", "desc", "--output", "json"}},
			},
			reject: []rejectCase{
				{
					name:            "bogus",
					args:            []string{"logs", "query", "--sort-direction", "sideways", "--output", "json"},
					expectFlagInMsg: "--sort-direction",
					expectAllowed:   []string{"asc", "desc"},
				},
			},
		},
		{
			group:     "endpoints list --order-by",
			emptyJSON: `{"data":[],"pagination":{}}`,
			valid: []validCase{
				{"impact", []string{"endpoints", "list", "--order-by", "impact", "--output", "json"}},
				{"count", []string{"endpoints", "list", "--order-by", "count", "--output", "json"}},
				{"p95", []string{"endpoints", "list", "--order-by", "p95", "--output", "json"}},
				{"lastSeen", []string{"endpoints", "list", "--order-by", "lastSeen", "--output", "json"}},
			},
			reject: []rejectCase{
				{
					name:            "bogus",
					args:            []string{"endpoints", "list", "--order-by", "bogus", "--output", "json"},
					expectFlagInMsg: "--order-by",
					expectAllowed:   []string{"impact", "count", "p95", "lastSeen"},
				},
			},
		},
		{
			group:     "endpoints list --sort-direction",
			emptyJSON: `{"data":[],"pagination":{}}`,
			valid: []validCase{
				{"asc", []string{"endpoints", "list", "--sort-direction", "asc", "--output", "json"}},
				{"desc", []string{"endpoints", "list", "--sort-direction", "desc", "--output", "json"}},
			},
			reject: []rejectCase{
				{
					name:            "bogus",
					args:            []string{"endpoints", "list", "--sort-direction", "sideways", "--output", "json"},
					expectFlagInMsg: "--sort-direction",
					expectAllowed:   []string{"asc", "desc"},
				},
			},
		},
	}

	for _, group := range tests {
		t.Run(group.group, func(t *testing.T) {
			for _, vc := range group.valid {
				t.Run("accepts/"+vc.name, func(t *testing.T) {
					body := group.emptyJSON
					srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
						emptyOK(w, r)
						_, _ = w.Write([]byte(body))
					}))
					defer srv.Close()
					seedSessionFor(t, srv.URL)

					_, stderr, err := runCmd(t, "", vc.args...)
					if err != nil {
						t.Fatalf("args %v rejected: %v\nstderr: %s", vc.args, err, stderr.String())
					}
				})
			}

			for _, rc := range group.reject {
				t.Run("rejects/"+rc.name, func(t *testing.T) {
					srv := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {
						t.Errorf("server should not be called for invalid enum input: args=%v", rc.args)
					}))
					defer srv.Close()
					seedSessionFor(t, srv.URL)

					_, stderr, err := runCmd(t, "", rc.args...)
					if err == nil {
						t.Fatalf("expected error for args %v", rc.args)
					}
					if !strings.Contains(stderr.String(), `"usage_error"`) {
						t.Errorf("expected usage_error envelope, got: %s", stderr.String())
					}
					if !strings.Contains(stderr.String(), rc.expectFlagInMsg) {
						t.Errorf("expected flag %q in error, got: %s", rc.expectFlagInMsg, stderr.String())
					}
					for _, want := range rc.expectAllowed {
						if !strings.Contains(stderr.String(), want) {
							t.Errorf("expected allowed value %q in error, got: %s", want, stderr.String())
						}
					}
					var ce *cliError
					if !errors.As(err, &ce) || ce.code != exitcode.Usage {
						t.Errorf("expected cliError(Usage=2), got %v", err)
					}
				})
			}
		})
	}
}
