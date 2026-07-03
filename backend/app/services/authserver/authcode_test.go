//go:build !pgch

package authserver

import (
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"errors"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/tracewayapp/traceway/backend/app/db"
	"github.com/tracewayapp/traceway/backend/app/models"
	"github.com/tracewayapp/traceway/backend/app/repositories"
)

const testVerifier = "dBjftJeZ4CVP-mB92K27uhbUJU1p1r_wW1gFWFOEjXk"

func testChallenge(verifier string) string {
	sum := sha256.Sum256([]byte(verifier))
	return base64.RawURLEncoding.EncodeToString(sum[:])
}

func registerTestClient(t *testing.T, uris ...string) *models.OauthClient {
	t.Helper()
	if len(uris) == 0 {
		uris = []string{"http://127.0.0.1:8123/callback"}
	}
	client, err := db.ExecuteTransaction(func(tx *sql.Tx) (*models.OauthClient, error) {
		return RegisterClient(tx, "Test MCP Client", uris)
	})
	if err != nil {
		t.Fatalf("register client: %v", err)
	}
	return client
}

func approve(t *testing.T, req models.AuthorizeApproveRequest) string {
	t.Helper()
	redirectTo, err := db.ExecuteTransaction(func(tx *sql.Tx) (string, error) {
		return ApproveAuthorization(tx, 1, req, "https://traceway.example.com")
	})
	if err != nil {
		t.Fatalf("approve authorization: %v", err)
	}
	return redirectTo
}

func codeFromRedirect(t *testing.T, redirectTo string) (code, state string) {
	t.Helper()
	u, err := url.Parse(redirectTo)
	if err != nil {
		t.Fatalf("parse redirect %q: %v", redirectTo, err)
	}
	return u.Query().Get("code"), u.Query().Get("state")
}

func TestRegisterClient_validation(t *testing.T) {
	setupAuthDB(t)

	cases := []struct {
		name string
		uris []string
	}{
		{"no uris", nil},
		{"http non-loopback", []string{"http://evil.example.com/cb"}},
		{"fragment", []string{"https://app.example.com/cb#frag"}},
		{"relative", []string{"/callback"}},
		{"empty", []string{""}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := db.ExecuteTransaction(func(tx *sql.Tx) (*models.OauthClient, error) {
				return RegisterClient(tx, "c", tc.uris)
			})
			if !errors.Is(err, ErrInvalidClientMetadata) {
				t.Errorf("expected ErrInvalidClientMetadata, got %v", err)
			}
		})
	}

	client := registerTestClient(t, "https://claude.ai/api/mcp/auth_callback", "http://localhost:3000/cb", "cursor://anysphere.cursor-mcp/callback")
	loaded, err := repositories.OauthClientRepository.FindById(db.DB, client.Id)
	if err != nil || loaded == nil {
		t.Fatalf("reload client: %v, %v", loaded, err)
	}
	if len(loaded.RedirectUris) != 3 || loaded.Name != "Test MCP Client" {
		t.Fatalf("client round trip mismatch: %+v", loaded)
	}
}

func TestAuthCode_happyPathAndSingleUse(t *testing.T) {
	setupAuthDB(t)
	client := registerTestClient(t)

	redirectTo := approve(t, models.AuthorizeApproveRequest{
		ClientId:            client.Id,
		RedirectUri:         "http://127.0.0.1:8123/callback",
		State:               "xyz-state",
		CodeChallenge:       testChallenge(testVerifier),
		CodeChallengeMethod: "S256",
		Resource:            "https://traceway.example.com/mcp",
	})
	code, state := codeFromRedirect(t, redirectTo)
	if code == "" || !strings.HasPrefix(code, "twa_") {
		t.Fatalf("redirect should carry a twa_ code, got %q", redirectTo)
	}
	if state != "xyz-state" {
		t.Fatalf("state should ride back on the redirect, got %q", state)
	}

	ts, err := RedeemAuthorizationCode(client.Id, code, testVerifier, "http://127.0.0.1:8123/callback")
	if err != nil {
		t.Fatalf("redeem: %v", err)
	}
	if ts.AccessToken == "" || ts.RefreshToken == "" || ts.TokenType != "Bearer" {
		t.Fatalf("unexpected token set: %+v", ts)
	}

	if _, err := RedeemAuthorizationCode(client.Id, code, testVerifier, "http://127.0.0.1:8123/callback"); !errors.Is(err, ErrInvalidGrant) {
		t.Fatalf("replayed code should be invalid_grant, got %v", err)
	}
}

