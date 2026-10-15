//go:build !transactional_pg && !telemetry_ch && !telemetry_duckdb

package controllers

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"

	"github.com/tracewayapp/traceway/backend/app/config"
	"github.com/tracewayapp/traceway/backend/app/db"
	"github.com/tracewayapp/traceway/backend/app/dbtest"
	"github.com/tracewayapp/traceway/backend/app/middleware"
	"github.com/tracewayapp/traceway/backend/app/models"
	"github.com/tracewayapp/traceway/backend/app/repositories/transactional"
	"github.com/tracewayapp/traceway/backend/app/services"
	"github.com/tracewayapp/traceway/backend/app/services/authserver"
)

const agentTestSecret = "agent-access-test-secret-0123456789abcdef"

type agentFixture struct {
	engine       *gin.Engine
	orgId        int
	project      *models.Project
	otherProject *models.Project
	userId       int
}

func setupAgentFixture(t *testing.T) *agentFixture {
	t.Helper()
	gin.SetMode(gin.TestMode)
	dbtest.SetupSQLite(t)
	previous := config.Config.JWTSecret
	config.Config.JWTSecret = agentTestSecret
	t.Cleanup(func() { config.Config.JWTSecret = previous })
	if err := services.InitJWT(); err != nil {
		t.Fatalf("init jwt: %v", err)
	}
	middleware.InitUseAppAuth()
	middleware.InitRequireWriteAccess()
	middleware.InitRequireProjectAccess()
	middleware.InitRequireAdminAccess()
	middleware.InitRequireOrganizationAccess()

	f := &agentFixture{}
	f.orgId = createTestOrg(t, "agent-org")
	otherOrg := createTestOrg(t, "foreign-org")
	var err error
	f.project, err = db.ExecuteTransaction(func(tx *sql.Tx) (*models.Project, error) {
		return transactional.ProjectRepository.CreateWithOrganization(tx, "agent-project", "opentelemetry", f.orgId)
	})
	if err != nil {
		t.Fatalf("create project: %v", err)
	}
	f.otherProject, err = db.ExecuteTransaction(func(tx *sql.Tx) (*models.Project, error) {
		return transactional.ProjectRepository.CreateWithOrganization(tx, "foreign-project", "opentelemetry", otherOrg)
	})
	if err != nil {
		t.Fatalf("create foreign project: %v", err)
	}
	user, err := db.ExecuteTransaction(func(tx *sql.Tx) (*models.User, error) {
		u, err := transactional.UserRepository.Create(tx, "agent@example.com", "Agent Owner", "x")
		if err != nil {
			return nil, err
		}
		_, err = transactional.OrganizationRepository.AddUser(tx, f.orgId, u.Id, "owner")
		return u, err
	})
	if err != nil {
		t.Fatalf("create user: %v", err)
	}
	f.userId = user.Id

	f.engine = gin.New()
	RegisterControllers(f.engine.Group("/api"))
	return f
}

func (f *agentFixture) pat(t *testing.T, scope string) string {
	t.Helper()
	prefix := "twp_"
	if scope == models.ScopeWrite {
		prefix = "tww_"
	}
	token := prefix + uuid.NewString()
	expires := time.Now().Add(time.Hour)
	_, err := db.ExecuteTransaction(func(tx *sql.Tx) (struct{}, error) {
		return struct{}{}, transactional.PersonalAccessTokenRepository.Create(tx, uuid.NewString(), token, token[:12], f.userId, "test "+scope, scope, &expires)
	})
	if err != nil {
		t.Fatalf("create pat: %v", err)
	}
	return token
}

func (f *agentFixture) do(t *testing.T, token, method, path string, body any) *httptest.ResponseRecorder {
	t.Helper()
	var buf bytes.Buffer
	if body != nil {
		_ = json.NewEncoder(&buf).Encode(body)
	}
	req := httptest.NewRequest(method, path, &buf)
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	f.engine.ServeHTTP(rec, req)
	return rec
}

func (f *agentFixture) archivePath(project *models.Project) string {
	return "/api/exception-stack-traces/archive?projectId=" + project.Id.String()
}

func (f *agentFixture) archive(t *testing.T, token string) *httptest.ResponseRecorder {
	t.Helper()
	return f.do(t, token, http.MethodPost, f.archivePath(f.project), map[string]any{"hashes": []string{"abcdef0123456789"}})
}

