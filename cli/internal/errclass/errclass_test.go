package errclass

import (
	"errors"
	"fmt"
	"net/url"
	"testing"

	"github.com/tracewayapp/traceway/cli/internal/exitcode"
	"github.com/tracewayapp/traceway/cli/pkg/client"
)

func TestClassify(t *testing.T) {
	cases := []struct {
		name     string
		err      error
		code     string
		exitCode int
	}{
		{"unauthorized", client.ErrUnauthorized, "token_expired", exitcode.Auth},
		{"wrapped unauthorized", fmt.Errorf("wrap: %w", client.ErrUnauthorized), "token_expired", exitcode.Auth},
		{"forbidden", client.ErrForbidden, "forbidden", exitcode.Auth},
		{"not found", client.ErrNotFound, "not_found", exitcode.NotFound},
		{"rate limited", client.ErrRateLimited, "rate_limited", exitcode.RateLimited},
		{"server error", &client.APIError{StatusCode: 502, Body: "bad gateway"}, "server_error", exitcode.Server},
		{"api error", &client.APIError{StatusCode: 422, Body: "nope"}, "api_error", exitcode.Generic},
		{"connection", &url.Error{Op: "Post", URL: "http://x", Err: errors.New("refused")}, "connection_failed", exitcode.Connection},
		{"unknown", errors.New("boom"), "internal", exitcode.Generic},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c := Classify(tc.err)
			if c.Code != tc.code {
				t.Errorf("Code = %q, want %q", c.Code, tc.code)
			}
			if c.ExitCode != tc.exitCode {
				t.Errorf("ExitCode = %d, want %d", c.ExitCode, tc.exitCode)
			}
			if c.Message == "" {
				t.Error("Message should not be empty")
			}
		})
	}
}
