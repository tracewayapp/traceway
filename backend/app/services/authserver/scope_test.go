//go:build !telemetry_ch && !telemetry_duckdb

package authserver

import (
	"testing"
	"time"

	"github.com/tracewayapp/traceway/backend/app/db"
	"github.com/tracewayapp/traceway/backend/app/models"
	"github.com/tracewayapp/traceway/backend/app/repositories/transactional"
	"github.com/tracewayapp/traceway/backend/app/services"
)

func accessScope(t *testing.T, access string) string {
	t.Helper()
	claims, err := services.ValidateToken(access)
	if err != nil {
		t.Fatalf("validate access token: %v", err)
	}
	return claims.Scope
}

func approvedDeviceToken(t *testing.T, clientId, userCode string) *models.TokenSetResponse {
	t.Helper()
	deviceCode := "dev-" + clientId
	if err := transactional.DeviceAuthorizationRepository.Create(db.DB, deviceCode, userCode, clientId, 5, time.Now().Add(deviceCodeTTL)); err != nil {
		t.Fatalf("create device auth: %v", err)
	}
	if _, err := transactional.DeviceAuthorizationRepository.Approve(db.DB, userCode, 1); err != nil {
		t.Fatalf("approve: %v", err)
	}
	ts, err := PollDeviceToken(deviceCode)
	if err != nil {
		t.Fatalf("poll: %v", err)
	}
	return ts
}

func TestDeviceFlow_scopeFollowsClient(t *testing.T) {
	setupAuthDB(t)

	read := approvedDeviceToken(t, cliClientID, "READ-AAAA")
	if read.Scope != models.ScopeRead || accessScope(t, read.AccessToken) != models.ScopeRead {
		t.Errorf("traceway-cli login scope = %q / %q, want read", read.Scope, accessScope(t, read.AccessToken))
	}

	write := approvedDeviceToken(t, cliWriteClientID, "WRIT-BBBB")
	if write.Scope != models.ScopeWrite || accessScope(t, write.AccessToken) != models.ScopeWrite {
		t.Errorf("traceway-cli-write login scope = %q, want write", write.Scope)
	}

	rotated, err := RotateRefresh(write.RefreshToken)
	if err != nil {
		t.Fatalf("rotate: %v", err)
	}
	if rotated.Scope != models.ScopeWrite || accessScope(t, rotated.AccessToken) != models.ScopeWrite {
		t.Errorf("refresh must keep the write scope, got %q", rotated.Scope)
	}

	rotatedRead, err := RotateRefresh(read.RefreshToken)
	if err != nil {
		t.Fatalf("rotate read: %v", err)
	}
	if rotatedRead.Scope != models.ScopeRead {
		t.Errorf("refresh must never upgrade a read token, got %q", rotatedRead.Scope)
	}
}

func TestAuthCode_scopeComesFromTheRequest(t *testing.T) {
	setupAuthDB(t)
	client := registerTestClient(t)

	for _, tc := range []struct {
		requested, want string
	}{
		{"", models.ScopeRead},
		{"traceway:read", models.ScopeRead},
		{"openid traceway:write", models.ScopeWrite},
		{"traceway:admin", models.ScopeRead},
	} {
		redirectTo := approve(t, models.AuthorizeApproveRequest{
			ClientId:            client.Id,
			RedirectUri:         "http://127.0.0.1:8123/callback",
			CodeChallenge:       testChallenge(testVerifier),
			CodeChallengeMethod: "S256",
			Scope:               tc.requested,
		})
		code, _ := codeFromRedirect(t, redirectTo)
		ts, err := RedeemAuthorizationCode(client.Id, code, testVerifier, "http://127.0.0.1:8123/callback", "", "")
		if err != nil {
			t.Fatalf("redeem (%q): %v", tc.requested, err)
		}
		if ts.Scope != tc.want || accessScope(t, ts.AccessToken) != tc.want {
			t.Errorf("requested %q got scope %q, want %q", tc.requested, ts.Scope, tc.want)
		}
	}
}
