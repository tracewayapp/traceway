//go:build !transactional_pg

package transactional

import (
	"testing"
	"time"

	"github.com/tracewayapp/traceway/backend/app/db"
	"github.com/tracewayapp/traceway/backend/app/repositories/transactional/shared"
)

func setupSetupTokenTables(t *testing.T) {
	t.Helper()
	if _, err := db.DB.Exec(`CREATE TABLE IF NOT EXISTS users (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		email TEXT NOT NULL,
		name TEXT NOT NULL DEFAULT '',
		password TEXT NOT NULL DEFAULT ''
	)`); err != nil {
		t.Fatalf("failed to create users table: %v", err)
	}
	if _, err := db.DB.Exec(`CREATE TABLE IF NOT EXISTS setup_tokens (
		id TEXT PRIMARY KEY,
		token_hash TEXT NOT NULL UNIQUE,
		prefix TEXT NOT NULL,
		user_id INTEGER NOT NULL,
		organization_id INTEGER NOT NULL,
		created_at DATETIME NOT NULL,
		expires_at DATETIME NOT NULL
	)`); err != nil {
		t.Fatalf("failed to create setup_tokens table: %v", err)
	}
	if _, err := db.DB.Exec(`CREATE TABLE IF NOT EXISTS organization_users (
		user_id INTEGER NOT NULL,
		organization_id INTEGER NOT NULL,
		role TEXT NOT NULL,
		UNIQUE(user_id, organization_id)
	)`); err != nil {
		t.Fatalf("failed to create organization_users table: %v", err)
	}
}

func insertSetupTestUser(t *testing.T, email string) int {
	t.Helper()
	res, err := db.DB.Exec("INSERT INTO users (email) VALUES (?)", email)
	if err != nil {
		t.Fatalf("failed to insert user: %v", err)
	}
	id, _ := res.LastInsertId()
	return int(id)
}

func grantSetupTestOrg(t *testing.T, userId, organizationId int, role string) {
	t.Helper()
	if _, err := db.DB.Exec(
		"INSERT INTO organization_users (user_id, organization_id, role) VALUES (?, ?, ?)",
		userId, organizationId, role,
	); err != nil {
		t.Fatalf("failed to grant organization role: %v", err)
	}
}

func TestSetupTokenRoundTrip(t *testing.T) {
	setupTestDB(t)
	setupSetupTokenTables(t)
	userId := insertSetupTestUser(t, "setup@example.com")
	grantSetupTestOrg(t, userId, 7, "user")

	token := "tws_test-token-value"
	expiresAt := time.Now().Add(time.Hour).UTC()
	if err := SetupTokenRepository.Create(db.DB, "tok-1", token, token[:12], userId, 7, expiresAt); err != nil {
		t.Fatalf("failed to create setup token: %v", err)
	}

	var storedHash string
	if err := db.DB.QueryRow("SELECT token_hash FROM setup_tokens WHERE id = 'tok-1'").Scan(&storedHash); err != nil {
		t.Fatalf("failed to read token_hash: %v", err)
	}
	if storedHash == token {
		t.Fatal("token must not be stored in plaintext")
	}
	if storedHash != shared.HashAuthToken(token) {
		t.Fatalf("token_hash = %q, want sha256 of the token", storedHash)
	}

	st, err := SetupTokenRepository.FindActiveByToken(db.DB, token)
	if err != nil {
		t.Fatalf("failed to find setup token: %v", err)
	}
	if st == nil {
		t.Fatal("expected an active setup token")
	}
	if st.UserId != userId || st.OrganizationId != 7 || st.Email != "setup@example.com" {
		t.Errorf("unexpected token identity: %+v", st)
	}
	if st.ExpiresAt.Sub(expiresAt).Abs() > time.Second {
		t.Errorf("expiresAt = %v, want %v", st.ExpiresAt, expiresAt)
	}
}

func TestSetupTokenUnknownAndExpired(t *testing.T) {
	setupTestDB(t)
	setupSetupTokenTables(t)
	userId := insertSetupTestUser(t, "setup@example.com")
	grantSetupTestOrg(t, userId, 7, "user")

	if st, err := SetupTokenRepository.FindActiveByToken(db.DB, "tws_unknown"); err != nil || st != nil {
		t.Fatalf("unknown token: got (%v, %v), want (nil, nil)", st, err)
	}

	expired := "tws_expired-token"
	if err := SetupTokenRepository.Create(db.DB, "tok-exp", expired, expired[:12], userId, 7, time.Now().Add(-time.Minute)); err != nil {
		t.Fatalf("failed to create expired token: %v", err)
	}
	if st, err := SetupTokenRepository.FindActiveByToken(db.DB, expired); err != nil || st != nil {
		t.Fatalf("expired token: got (%v, %v), want (nil, nil)", st, err)
	}
}

