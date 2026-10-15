package mcpmount

import (
	"slices"
	"testing"
	"time"

	"github.com/tracewayapp/traceway/backend/app/config"
	"github.com/tracewayapp/traceway/backend/app/models"
	"github.com/tracewayapp/traceway/backend/app/services"
)

func TestVerifyBearer_writeScopeOnlyForAgentWriteCredentials(t *testing.T) {
	previous := config.Config
	config.Config = &config.Cfg{JWTSecret: "mcpmount-test-secret-0123456789abcdefgh"}
	t.Cleanup(func() { config.Config = previous })
	if err := services.InitJWT(); err != nil {
		t.Fatal(err)
	}

	session, err := services.GenerateToken(1, "a@example.com")
	if err != nil {
		t.Fatal(err)
	}
	read, err := services.GenerateAccessToken(1, "a@example.com", models.ScopeRead, "traceway-cli", 15*time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	write, err := services.GenerateAccessToken(1, "a@example.com", models.ScopeWrite, "traceway-cli-write", 15*time.Minute)
	if err != nil {
		t.Fatal(err)
	}

	for _, tc := range []struct {
		name      string
		token     string
		wantWrite bool
	}{
		{"dashboard session", session, false},
		{"read agent token", read, false},
		{"write agent token", write, true},
	} {
		info, err := verifyBearer(t.Context(), tc.token, nil)
		if err != nil {
			t.Fatalf("%s: %v", tc.name, err)
		}
		if !slices.Contains(info.Scopes, models.ScopeRead) {
			t.Errorf("%s: scopes %v must include read", tc.name, info.Scopes)
		}
		if got := slices.Contains(info.Scopes, models.ScopeWrite); got != tc.wantWrite {
			t.Errorf("%s: write scope = %v, want %v", tc.name, got, tc.wantWrite)
		}
	}
}
