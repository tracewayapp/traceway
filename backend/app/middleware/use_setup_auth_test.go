//go:build !transactional_pg

package middleware

import (
	"database/sql"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/tracewayapp/lit/v2"
	"github.com/tracewayapp/traceway/backend/app/db"
	"github.com/tracewayapp/traceway/backend/app/repositories/transactional"
	_ "modernc.org/sqlite"
)

func setupSetupAuthDB(t *testing.T) {
	t.Helper()

	mainDB, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("failed to open in-memory sqlite: %v", err)
	}
	mainDB.SetMaxOpenConns(1)

	prevDB, prevDriver := db.DB, db.Driver
	db.DB = mainDB
	db.Driver = lit.SQLite
	t.Cleanup(func() {
		mainDB.Close()
		db.DB = prevDB
		db.Driver = prevDriver
	})

	for _, ddl := range []string{
		`CREATE TABLE users (id INTEGER PRIMARY KEY AUTOINCREMENT, email TEXT NOT NULL)`,
		`CREATE TABLE organization_users (
			user_id INTEGER NOT NULL,
			organization_id INTEGER NOT NULL,
			role TEXT NOT NULL,
			UNIQUE(user_id, organization_id)
		)`,
		`CREATE TABLE setup_tokens (
			id TEXT PRIMARY KEY,
			token_hash TEXT NOT NULL UNIQUE,
			prefix TEXT NOT NULL,
			user_id INTEGER NOT NULL,
			organization_id INTEGER NOT NULL,
			created_at DATETIME NOT NULL,
			expires_at DATETIME NOT NULL
		)`,
	} {
		if _, err := mainDB.Exec(ddl); err != nil {
			t.Fatalf("ddl failed: %v", err)
		}
	}
}

func setupAuthContext(t *testing.T, authorization string) (*gin.Context, *httptest.ResponseRecorder) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/setup/session", nil)
	if authorization != "" {
		c.Request.Header.Set("Authorization", authorization)
	}
	return c, recorder
}

func TestUseSetupAuthValidToken(t *testing.T) {
	setupSetupAuthDB(t)

	res, err := db.DB.Exec("INSERT INTO users (email) VALUES ('setup@example.com')")
	if err != nil {
		t.Fatalf("insert user: %v", err)
	}
	userId64, _ := res.LastInsertId()
	userId := int(userId64)
	if _, err := db.DB.Exec("INSERT INTO organization_users (user_id, organization_id, role) VALUES (?, ?, 'user')", userId, 42); err != nil {
		t.Fatalf("grant organization role: %v", err)
	}

	token := "tws_valid-token"
	if err := transactional.SetupTokenRepository.Create(db.DB, "tok-1", token, token, userId, 42, time.Now().Add(time.Hour)); err != nil {
		t.Fatalf("create token: %v", err)
	}

	c, recorder := setupAuthContext(t, "Bearer "+token)
	UseSetupAuth(c)

	if c.IsAborted() {
		t.Fatalf("request aborted with status %d", recorder.Code)
	}
	if got := GetUserId(c); got != userId {
		t.Errorf("userId = %d, want %d", got, userId)
	}
	if got := GetUserEmail(c); got != "setup@example.com" {
		t.Errorf("userEmail = %q", got)
	}
	if got := GetSetupOrganizationId(c); got != 42 {
		t.Errorf("setupOrganizationId = %d, want 42", got)
	}
	if !IsSetupScope(c) {
		t.Error("setup scope marker not set")
	}
}

func TestUseSetupAuthRejects(t *testing.T) {
	setupSetupAuthDB(t)

	res, _ := db.DB.Exec("INSERT INTO users (email) VALUES ('setup@example.com')")
	userId64, _ := res.LastInsertId()
	if _, err := db.DB.Exec("INSERT INTO organization_users (user_id, organization_id, role) VALUES (?, ?, 'user')", userId64, 42); err != nil {
		t.Fatalf("grant organization role: %v", err)
	}

	expired := "tws_expired-token"
	if err := transactional.SetupTokenRepository.Create(db.DB, "tok-exp", expired, expired, int(userId64), 42, time.Now().Add(-time.Minute)); err != nil {
		t.Fatalf("create expired token: %v", err)
	}

	cases := map[string]string{
		"missing header":      "",
		"non-bearer":          "Basic abc",
		"unknown setup token": "Bearer tws_unknown-token",
		"expired setup token": "Bearer " + expired,
		"pat prefix":          "Bearer twp_not-a-setup-token",
		"jwt-looking string":  "Bearer eyJhbGciOiJIUzI1NiJ9.e30.x",
	}
	for name, header := range cases {
		t.Run(name, func(t *testing.T) {
			c, recorder := setupAuthContext(t, header)
			UseSetupAuth(c)
			if !c.IsAborted() || recorder.Code != http.StatusUnauthorized {
				t.Errorf("expected 401 abort, got aborted=%v status=%d", c.IsAborted(), recorder.Code)
			}
			if IsSetupScope(c) {
				t.Error("setup scope must not be set on rejection")
			}
		})
	}
}

// A setup token must never authenticate as an app credential: UseAppAuth's
// bearer path only special-cases twp_ and JWT parsing rejects opaque strings.
// This test locks that in against a future "generic opaque token" refactor.
func TestSetupTokenNeverPassesAppAuth(t *testing.T) {
	setupSetupAuthDB(t)

	identity, err := AuthenticateBearer("tws_" + "some-opaque-setup-token")
	if !errors.Is(err, ErrInvalidBearer) {
		t.Fatalf("AuthenticateBearer(tws_...) = (%v, %v), want ErrInvalidBearer", identity, err)
	}
}