func TestSetupTokenDeleteByUserAndOrganizationScopes(t *testing.T) {
	setupTestDB(t)
	setupSetupTokenTables(t)
	userId := insertSetupTestUser(t, "setup@example.com")
	otherUser := insertSetupTestUser(t, "other@example.com")
	grantSetupTestOrg(t, userId, 1, "user")
	grantSetupTestOrg(t, userId, 2, "user")
	grantSetupTestOrg(t, otherUser, 1, "user")

	expiresAt := time.Now().Add(time.Hour)
	mustCreate := func(id, token string, uid, orgId int) {
		t.Helper()
		if err := SetupTokenRepository.Create(db.DB, id, token, token, uid, orgId, expiresAt); err != nil {
			t.Fatalf("failed to create %s: %v", id, err)
		}
	}
	mustCreate("tok-a", "tws_token-a", userId, 1)
	mustCreate("tok-b", "tws_token-b", userId, 2)
	mustCreate("tok-c", "tws_token-c", otherUser, 1)

	if err := SetupTokenRepository.DeleteByUserAndOrganization(db.DB, userId, 1); err != nil {
		t.Fatalf("failed to delete: %v", err)
	}

	if st, _ := SetupTokenRepository.FindActiveByToken(db.DB, "tws_token-a"); st != nil {
		t.Error("token for (user, org 1) should be deleted")
	}
	if st, _ := SetupTokenRepository.FindActiveByToken(db.DB, "tws_token-b"); st == nil {
		t.Error("same user's token for another org must survive")
	}
	if st, _ := SetupTokenRepository.FindActiveByToken(db.DB, "tws_token-c"); st == nil {
		t.Error("another user's token for the same org must survive")
	}
}

func TestSetupTokenRequiresCurrentOrganizationWriteAccess(t *testing.T) {
	setupTestDB(t)
	setupSetupTokenTables(t)
	userId := insertSetupTestUser(t, "setup@example.com")
	grantSetupTestOrg(t, userId, 7, "user")

	token := "tws_membership-token"
	if err := SetupTokenRepository.Create(db.DB, "tok-membership", token, token, userId, 7, time.Now().Add(time.Hour)); err != nil {
		t.Fatalf("failed to create setup token: %v", err)
	}
	if st, err := SetupTokenRepository.FindActiveByToken(db.DB, token); err != nil || st == nil {
		t.Fatalf("writable membership token = (%v, %v), want active", st, err)
	}

	if _, err := db.DB.Exec("UPDATE organization_users SET role = 'readonly' WHERE user_id = ? AND organization_id = ?", userId, 7); err != nil {
		t.Fatalf("downgrade membership: %v", err)
	}
	if st, err := SetupTokenRepository.FindActiveByToken(db.DB, token); err != nil || st != nil {
		t.Fatalf("read-only membership token = (%v, %v), want inactive", st, err)
	}

	if _, err := db.DB.Exec("DELETE FROM organization_users WHERE user_id = ? AND organization_id = ?", userId, 7); err != nil {
		t.Fatalf("remove membership: %v", err)
	}
	if st, err := SetupTokenRepository.FindActiveByToken(db.DB, token); err != nil || st != nil {
		t.Fatalf("removed membership token = (%v, %v), want inactive", st, err)
	}
}

func TestSetupTokenPruneExpired(t *testing.T) {
	setupTestDB(t)
	setupSetupTokenTables(t)
	userId := insertSetupTestUser(t, "setup@example.com")
	grantSetupTestOrg(t, userId, 1, "user")

	if err := SetupTokenRepository.Create(db.DB, "tok-live", "tws_live", "tws_live", userId, 1, time.Now().Add(time.Hour)); err != nil {
		t.Fatalf("failed to create live token: %v", err)
	}
	if err := SetupTokenRepository.Create(db.DB, "tok-dead", "tws_dead", "tws_dead", userId, 1, time.Now().Add(-time.Hour)); err != nil {
		t.Fatalf("failed to create expired token: %v", err)
	}

	pruned, err := SetupTokenRepository.PruneExpired(db.DB, time.Now())
	if err != nil {
		t.Fatalf("prune failed: %v", err)
	}
	if pruned != 1 {
		t.Fatalf("pruned = %d, want 1", pruned)
	}
	if st, _ := SetupTokenRepository.FindActiveByToken(db.DB, "tws_live"); st == nil {
		t.Error("live token must survive the prune")
	}
}