func (f *agentFixture) setAgentWrites(t *testing.T, enabled bool) {
	t.Helper()
	_, err := db.ExecuteTransaction(func(tx *sql.Tx) (struct{}, error) {
		return struct{}{}, transactional.OrganizationRepository.SetAgentWriteEnabled(tx, f.orgId, enabled)
	})
	if err != nil {
		t.Fatalf("set agent writes: %v", err)
	}
}

func (f *agentFixture) auditEvents(t *testing.T) []*models.AgentWriteEvent {
	t.Helper()
	events, err := db.ExecuteTransaction(func(tx *sql.Tx) ([]*models.AgentWriteEvent, error) {
		events, _, err := transactional.AgentWriteEventRepository.ListByOrganization(tx, f.orgId, 1, 100)
		return events, err
	})
	if err != nil {
		t.Fatalf("list audit events: %v", err)
	}
	return events
}

func signTestJWT(t *testing.T, userId int, scope, clientId string, ttl time.Duration) string {
	t.Helper()
	now := time.Now()
	claims := services.JWTClaims{
		UserId:   userId,
		Email:    "agent@example.com",
		Scope:    scope,
		ClientId: clientId,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(now.Add(ttl)),
			IssuedAt:  jwt.NewNumericDate(now),
			NotBefore: jwt.NewNumericDate(now),
		},
	}
	token, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(agentTestSecret))
	if err != nil {
		t.Fatalf("sign jwt: %v", err)
	}
	return token
}