func TestAuthCode_rejectsWrongVerifierClientAndRedirect(t *testing.T) {
	setupAuthDB(t)
	client := registerTestClient(t)

	mint := func() string {
		redirectTo := approve(t, models.AuthorizeApproveRequest{
			ClientId:            client.Id,
			RedirectUri:         "http://127.0.0.1:8123/callback",
			CodeChallenge:       testChallenge(testVerifier),
			CodeChallengeMethod: "S256",
		})
		code, _ := codeFromRedirect(t, redirectTo)
		return code
	}

	if _, err := RedeemAuthorizationCode(client.Id, mint(), "wrong-verifier-wrong-verifier-wrong-verifier-x", "http://127.0.0.1:8123/callback"); !errors.Is(err, ErrInvalidGrant) {
		t.Errorf("wrong verifier should be invalid_grant, got %v", err)
	}
	if _, err := RedeemAuthorizationCode("other-client", mint(), testVerifier, "http://127.0.0.1:8123/callback"); !errors.Is(err, ErrInvalidGrant) {
		t.Errorf("wrong client should be invalid_grant, got %v", err)
	}
	if _, err := RedeemAuthorizationCode(client.Id, mint(), testVerifier, "http://127.0.0.1:9999/other"); !errors.Is(err, ErrInvalidGrant) {
		t.Errorf("wrong redirect should be invalid_grant, got %v", err)
	}
	if _, err := RedeemAuthorizationCode(client.Id, "twa_bogus", testVerifier, "http://127.0.0.1:8123/callback"); !errors.Is(err, ErrInvalidGrant) {
		t.Errorf("unknown code should be invalid_grant, got %v", err)
	}
}

func TestAuthCode_expiredCodeRejected(t *testing.T) {
	setupAuthDB(t)
	client := registerTestClient(t)

	_, err := db.ExecuteTransaction(func(tx *sql.Tx) (struct{}, error) {
		return struct{}{}, repositories.AuthorizationCodeRepository.Create(
			tx, "twa_expired", client.Id, 1, "http://127.0.0.1:8123/callback", testChallenge(testVerifier), time.Now().Add(-time.Minute))
	})
	if err != nil {
		t.Fatalf("insert expired code: %v", err)
	}

	if _, err := RedeemAuthorizationCode(client.Id, "twa_expired", testVerifier, "http://127.0.0.1:8123/callback"); !errors.Is(err, ErrInvalidGrant) {
		t.Fatalf("expired code should be invalid_grant, got %v", err)
	}
	if got, _ := repositories.AuthorizationCodeRepository.FindByCode(db.DB, "twa_expired"); got != nil {
		t.Fatal("expired code should be consumed even on a failed exchange")
	}
}

