//go:build !pgch

package authserver

import (
	"database/sql"
	"errors"
	"testing"
	"time"

	"github.com/tracewayapp/lit/v2"
	"github.com/tracewayapp/traceway/backend/app/config"
	"github.com/tracewayapp/traceway/backend/app/db"
	"github.com/tracewayapp/traceway/backend/app/migrations"
	"github.com/tracewayapp/traceway/backend/app/models"
	"github.com/tracewayapp/traceway/backend/app/repositories"
	"github.com/tracewayapp/traceway/backend/app/services"

	_ "modernc.org/sqlite"
)

func setupAuthDB(t *testing.T) {
	t.Helper()

	mainDB, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open main sqlite: %v", err)
	}
	mainDB.SetMaxOpenConns(1)

	telemetryDB, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open telemetry sqlite: %v", err)
	}
	telemetryDB.SetMaxOpenConns(1)

	db.DB = mainDB
	db.TelemetryDB = telemetryDB
	db.Driver = lit.SQLite
	models.Init(db.Driver)

	if err := migrations.Run("sqlite"); err != nil {
		t.Fatalf("run migrations: %v", err)
	}

	config.Init(&config.Cfg{JWTSecret: "test-secret-that-is-at-least-32-chars-long"})
	if err := services.InitJWT(); err != nil {
		t.Fatalf("init jwt: %v", err)
	}

	if _, err := mainDB.Exec("INSERT INTO users (id, email, name, password) VALUES (1, 'dev@example.com', 'Dev', 'x')"); err != nil {
		t.Fatalf("seed user: %v", err)
	}

	t.Cleanup(func() {
		mainDB.Close()
		telemetryDB.Close()
	})
}

func insertRefresh(t *testing.T, token, family string) {
	t.Helper()
	if err := repositories.RefreshTokenRepository.Insert(db.DB, token, family, 1, cliClientID, time.Now().Add(time.Hour)); err != nil {
		t.Fatalf("insert refresh token: %v", err)
	}
}

func TestRotateRefresh_sequentialRotation(t *testing.T) {
	setupAuthDB(t)
	insertRefresh(t, "twr_seq", "fam-seq")

	ts, err := RotateRefresh("twr_seq")
	if err != nil {
		t.Fatalf("RotateRefresh: %v", err)
	}
	if ts.RefreshToken == "" || ts.RefreshToken == "twr_seq" {
		t.Fatalf("expected a rotated refresh token, got %q", ts.RefreshToken)
	}
	if ts.AccessToken == "" {
		t.Fatal("expected an access token")
	}

	old, _ := repositories.RefreshTokenRepository.FindByToken(db.DB, "twr_seq")
	if old == nil || !old.Used {
		t.Fatalf("presented token should be marked used, got %+v", old)
	}
	fresh, _ := repositories.RefreshTokenRepository.FindByToken(db.DB, ts.RefreshToken)
	if fresh == nil || fresh.Used || fresh.Revoked || fresh.FamilyId != "fam-seq" {
		t.Fatalf("rotated token should be live in the same family, got %+v", fresh)
	}
}

func TestRotateRefresh_replayOutsideGraceRevokesFamily(t *testing.T) {
	setupAuthDB(t)
	insertRefresh(t, "twr_old", "fam-old")
	insertRefresh(t, "twr_sibling", "fam-old")

	// Simulate a token consumed well outside the reuse-grace window.
	if _, err := repositories.RefreshTokenRepository.MarkUsedIfUnused(db.DB, "twr_old", time.Now().Add(-time.Hour)); err != nil {
		t.Fatalf("mark used: %v", err)
	}

	if _, err := RotateRefresh("twr_old"); !errors.Is(err, ErrInvalidGrant) {
		t.Fatalf("expected ErrInvalidGrant, got %v", err)
	}

	sib, _ := repositories.RefreshTokenRepository.FindByToken(db.DB, "twr_sibling")
	if sib == nil || !sib.Revoked {
		t.Fatalf("genuine replay must revoke the whole family, sibling = %+v", sib)
	}
}

func TestRotateRefresh_concurrentWithinGraceKeepsFamily(t *testing.T) {
	setupAuthDB(t)
	insertRefresh(t, "twr_race", "fam-race")
	insertRefresh(t, "twr_winner", "fam-race")

	// The racing request already consumed the token just now (within grace).
	if _, err := repositories.RefreshTokenRepository.MarkUsedIfUnused(db.DB, "twr_race", time.Now()); err != nil {
		t.Fatalf("mark used: %v", err)
	}

	if _, err := RotateRefresh("twr_race"); !errors.Is(err, ErrInvalidGrant) {
		t.Fatalf("expected ErrInvalidGrant, got %v", err)
	}

	winner, _ := repositories.RefreshTokenRepository.FindByToken(db.DB, "twr_winner")
	if winner == nil || winner.Revoked {
		t.Fatalf("benign concurrent retry must NOT revoke the family, winner = %+v", winner)
	}
}