func errorMessage(rec *httptest.ResponseRecorder) string {
	var body struct {
		Error string `json:"error"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &body)
	return body.Error
}

func TestAgentGate_readTokenReadsButCannotWrite(t *testing.T) {
	f := setupAgentFixture(t)
	read := f.pat(t, models.ScopeRead)

	if rec := f.do(t, read, http.MethodGet, "/api/projects", nil); rec.Code != http.StatusOK {
		t.Fatalf("read token GET /api/projects = %d %s", rec.Code, rec.Body.String())
	}

	f.setAgentWrites(t, true)
	rec := f.archive(t, read)
	if rec.Code != http.StatusForbidden || !strings.Contains(errorMessage(rec), "read-only") {
		t.Fatalf("read token archive = %d %q, want 403 read-only", rec.Code, errorMessage(rec))
	}
	if got := rec.Header().Get("WWW-Authenticate"); !strings.Contains(got, `scope="traceway:write"`) {
		t.Errorf("insufficient scope challenge missing: %q", got)
	}
	if events := f.auditEvents(t); len(events) != 0 {
		t.Errorf("a scope rejection must not reach the audit log, got %d events", len(events))
	}
}

func TestAgentGate_routesOutsideAllowlistAreDenied(t *testing.T) {
	f := setupAgentFixture(t)
	f.setAgentWrites(t, true)
	write := f.pat(t, models.ScopeWrite)

	for _, tc := range []struct{ method, path string }{
		{http.MethodPost, "/api/personal-access-tokens"},
		{http.MethodPost, "/api/device/approve"},
		{http.MethodPost, "/api/oauth/approve"},
		{http.MethodDelete, "/api/projects?projectId=" + f.project.Id.String()},
		{http.MethodPut, "/api/organizations/" + itoa(f.orgId) + "/agent-access"},
		{http.MethodGet, "/api/me/login-bundle"},
	} {
		rec := f.do(t, write, tc.method, tc.path, map[string]any{})
		if rec.Code != http.StatusForbidden || !strings.Contains(errorMessage(rec), "not available") {
			t.Errorf("%s %s with a write token = %d %q, want 403 not available", tc.method, tc.path, rec.Code, errorMessage(rec))
		}
	}
}

func TestAgentGate_writeNeedsOrganizationOptIn(t *testing.T) {
	f := setupAgentFixture(t)
	write := f.pat(t, models.ScopeWrite)

	rec := f.archive(t, write)
	if rec.Code != http.StatusForbidden || !strings.Contains(errorMessage(rec), "turned off") {
		t.Fatalf("write before opt-in = %d %q, want 403 turned off", rec.Code, errorMessage(rec))
	}

	f.setAgentWrites(t, true)
	if rec := f.archive(t, write); rec.Code != http.StatusOK {
		t.Fatalf("write after opt-in = %d %s", rec.Code, rec.Body.String())
	}

	events := f.auditEvents(t)
	if len(events) != 2 {
		t.Fatalf("audit events = %d, want 2 (the refused and the accepted write)", len(events))
	}
	accepted, refused := events[0], events[1]
	if accepted.StatusCode != http.StatusOK || refused.StatusCode != http.StatusForbidden {
		t.Errorf("audit statuses = %d, %d; want 200 then 403 (newest first)", accepted.StatusCode, refused.StatusCode)
	}
	if accepted.CredentialKind != middleware.CredentialPAT || accepted.Route != "/api/exception-stack-traces/archive" || accepted.ProjectName != "agent-project" || accepted.UserEmail != "agent@example.com" || accepted.CredentialName != "test traceway:write" {
		t.Errorf("audit row = %+v", accepted)
	}
}

func TestAgentGate_writeToForeignProjectIsRefused(t *testing.T) {
	f := setupAgentFixture(t)
	f.setAgentWrites(t, true)
	write := f.pat(t, models.ScopeWrite)

	rec := f.do(t, write, http.MethodPost, f.archivePath(f.otherProject), map[string]any{"hashes": []string{"abcdef0123456789"}})
	if rec.Code != http.StatusForbidden {
		t.Fatalf("write to a foreign org's project = %d, want 403", rec.Code)
	}
	if events := f.auditEvents(t); len(events) != 0 {
		t.Errorf("a foreign project write must not be recorded in this org, got %d", len(events))
	}
}

func TestAgentGate_jwtKinds(t *testing.T) {
	f := setupAgentFixture(t)
	f.setAgentWrites(t, true)

	session, err := services.GenerateToken(f.userId, "agent@example.com")
	if err != nil {
		t.Fatal(err)
	}
	if rec := f.archive(t, session); rec.Code != http.StatusOK {
		t.Fatalf("dashboard session archive = %d %s", rec.Code, rec.Body.String())
	}
	if rec := f.do(t, session, http.MethodGet, "/api/me/login-bundle", nil); rec.Code == http.StatusForbidden {
		t.Errorf("dashboard sessions must not be held to the agent allowlist")
	}
	if n := len(f.auditEvents(t)); n != 0 {
		t.Errorf("dashboard writes are not agent writes, got %d audit rows", n)
	}

	legacy := signTestJWT(t, f.userId, "", "", 15*time.Minute)
	if rec := f.archive(t, legacy); rec.Code != http.StatusForbidden {
		t.Errorf("unscoped 15-minute token archive = %d, want 403 (held to read)", rec.Code)
	}

	readOAuth := signTestJWT(t, f.userId, models.ScopeRead, "traceway-cli", 15*time.Minute)
	if rec := f.archive(t, readOAuth); rec.Code != http.StatusForbidden {
		t.Errorf("read OAuth token archive = %d, want 403", rec.Code)
	}

	writeOAuth := signTestJWT(t, f.userId, models.ScopeWrite, "traceway-cli-write", 15*time.Minute)
	if rec := f.archive(t, writeOAuth); rec.Code != http.StatusOK {
		t.Fatalf("write OAuth token archive = %d %s", rec.Code, rec.Body.String())
	}
	events := f.auditEvents(t)
	if len(events) != 1 || events[0].CredentialKind != middleware.CredentialOAuth || events[0].CredentialRef != "traceway-cli-write" {
		t.Errorf("OAuth write audit = %+v", events)
	}
}

func (f *agentFixture) member(t *testing.T, email, role string) (userId int, writeToken, session string) {
	t.Helper()
	user, err := db.ExecuteTransaction(func(tx *sql.Tx) (*models.User, error) {
		u, err := transactional.UserRepository.Create(tx, email, email, "x")
		if err != nil {
			return nil, err
		}
		_, err = transactional.OrganizationRepository.AddUser(tx, f.orgId, u.Id, role)
		return u, err
	})
	if err != nil {
		t.Fatalf("create member: %v", err)
	}
	writeToken = "tww_" + uuid.NewString()
	expires := time.Now().Add(time.Hour)
	_, err = db.ExecuteTransaction(func(tx *sql.Tx) (struct{}, error) {
		return struct{}{}, transactional.PersonalAccessTokenRepository.Create(tx, uuid.NewString(), writeToken, writeToken[:12], user.Id, "member write", models.ScopeWrite, &expires)
	})
	if err != nil {
		t.Fatalf("create member pat: %v", err)
	}
	session, err = services.GenerateToken(user.Id, email)
	if err != nil {
		t.Fatal(err)
	}
	return user.Id, writeToken, session
}

func TestAgentGate_readonlyMemberWriteTokenStillNeedsWriteRole(t *testing.T) {
	f := setupAgentFixture(t)
	f.setAgentWrites(t, true)
	_, write, _ := f.member(t, "readonly@example.com", "readonly")

	rec := f.archive(t, write)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("readonly member's write token archive = %d, want 403 from the role check after the gate", rec.Code)
	}
	events := f.auditEvents(t)
	if len(events) != 1 || events[0].StatusCode != http.StatusForbidden || events[0].UserEmail != "readonly@example.com" {
		t.Errorf("the refused write must be audited with its 403, got %+v", events)
	}
}

func TestAgentAccessSettings_adminOnly(t *testing.T) {
	f := setupAgentFixture(t)
	_, _, session := f.member(t, "user@example.com", "user")
	base := "/api/organizations/" + itoa(f.orgId)

	if rec := f.do(t, session, http.MethodPut, base+"/agent-access", map[string]any{"agentWriteEnabled": true}); rec.Code != http.StatusForbidden {
		t.Errorf("non-admin agent-access toggle = %d, want 403", rec.Code)
	}
	if rec := f.do(t, session, http.MethodGet, base+"/agent-activity", nil); rec.Code != http.StatusForbidden {
		t.Errorf("non-admin agent activity = %d, want 403", rec.Code)
	}
	enabled, err := db.ExecuteTransaction(func(tx *sql.Tx) (bool, error) {
		return transactional.OrganizationRepository.GetAgentWriteEnabled(tx, f.orgId)
	})
	if err != nil || enabled {
		t.Errorf("agent writes enabled = %v (%v), want still off", enabled, err)
	}
}

func TestDeviceLookup_reportsRequestedScope(t *testing.T) {
	f := setupAgentFixture(t)
	session, err := services.GenerateToken(f.userId, "agent@example.com")
	if err != nil {
		t.Fatal(err)
	}
	for client, want := range map[string]string{"traceway-cli": models.ScopeRead, "traceway-cli-write": models.ScopeWrite} {
		da, err := authserver.CreateDeviceAuth(client, "http://localhost")
		if err != nil {
			t.Fatalf("create device auth for %s: %v", client, err)
		}
		rec := f.do(t, session, http.MethodGet, "/api/device?user_code="+url.QueryEscape(da.UserCode), nil)
		var body models.DeviceLookupResponse
		_ = json.Unmarshal(rec.Body.Bytes(), &body)
		if rec.Code != http.StatusOK || body.Scope != want {
			t.Errorf("%s lookup = %d scope %q, want %q (the approval screen's write warning depends on it)", client, rec.Code, body.Scope, want)
		}
	}
}

func TestAgentAccessSettings(t *testing.T) {
	f := setupAgentFixture(t)
	session, err := services.GenerateToken(f.userId, "agent@example.com")
	if err != nil {
		t.Fatal(err)
	}
	base := "/api/organizations/" + itoa(f.orgId)

	rec := f.do(t, session, http.MethodGet, base+"/settings", nil)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"agentWriteEnabled":false`) {
		t.Fatalf("settings = %d %s", rec.Code, rec.Body.String())
	}
	if rec := f.do(t, session, http.MethodPut, base+"/agent-access", map[string]any{}); rec.Code != http.StatusUnprocessableEntity {
		t.Errorf("missing flag = %d, want 422", rec.Code)
	}
	if rec := f.do(t, session, http.MethodPut, base+"/agent-access", map[string]any{"agentWriteEnabled": true}); rec.Code != http.StatusOK {
		t.Fatalf("enable = %d %s", rec.Code, rec.Body.String())
	}
	if rec := f.do(t, session, http.MethodGet, base+"/settings", nil); !strings.Contains(rec.Body.String(), `"agentWriteEnabled":true`) {
		t.Errorf("settings after enable = %s", rec.Body.String())
	}

	write := f.pat(t, models.ScopeWrite)
	f.archive(t, write)
	rec = f.do(t, session, http.MethodGet, base+"/agent-activity", nil)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"total":1`) {
		t.Errorf("agent activity = %d %s", rec.Code, rec.Body.String())
	}
}

func TestPATCreation_scopes(t *testing.T) {
	f := setupAgentFixture(t)
	session, err := services.GenerateToken(f.userId, "agent@example.com")
	if err != nil {
		t.Fatal(err)
	}
	days := 30
	tooLong := 120

	rec := f.do(t, session, http.MethodPost, "/api/personal-access-tokens", map[string]any{"name": "default"})
	if rec.Code != http.StatusCreated || !strings.Contains(rec.Body.String(), `"token":"twp_`) || !strings.Contains(rec.Body.String(), `"scope":"traceway:read"`) {
		t.Errorf("default PAT = %d %s", rec.Code, rec.Body.String())
	}
	rec = f.do(t, session, http.MethodPost, "/api/personal-access-tokens", map[string]any{"name": "w", "scope": "write", "expiresInDays": days})
	if rec.Code != http.StatusCreated || !strings.Contains(rec.Body.String(), `"token":"tww_`) {
		t.Errorf("write PAT = %d %s", rec.Code, rec.Body.String())
	}
	for _, body := range []map[string]any{
		{"name": "w", "scope": "write"},
		{"name": "w", "scope": "write", "expiresInDays": tooLong},
		{"name": "w", "scope": "admin"},
	} {
		if rec := f.do(t, session, http.MethodPost, "/api/personal-access-tokens", body); rec.Code != http.StatusUnprocessableEntity {
			t.Errorf("PAT %v = %d, want 422", body, rec.Code)
		}
	}
	rec = f.do(t, session, http.MethodGet, "/api/personal-access-tokens", nil)
	if !strings.Contains(rec.Body.String(), `"scope":"traceway:write"`) {
		t.Errorf("PAT list should carry scopes: %s", rec.Body.String())
	}
}

