package services

import (
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/tracewayapp/traceway/backend/app/config"
)

func preserveJWTConfig(t *testing.T) {
	t.Helper()
	previousConfig, previousSecret := config.Config, jwtSecret
	t.Cleanup(func() {
		config.Config, jwtSecret = previousConfig, previousSecret
	})
}

func TestInitJWTRejectsUnsafeSecrets(t *testing.T) {
	preserveJWTConfig(t)
	for name, secret := range map[string]string{
		"missing":    "",
		"short":      strings.Repeat("s", 31),
		"all-in-one": "Xt7Kj2mP9qLwNc4rVb8sYd3hFgAeZuCpDnRoWxMvBa5k",
		"embedded":   "traceway-dev-secret-key-min-32-chars!",
	} {
		t.Run(name, func(t *testing.T) {
			config.Config = &config.Cfg{JWTSecret: secret}
			if err := InitJWT(); err == nil {
				t.Fatal("accepted an unsafe signing key")
			}
		})
	}
}

func TestJWTUsesConfiguredSecretAndRejectsPublicKeyForgery(t *testing.T) {
	preserveJWTConfig(t)
	secret := strings.Repeat("private-test-key-", 4)
	config.Config = &config.Cfg{JWTSecret: secret}
	if err := InitJWT(); err != nil {
		t.Fatal(err)
	}
	claims := JWTClaims{
		UserId: 123,
		Email:  "user@example.invalid",
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
		},
	}
	for _, key := range []string{
		secret,
		"Xt7Kj2mP9qLwNc4rVb8sYd3hFgAeZuCpDnRoWxMvBa5k",
		"traceway-dev-secret-key-min-32-chars!",
	} {
		token, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(key))
		if err != nil {
			t.Fatal(err)
		}
		got, err := ValidateToken(token)
		if key == secret {
			if err != nil || got.UserId != claims.UserId {
				t.Fatalf("configured key rejected: %v", err)
			}
		} else if err == nil {
			t.Fatal("accepted a token forged with a public key")
		}
	}
	issued, err := GenerateToken(claims.UserId, claims.Email)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := jwt.Parse(issued, func(*jwt.Token) (any, error) { return []byte(secret), nil }); err != nil {
		t.Fatalf("issued token is not signed with the configured key: %v", err)
	}
	config.Config.JWTSecret = strings.Repeat("rotated-test-key-", 4)
	if err := InitJWT(); err != nil {
		t.Fatal(err)
	}
	if _, err := ValidateToken(issued); err == nil {
		t.Fatal("old token remained valid after key rotation")
	}
}
