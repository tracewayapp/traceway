package main

import (
	"strings"
	"testing"

	"github.com/tracewayapp/traceway/cli/v2/internal/output"
	"github.com/tracewayapp/traceway/cli/v2/pkg/client"
)

func TestClassifyError_tokenExpired_noTrailingSpaceWhenProfileEmpty(t *testing.T) {
	flagProfile = ""
	defer func() { flagProfile = "" }()

	env := classifyError(client.ErrUnauthorized, false)
	if env.Code != "token_expired" {
		t.Fatalf("Code = %q, want token_expired", env.Code)
	}
	if env.Hint == "" {
		t.Fatal("expected non-empty hint")
	}
	if strings.HasSuffix(env.Hint, " ") {
		t.Errorf("hint has trailing space: %q", env.Hint)
	}
	if strings.Contains(env.Hint, "--profile ") && !strings.Contains(env.Hint, "--profile X") {
		// Must not be "--profile " with empty value
		if strings.HasSuffix(env.Hint, "--profile ") || strings.HasSuffix(env.Hint, "--profile") {
			t.Errorf("hint references --profile with no value: %q", env.Hint)
		}
	}
}

func TestClassifyError_tokenExpired_includesProfileWhenSet(t *testing.T) {
	flagProfile = "stormwind"
	defer func() { flagProfile = "" }()

	env := classifyError(client.ErrUnauthorized, false)
	if !strings.Contains(env.Hint, "stormwind") {
		t.Errorf("hint should include profile name 'stormwind', got: %q", env.Hint)
	}
}

// silence unused-import warning in case output isn't actually used
var _ output.ErrorEnvelope
