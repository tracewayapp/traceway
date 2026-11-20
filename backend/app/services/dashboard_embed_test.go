package services

import (
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

func TestDashboardEmbedTokenBoundaries(t *testing.T) {
	preserveJWTConfig(t)
	jwtSecret = []byte(t.Name() + "-test-signing-key")
	claims, token, err := GenerateDashboardEmbed(1, uuid.New(), DashboardEmbedView{})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ValidateToken(token); err == nil {
		t.Fatal("embed accepted as app session")
	}
	if _, err := ValidateDashboardEmbed(token, claims.Subject); err != nil {
		t.Fatal(err)
	}
	key, err := dashboardEmbedKey()
	if err != nil {
		t.Fatal(err)
	}
	for _, change := range []func(*DashboardEmbedClaims){
		func(c *DashboardEmbedClaims) { c.ExpiresAt = nil },
		func(c *DashboardEmbedClaims) { c.ExpiresAt = jwt.NewNumericDate(time.Now().Add(-time.Second)) },
		func(c *DashboardEmbedClaims) { c.Audience = jwt.ClaimStrings{"other"} },
	} {
		invalid := *claims
		change(&invalid)
		raw, err := jwt.NewWithClaims(jwt.SigningMethodHS256, &invalid).SignedString(key)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := ValidateDashboardEmbed(raw, claims.Subject); err == nil {
			t.Fatal("accepted invalid claims")
		}
	}
	jwtSecret = []byte(t.Name() + "-rotated-test-signing-key")
	if _, err := ValidateDashboardEmbed(token, claims.Subject); err == nil {
		t.Fatal("accepted token after key rotation")
	}
}
