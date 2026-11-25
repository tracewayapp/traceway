package mcpmount

import (
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/tracewayapp/traceway/backend/v2/app/config"
	"github.com/tracewayapp/traceway/backend/v2/app/models"
	"github.com/tracewayapp/traceway/backend/v2/app/services"
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

func mcpPost(engine *gin.Engine, token, sessionID, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, Path, strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")
	if sessionID != "" {
		req.Header.Set("Mcp-Session-Id", sessionID)
	}
	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, req)
	return rec
}

func TestGinHandler_closesIdleSessions(t *testing.T) {
	previous := config.Config
	config.Config = &config.Cfg{JWTSecret: "mcpmount-test-secret-0123456789abcdefgh"}
	t.Cleanup(func() { config.Config = previous })
	if err := services.InitJWT(); err != nil {
		t.Fatal(err)
	}
	token, err := services.GenerateAccessToken(1, "a@example.com", models.ScopeRead, "traceway-cli", 15*time.Minute)
	if err != nil {
		t.Fatal(err)
	}

	previousTimeout := sessionIdleTimeout
	sessionIdleTimeout = 100 * time.Millisecond
	t.Cleanup(func() { sessionIdleTimeout = previousTimeout })

	gin.SetMode(gin.TestMode)
	engine := gin.New()
	engine.Any(Path, GinHandler(engine, "test", false))

	initialized := mcpPost(engine, token, "", `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-06-18","capabilities":{},"clientInfo":{"name":"test","version":"1"}}}`)
	if initialized.Code != http.StatusOK {
		t.Fatalf("initialize answered %d: %s", initialized.Code, initialized.Body.String())
	}
	sessionID := initialized.Header().Get("Mcp-Session-Id")
	if sessionID == "" {
		t.Fatal("initialize returned no Mcp-Session-Id")
	}

	if rec := mcpPost(engine, token, sessionID, `{"jsonrpc":"2.0","id":2,"method":"ping"}`); rec.Code != http.StatusOK {
		t.Fatalf("a live session answered %d: %s", rec.Code, rec.Body.String())
	}

	time.Sleep(5 * sessionIdleTimeout)

	if rec := mcpPost(engine, token, sessionID, `{"jsonrpc":"2.0","id":3,"method":"ping"}`); rec.Code != http.StatusNotFound {
		t.Fatalf("an idle session answered %d, want 404 so the client starts a new one: %s", rec.Code, rec.Body.String())
	}
}