func TestApproveAuthorization_validation(t *testing.T) {
	setupAuthDB(t)
	client := registerTestClient(t)
	base := models.AuthorizeApproveRequest{
		ClientId:            client.Id,
		RedirectUri:         "http://127.0.0.1:8123/callback",
		CodeChallenge:       testChallenge(testVerifier),
		CodeChallengeMethod: "S256",
	}

	run := func(mutate func(*models.AuthorizeApproveRequest)) error {
		req := base
		mutate(&req)
		_, err := db.ExecuteTransaction(func(tx *sql.Tx) (string, error) {
			return ApproveAuthorization(tx, 1, req, "https://traceway.example.com")
		})
		return err
	}

	if err := run(func(r *models.AuthorizeApproveRequest) { r.ClientId = "nope" }); !errors.Is(err, ErrUnknownClient) {
		t.Errorf("unknown client: got %v", err)
	}
	if err := run(func(r *models.AuthorizeApproveRequest) { r.RedirectUri = "https://evil.example.com/cb" }); !errors.Is(err, ErrInvalidRedirect) {
		t.Errorf("unregistered redirect: got %v", err)
	}
	if err := run(func(r *models.AuthorizeApproveRequest) { r.CodeChallengeMethod = "plain" }); !errors.Is(err, ErrInvalidChallenge) {
		t.Errorf("plain method: got %v", err)
	}
	if err := run(func(r *models.AuthorizeApproveRequest) { r.CodeChallenge = "short" }); !errors.Is(err, ErrInvalidChallenge) {
		t.Errorf("malformed challenge: got %v", err)
	}
	if err := run(func(r *models.AuthorizeApproveRequest) { r.Resource = "https://other.example.com/mcp" }); !errors.Is(err, ErrInvalidTarget) {
		t.Errorf("foreign resource: got %v", err)
	}

	// Loopback redirect URIs match on any port (RFC 8252 section 7.3).
	if err := run(func(r *models.AuthorizeApproveRequest) { r.RedirectUri = "http://127.0.0.1:60999/callback" }); err != nil {
		t.Errorf("loopback port change should be allowed: %v", err)
	}
}

func TestDenyAuthorization_buildsErrorRedirect(t *testing.T) {
	setupAuthDB(t)
	client := registerTestClient(t)

	redirectTo, err := DenyAuthorization(db.DB, client.Id, "http://127.0.0.1:8123/callback", "st")
	if err != nil {
		t.Fatalf("deny: %v", err)
	}
	u, _ := url.Parse(redirectTo)
	if u.Query().Get("error") != "access_denied" || u.Query().Get("state") != "st" {
		t.Fatalf("deny redirect malformed: %q", redirectTo)
	}

	if _, err := DenyAuthorization(db.DB, client.Id, "https://evil.example.com/cb", ""); !errors.Is(err, ErrInvalidRedirect) {
		t.Fatalf("deny must validate the redirect target, got %v", err)
	}
}

func TestValidateResource(t *testing.T) {
	issuer := "https://traceway.example.com"
	if err := ValidateResource("", issuer); err != nil {
		t.Errorf("empty resource should pass: %v", err)
	}
	if err := ValidateResource("https://traceway.example.com/mcp", issuer); err != nil {
		t.Errorf("same-origin resource should pass: %v", err)
	}
	if err := ValidateResource("https://other.example.com/mcp", issuer); !errors.Is(err, ErrInvalidTarget) {
		t.Errorf("foreign resource should fail, got %v", err)
	}
	if err := ValidateResource("http://traceway.example.com/mcp", issuer); !errors.Is(err, ErrInvalidTarget) {
		t.Errorf("scheme downgrade should fail, got %v", err)
	}
}

func TestAuthorizationCodeRepository_PruneExpired(t *testing.T) {
	setupAuthDB(t)
	client := registerTestClient(t)

	_, err := db.ExecuteTransaction(func(tx *sql.Tx) (struct{}, error) {
		if err := repositories.AuthorizationCodeRepository.Create(tx, "twa_live", client.Id, 1, "http://127.0.0.1:1/cb", testChallenge(testVerifier), time.Now().Add(time.Minute)); err != nil {
			return struct{}{}, err
		}
		return struct{}{}, repositories.AuthorizationCodeRepository.Create(tx, "twa_dead", client.Id, 1, "http://127.0.0.1:1/cb", testChallenge(testVerifier), time.Now().Add(-time.Minute))
	})
	if err != nil {
		t.Fatalf("seed codes: %v", err)
	}

	pruned, err := db.ExecuteTransaction(func(tx *sql.Tx) (int64, error) {
		return repositories.AuthorizationCodeRepository.PruneExpired(tx, time.Now())
	})
	if err != nil || pruned != 1 {
		t.Fatalf("prune = %d, %v; want 1, nil", pruned, err)
	}
	if live, _ := repositories.AuthorizationCodeRepository.FindByCode(db.DB, "twa_live"); live == nil {
		t.Fatal("live code should survive the prune")
	}
}
