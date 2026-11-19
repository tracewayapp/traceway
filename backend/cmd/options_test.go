package cmd

import (
	"encoding/hex"
	"os"
	"strings"
	"testing"
)

func TestEmbeddedJWTSecretIgnoresHostJWTSecret(t *testing.T) {
	t.Setenv("JWT_SECRET", "host-app-key")
	t.Setenv("TRACEWAY_JWT_SECRET", "")
	if err := os.Unsetenv("TRACEWAY_JWT_SECRET"); err != nil {
		t.Fatal(err)
	}
	if got := embeddedJWTSecret(&options{}); got == "host-app-key" {
		t.Fatal("embedded mode adopted the host application's JWT_SECRET")
	}
}

func TestEmbeddedJWTSecretHonorsTracewayEnvironment(t *testing.T) {
	for _, secret := range []string{"", "short", strings.Repeat("operator-test-key-", 4)} {
		t.Setenv("TRACEWAY_JWT_SECRET", secret)
		if got := embeddedJWTSecret(&options{}); got != secret {
			t.Fatal("embedded mode replaced an explicitly configured TRACEWAY_JWT_SECRET")
		}
	}
}

func TestEmbeddedJWTSecretOptionWins(t *testing.T) {
	t.Setenv("TRACEWAY_JWT_SECRET", strings.Repeat("environment-key-", 4))
	o := &options{}
	want := strings.Repeat("option-test-key-", 4)
	WithJWTSecret(want)(o)
	if got := embeddedJWTSecret(o); got != want {
		t.Fatal("WithJWTSecret did not take precedence over the environment")
	}
}

func TestEmbeddedJWTSecretGeneratesIndependentKeys(t *testing.T) {
	t.Setenv("TRACEWAY_JWT_SECRET", "")
	if err := os.Unsetenv("TRACEWAY_JWT_SECRET"); err != nil {
		t.Fatal(err)
	}
	first, second := embeddedJWTSecret(&options{}), embeddedJWTSecret(&options{})
	if first == second {
		t.Fatal("embedded starts shared a signing key")
	}
	for _, secret := range []string{first, second} {
		decoded, err := hex.DecodeString(secret)
		if err != nil || len(decoded) != 32 {
			t.Fatalf("expected a 256-bit key, got %d bytes: %v", len(decoded), err)
		}
	}
}