func TestRotateRefresh_lostResponseReplaysTokenSetWithinGrace(t *testing.T) {
	setupAuthDB(t)
	insertRefresh(t, "twr_lost", "fam-lost")

	first, err := RotateRefresh("twr_lost")
	if err != nil {
		t.Fatalf("first rotation: %v", err)
	}

	// The client lost the response (or failed to persist it) and re-presents
	// the consumed token within the grace window: it must receive the same
	// rotated token set, not invalid_grant, and the family must stay intact.
	second, err := RotateRefresh("twr_lost")
	if err != nil {
		t.Fatalf("grace-window retry: %v", err)
	}
	if second.RefreshToken != first.RefreshToken || second.AccessToken != first.AccessToken {
		t.Fatalf("grace-window retry must replay the rotated token set, got %+v want %+v", second, first)
	}

	winner, _ := repositories.RefreshTokenRepository.FindByToken(db.DB, first.RefreshToken)
	if winner == nil || winner.Revoked {
		t.Fatalf("grace-window retry must not revoke the family, winner = %+v", winner)
	}
}

func TestCreateDeviceAuth_unknownClientRejected(t *testing.T) {
	setupAuthDB(t)

	if _, err := CreateDeviceAuth("Traceway Security Check", "http://localhost"); !errors.Is(err, ErrUnknownClient) {
		t.Fatalf("expected ErrUnknownClient, got %v", err)
	}
	if _, err := CreateDeviceAuth(mcpClientID, "http://localhost"); err != nil {
		t.Fatalf("known client must be accepted, got %v", err)
	}
}

func TestApprove_expiredCodeRejected(t *testing.T) {
	setupAuthDB(t)

	if err := repositories.DeviceAuthorizationRepository.Create(db.DB, "dev-exp", "CCCC-DDDD", cliClientID, 5, time.Now().Add(-time.Minute)); err != nil {
		t.Fatalf("create device auth: %v", err)
	}

	tx, err := db.DB.Begin()
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	defer tx.Rollback()

	if err := Approve(tx, "CCCC-DDDD", 1); !errors.Is(err, ErrDeviceExpired) {
		t.Fatalf("expected ErrDeviceExpired, got %v", err)
	}
}