func TestPasswordLogin_scopedLoginIsARevocableTokenSet(t *testing.T) {
	f := setupAgentFixture(t)
	hash, err := services.HashPassword("cli-password-1")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecuteTransaction(func(tx *sql.Tx) (*models.User, error) {
		return transactional.UserRepository.Create(tx, "cli@example.com", "CLI User", hash)
	}); err != nil {
		t.Fatal(err)
	}

	rec := f.do(t, "", http.MethodPost, "/api/login", map[string]any{"email": "cli@example.com", "password": "cli-password-1", "scope": models.ScopeWrite})
	if rec.Code != http.StatusOK {
		t.Fatalf("scoped login = %d %s", rec.Code, rec.Body.String())
	}
	var res struct {
		Token        string `json:"token"`
		RefreshToken string `json:"refreshToken"`
		ExpiresIn    int    `json:"expiresIn"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &res)
	claims, err := services.ValidateToken(res.Token)
	if err != nil {
		t.Fatalf("scoped login token: %v", err)
	}
	if claims.Scope != models.ScopeWrite || claims.ClientId != "traceway-cli-write" {
		t.Errorf("claims = scope %q cid %q, want the write scope from traceway-cli-write", claims.Scope, claims.ClientId)
	}
	if !strings.HasPrefix(res.RefreshToken, "twr_") || res.ExpiresIn <= 0 || res.ExpiresIn > 3600 {
		t.Errorf("scoped login must be a short access token plus a refresh token, got refresh %q expiresIn %d", res.RefreshToken, res.ExpiresIn)
	}

	if rec := f.do(t, "", http.MethodPost, "/api/auth/logout", map[string]any{"refresh_token": res.RefreshToken}); rec.Code != http.StatusOK {
		t.Fatalf("logout = %d %s", rec.Code, rec.Body.String())
	}
	if rec := f.do(t, "", http.MethodPost, "/api/auth/token", map[string]any{"grant_type": "refresh_token", "refresh_token": res.RefreshToken}); rec.Code != http.StatusBadRequest {
		t.Errorf("refresh after logout = %d %s, want 400", rec.Code, rec.Body.String())
	}

	rec = f.do(t, "", http.MethodPost, "/api/login", map[string]any{"email": "cli@example.com", "password": "cli-password-1"})
	if rec.Code != http.StatusOK || strings.Contains(rec.Body.String(), "refreshToken") {
		t.Errorf("an unscoped login stays a dashboard session: %d %s", rec.Code, rec.Body.String())
	}
}

func TestNotificationRuleToggle_explicitStateIsIdempotent(t *testing.T) {
	f := setupAgentFixture(t)
	f.setAgentWrites(t, true)
	now := time.Now().UTC()
	ruleId, err := db.ExecuteTransaction(func(tx *sql.Tx) (int, error) {
		channelId, err := transactional.NotificationChannelRepository.Create(tx, &models.NotificationChannel{ProjectId: f.project.Id, Name: "mail", ChannelType: "email", Config: json.RawMessage(`{}`), Enabled: true, CreatedAt: now, UpdatedAt: now})
		if err != nil {
			return 0, err
		}
		return transactional.NotificationRuleRepository.Create(tx, &models.NotificationRule{ProjectId: f.project.Id, ChannelId: channelId, Name: "p95", RuleType: "endpoint_p95_threshold", Config: json.RawMessage(`{}`), Enabled: true, Severity: "warning", CreatedAt: now, UpdatedAt: now})
	})
	if err != nil {
		t.Fatal(err)
	}
	path := "/api/notification-rules/" + itoa(ruleId) + "/toggle?projectId=" + f.project.Id.String()
	write := f.pat(t, models.ScopeWrite)
	session, err := services.GenerateToken(f.userId, "agent@example.com")
	if err != nil {
		t.Fatal(err)
	}

	for _, step := range []struct {
		name  string
		token string
		body  map[string]any
		want  string
	}{
		{"enable an enabled rule", write, map[string]any{"enabled": true}, `"enabled":true`},
		{"disable", write, map[string]any{"enabled": false}, `"enabled":false`},
		{"disable again", write, map[string]any{"enabled": false}, `"enabled":false`},
		{"dashboard flip", session, map[string]any{}, `"enabled":true`},
	} {
		rec := f.do(t, step.token, http.MethodPost, path, step.body)
		if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), step.want) {
			t.Errorf("%s = %d %s, want %s", step.name, rec.Code, rec.Body.String(), step.want)
		}
	}
}

func TestPostMortemUpdate_staleExpectedUpdatedAtConflicts(t *testing.T) {
	f := setupAgentFixture(t)
	session, err := services.GenerateToken(f.userId, "agent@example.com")
	if err != nil {
		t.Fatal(err)
	}
	pm := createTestPostMortem(t, f.orgId, f.project.Id, "Outage", "draft", nil, nil)
	path := "/api/post-mortems/" + itoa(pm.Id) + "?projectId=" + f.project.Id.String()

	read := func() time.Time {
		t.Helper()
		var doc struct {
			UpdatedAt time.Time `json:"updatedAt"`
		}
		rec := f.do(t, session, http.MethodGet, path, nil)
		if err := json.Unmarshal(rec.Body.Bytes(), &doc); err != nil || doc.UpdatedAt.IsZero() {
			t.Fatalf("get post-mortem = %d %s", rec.Code, rec.Body.String())
		}
		return doc.UpdatedAt
	}
	seen := read()

	if rec := f.do(t, session, http.MethodPut, path, map[string]any{"title": "Outage", "contentMd": "edited in the dashboard", "tags": []string{}}); rec.Code != http.StatusOK {
		t.Fatalf("dashboard update = %d %s", rec.Code, rec.Body.String())
	}
	if rec := f.do(t, session, http.MethodPut, path, map[string]any{"title": "Outage", "contentMd": "agent edit", "tags": []string{}, "expectedUpdatedAt": seen}); rec.Code != http.StatusConflict {
		t.Fatalf("update from a stale read = %d %s, want 409", rec.Code, rec.Body.String())
	}
	if rec := f.do(t, session, http.MethodPut, path, map[string]any{"title": "Outage", "contentMd": "agent edit", "tags": []string{}, "expectedUpdatedAt": read()}); rec.Code != http.StatusOK {
		t.Fatalf("update from a fresh read = %d %s", rec.Code, rec.Body.String())
	}
}

func itoa(n int) string {
	b, _ := json.Marshal(n)
	return string(b)
}
