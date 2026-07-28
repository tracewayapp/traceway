package controllers

import (
	"errors"
	"fmt"
	"testing"
)

func TestIsExpectedOAuthAuthError(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{
			name: "nil error",
			err:  nil,
			want: false,
		},
		{
			name: "missing session (the reported production error)",
			err:  errors.New("could not find a matching session for this request"),
			want: true,
		},
		{
			name: "missing session wrapped as the controller reports it",
			err:  fmt.Errorf("OAuth complete failed (provider=%s): %w", "github", errors.New("could not find a matching session for this request")),
			want: true,
		},
		{
			name: "state token mismatch",
			err:  errors.New("state token mismatch"),
			want: true,
		},
		{
			name: "provider token exchange failure is unexpected",
			err:  errors.New("github cannot get user information without accessToken"),
			want: false,
		},
		{
			name: "generic server error is unexpected",
			err:  errors.New("dial tcp: connection refused"),
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isExpectedOAuthAuthError(tt.err); got != tt.want {
				t.Errorf("isExpectedOAuthAuthError(%v) = %v, want %v", tt.err, got, tt.want)
			}
		})
	}
}