func TestApprove_lowercaseUserCode(t *testing.T) {
	setupAuthDB(t)

	if err := repositories.DeviceAuthorizationRepository.Create(db.DB, "dev-lc", "EEEE-FFFF", cliClientID, 5, time.Now().Add(deviceCodeTTL)); err != nil {
		t.Fatalf("create device auth: %v", err)
	}

	tx, err := db.DB.Begin()
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	if err := Approve(tx, "eeee-ffff", 1); err != nil {
		tx.Rollback()
		t.Fatalf("approve with lowercase code: %v", err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatalf("commit: %v", err)
	}

	da, _ := repositories.DeviceAuthorizationRepository.FindByUserCode(db.DB, "EEEE-FFFF")
	if da == nil || da.Status != "approved" {
		t.Fatalf("code should be approved, got %+v", da)
	}
}

func TestRefreshTokenRepository_PruneExpired(t *testing.T) {
	setupAuthDB(t)

	insertRefresh(t, "twr_live", "fam-prune")
	insertRefresh(t, "twr_recent_used", "fam-prune")
	insertRefresh(t, "twr_old_used", "fam-prune")
	insertRefresh(t, "twr_revoked", "fam-dead")
	if err := repositories.RefreshTokenRepository.Insert(db.DB, "twr_expired", "fam-prune", 1, cliClientID, time.Now().Add(-time.Hour)); err != nil {
		t.Fatalf("insert expired: %v", err)
	}

	if _, err := repositories.RefreshTokenRepository.MarkUsedIfUnused(db.DB, "twr_recent_used", time.Now()); err != nil {
		t.Fatalf("mark used: %v", err)
	}
	if _, err := repositories.RefreshTokenRepository.MarkUsedIfUnused(db.DB, "twr_old_used", time.Now().Add(-31*24*time.Hour)); err != nil {
		t.Fatalf("mark used: %v", err)
	}
	if err := repositories.RefreshTokenRepository.RevokeFamily(db.DB, "fam-dead"); err != nil {
		t.Fatalf("revoke family: %v", err)
	}

	n, err := repositories.RefreshTokenRepository.PruneExpired(db.DB, time.Now())
	if err != nil {
		t.Fatalf("prune: %v", err)
	}
	if n != 3 {
		t.Fatalf("prune removed %d rows, want 3 (expired + revoked + anciently used)", n)
	}
	for _, tok := range []string{"twr_live", "twr_recent_used"} {
		if rt, _ := repositories.RefreshTokenRepository.FindByToken(db.DB, tok); rt == nil {
			t.Fatalf("%s should survive pruning", tok)
		}
	}
}

func TestRevokeByToken_revokesFamily(t *testing.T) {
	setupAuthDB(t)
	insertRefresh(t, "twr_a", "fam-logout")
	insertRefresh(t, "twr_b", "fam-logout")

	if err := RevokeByToken("twr_a"); err != nil {
		t.Fatalf("RevokeByToken: %v", err)
	}

	for _, tok := range []string{"twr_a", "twr_b"} {
		rt, _ := repositories.RefreshTokenRepository.FindByToken(db.DB, tok)
		if rt == nil || !rt.Revoked {
			t.Fatalf("logout must revoke the whole family, %s = %+v", tok, rt)
		}
	}
	// Unknown token is a no-op, not an error.
	if err := RevokeByToken("twr_missing"); err != nil {
		t.Fatalf("RevokeByToken(unknown) should be a no-op, got %v", err)
	}
}

func TestPollDeviceToken_singleUse(t *testing.T) {
	setupAuthDB(t)

	const deviceCode = "dev-code"
	const userCode = "AAAA-BBBB"
	if err := repositories.DeviceAuthorizationRepository.Create(db.DB, deviceCode, userCode, cliClientID, 5, time.Now().Add(deviceCodeTTL)); err != nil {
		t.Fatalf("create device auth: %v", err)
	}
	if _, err := repositories.DeviceAuthorizationRepository.Approve(db.DB, userCode, 1); err != nil {
		t.Fatalf("approve: %v", err)
	}

	ts, err := PollDeviceToken(deviceCode)
	if err != nil {
		t.Fatalf("first poll: %v", err)
	}
	if ts.AccessToken == "" || ts.RefreshToken == "" {
		t.Fatalf("first poll should issue a token set, got %+v", ts)
	}

	if _, err := PollDeviceToken(deviceCode); !errors.Is(err, ErrExpiredToken) {
		t.Fatalf("a device code must be single-use; second poll = %v", err)
	}
}

func TestMarkUsedIfUnused_isConditional(t *testing.T) {
	setupAuthDB(t)
	insertRefresh(t, "twr_once", "fam-once")

	n, err := repositories.RefreshTokenRepository.MarkUsedIfUnused(db.DB, "twr_once", time.Now())
	if err != nil || n != 1 {
		t.Fatalf("first mark = (%d, %v), want (1, nil)", n, err)
	}
	n, err = repositories.RefreshTokenRepository.MarkUsedIfUnused(db.DB, "twr_once", time.Now())
	if err != nil || n != 0 {
		t.Fatalf("second mark = (%d, %v), want (0, nil)", n, err)
	}
}

func TestPersonalAccessTokenRepository_PruneExpired(t *testing.T) {
	setupAuthDB(t)

	expired := time.Now().Add(-time.Hour)
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatalf("pat create: %v", err)
		}
	}
	must(repositories.PersonalAccessTokenRepository.Create(db.DB, "id-active", "twp_active", "twp_active12", 1, "active", nil))
	must(repositories.PersonalAccessTokenRepository.Create(db.DB, "id-expired", "twp_expired", "twp_expired1", 1, "expired", &expired))
	must(repositories.PersonalAccessTokenRepository.Create(db.DB, "id-revoked", "twp_revoked", "twp_revoked1", 1, "revoked", nil))
	if _, err := repositories.PersonalAccessTokenRepository.Revoke(db.DB, "id-revoked", 1); err != nil {
		t.Fatalf("revoke: %v", err)
	}

	n, err := repositories.PersonalAccessTokenRepository.PruneExpired(db.DB, time.Now())
	if err != nil {
		t.Fatalf("prune: %v", err)
	}
	if n != 2 {
		t.Fatalf("prune removed %d rows, want 2 (expired + revoked)", n)
	}

	active, _ := repositories.PersonalAccessTokenRepository.FindActiveByToken(db.DB, "twp_active")
	if active == nil {
		t.Fatal("active PAT should survive pruning")
	}
}
