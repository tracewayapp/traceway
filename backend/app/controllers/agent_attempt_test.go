//go:build !telemetry_ch && !transactional_pg && !telemetry_duckdb

package controllers

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/tracewayapp/traceway/backend/app/agent"
	"github.com/tracewayapp/traceway/backend/app/agentrunner"
	"github.com/tracewayapp/traceway/backend/app/config"
	"github.com/tracewayapp/traceway/backend/app/db"
	"github.com/tracewayapp/traceway/backend/app/middleware"
	"github.com/tracewayapp/traceway/backend/app/models"
	"github.com/tracewayapp/traceway/backend/app/repositories/transactional"
	"github.com/tracewayapp/traceway/backend/app/services"
)

func setupAgentControllerDB(t *testing.T) {
	t.Helper()
	setupSetupControllerDB(t)
	config.Config.JWTSecret = "test-secret-that-is-long-enough-for-jwt-0123"
	if err := services.InitJWT(); err != nil {
		t.Fatal(err)
	}
}

func agentRequest(t *testing.T, tx *sql.Tx, projectId uuid.UUID, userId int, method, path, body string) (*gin.Context, *httptest.ResponseRecorder) {
	t.Helper()
	c, recorder := newControllerTestContext(t, tx, userId, method, path, body)
	c.Set(middleware.ProjectIdContextKey, projectId)
	return c, recorder
}

func decodeJSON(t *testing.T, body string) map[string]any {
	t.Helper()
	var out map[string]any
	if err := json.Unmarshal([]byte(body), &out); err != nil {
		t.Fatalf("decode %s: %v", body, err)
	}
	return out
}

func TestAgentCapabilitiesFollowDeploymentConfig(t *testing.T) {
	setupAgentControllerDB(t)
	previousMode := config.Config.AgentMode
	t.Cleanup(func() { config.Config.AgentMode = previousMode })
	for _, tc := range []struct {
		mode    string
		enabled bool
	}{
		{"", false},
		{"off", false},
		{"embedded", true},
		{"remote", true},
	} {
		t.Run(tc.mode, func(t *testing.T) {
			config.Config.AgentMode = tc.mode
			rec := httptest.NewRecorder()
			ctx, _ := gin.CreateTestContext(rec)
			AgentAttemptController.Capabilities(ctx)
			if rec.Code != 200 || decodeJSON(t, rec.Body.String())["enabled"] != tc.enabled {
				t.Fatalf("capabilities: %d %s", rec.Code, rec.Body.String())
			}
		})
	}
}

func TestAgentPreflightStartAndEvents(t *testing.T) {
	setupAgentControllerDB(t)
	previousMode := config.Config.AgentMode
	config.Config.AgentMode = ""
	t.Cleanup(func() { config.Config.AgentMode = previousMode })
	tx, err := db.DB.Begin()
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	t.Cleanup(func() { tx.Rollback() })

	ownerId, orgId := createSetupTestAccount(t, tx, "owner@example.com", "owner")
	projectId := createChannelTestProject(t, tx, orgId)
	hash := "0123456789abcdef"

	c, rec := agentRequest(t, tx, projectId, ownerId, "GET", "/agent/preflight?hash="+hash, "")
	AgentAttemptController.Preflight(c)
	if rec.Code != 200 {
		t.Fatalf("preflight: %d %s", rec.Code, rec.Body.String())
	}
	preflight := decodeJSON(t, rec.Body.String())
	checks := preflight["checks"].([]any)
	if len(checks) != 4 || preflight["nextNumber"] != float64(1) || preflight["canStart"] != false {
		t.Fatalf("preflight = %v", preflight)
	}
	for _, raw := range checks[:3] {
		check := raw.(map[string]any)
		if check["ok"] != false || check["hint"] == "" {
			t.Fatalf("an unwired check must fail with a hint: %v", check)
		}
	}
	if checks[3].(map[string]any)["ok"] != true {
		t.Fatalf("the browser conversation is always available: %v", checks[3])
	}

	c, rec = agentRequest(t, tx, projectId, ownerId, "GET", "/agent/preflight?hash=nope", "")
	AgentAttemptController.Preflight(c)
	if rec.Code != 400 {
		t.Fatalf("bad hash: %d", rec.Code)
	}

	assertBlocked := func(hint string) {
		t.Helper()
		c, rec := agentRequest(t, tx, projectId, ownerId, "POST", "/agent/attempts", `{"hash":"`+hash+`"}`)
		AgentAttemptController.Start(c)
		if rec.Code != 422 || !strings.Contains(rec.Body.String(), hint) {
			t.Fatalf("expected setup failure containing %q: %d %s", hint, rec.Code, rec.Body.String())
		}
	}
	assertBlocked("Connect GitHub")
	now := time.Now().UTC()
	integration := &models.Integration{OrganizationId: orgId, Provider: "github", Kinds: models.StringSlice{agent.KindCodeHost}, Name: "GitHub", Config: models.JSONText(`{"mode":"app"}`), Enabled: true, CreatedAt: now, UpdatedAt: now}
	integration.Id, err = transactional.IntegrationRepository.Create(tx, integration)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := transactional.RepositoryRepository.Create(tx, &models.Repository{ProjectId: projectId, IntegrationId: &integration.Id, Owner: "acme", Name: "app", DefaultBranch: "main", CreatedAt: now, UpdatedAt: now}); err != nil {
		t.Fatal(err)
	}
	assertBlocked("Finish connecting GitHub")
	integration.Config = models.JSONText(`{"token":"test-pat"}`)
	if err := transactional.IntegrationRepository.Update(tx, integration); err != nil {
		t.Fatal(err)
	}
	assertBlocked("Choose an agent profile")
	profile := &models.AgentProfile{OrganizationId: orgId, Name: "Default", Agent: "claude-code", Provider: "anthropic", Credential: "test-key", IsDefault: true, AllowedTools: models.StringSlice{}, NetworkPolicy: models.JSONText(`{}`), CreatedAt: now, UpdatedAt: now}
	profile.Id, err = transactional.AgentProfileRepository.Create(tx, profile)
	if err != nil {
		t.Fatal(err)
	}
	assertBlocked("not enabled on this instance")
	for _, mode := range []string{agentrunner.ModeEmbedded, agentrunner.ModeRemote} {
		config.Config.AgentMode = mode
		c, rec = agentRequest(t, tx, projectId, ownerId, "GET", "/agent/preflight?hash="+hash, "")
		AgentAttemptController.Preflight(c)
		if rec.Code != 200 || decodeJSON(t, rec.Body.String())["canStart"] != true {
			t.Fatalf("configured %s without runner polls: %d %s", mode, rec.Code, rec.Body.String())
		}
	}
	for _, selected := range []int{0, profile.Id + 1} {
		c, rec = agentRequest(t, tx, projectId, ownerId, "POST", "/agent/attempts", fmt.Sprintf(`{"hash":%q,"profileId":%d}`, hash, selected))
		AgentAttemptController.Start(c)
		if rec.Code != 422 {
			t.Fatalf("invalid selected profile: %d %s", rec.Code, rec.Body.String())
		}
	}
	profile.IsDefault = false
	if err := transactional.AgentProfileRepository.Update(tx, profile); err != nil {
		t.Fatal(err)
	}
	assertBlocked("Choose an agent profile")
	c, rec = agentRequest(t, tx, projectId, ownerId, "GET", fmt.Sprintf("/agent/preflight?hash=%s&profileId=%d", hash, profile.Id), "")
	AgentAttemptController.Preflight(c)
	if rec.Code != 200 || decodeJSON(t, rec.Body.String())["canStart"] != true {
		t.Fatalf("explicit non-default profile: %d %s", rec.Code, rec.Body.String())
	}

	c, rec = agentRequest(t, tx, projectId, ownerId, "POST", "/agent/attempts", fmt.Sprintf(`{"hash":%q,"profileId":%d}`, hash, profile.Id))
	AgentAttemptController.Start(c)
	if rec.Code != 201 {
		t.Fatalf("start: %d %s", rec.Code, rec.Body.String())
	}
	started := decodeJSON(t, rec.Body.String())
	attempt := started["attempt"].(map[string]any)
	if started["existing"] != false || attempt["number"] != float64(1) || attempt["status"] != models.AttemptQueued {
		t.Fatalf("start = %v", started)
	}
	attemptId := attempt["id"].(string)
	if attempt["profileId"] != float64(profile.Id) {
		t.Fatalf("selected profile not used: %v", attempt)
	}
	profile.IsDefault = true
	if err := transactional.AgentProfileRepository.Update(tx, profile); err != nil {
		t.Fatal(err)
	}

	c, rec = agentRequest(t, tx, projectId, ownerId, "POST", "/agent/attempts", `{"hash":"`+hash+`"}`)
	AgentAttemptController.Start(c)
	if rec.Code != 200 || decodeJSON(t, rec.Body.String())["existing"] != true {
		t.Fatalf("second start: %d %s", rec.Code, rec.Body.String())
	}

	AttemptLimitHook = func(*sql.Tx, int) error { return &LimitExceededError{Message: "plan limit"} }
	t.Cleanup(func() { AttemptLimitHook = nil })
	c, rec = agentRequest(t, tx, projectId, ownerId, "POST", "/agent/attempts", `{"hash":"ffffffffffffffff"}`)
	AgentAttemptController.Start(c)
	if rec.Code != 422 || !strings.Contains(rec.Body.String(), "plan limit") {
		t.Fatalf("limit hook: %d %s", rec.Code, rec.Body.String())
	}
	AttemptLimitHook = nil

	id := uuid.MustParse(attemptId)
	for i := 0; i < 3; i++ {
		if _, err := agent.AppendEvent(tx, id, "progress", map[string]any{"i": i}, now); err != nil {
			t.Fatal(err)
		}
	}
	c, rec = agentRequest(t, tx, projectId, ownerId, "GET", "/agent/attempts/"+attemptId+"/events?after=1", "")
	c.Params = gin.Params{{Key: "id", Value: attemptId}}
	AgentAttemptController.Events(c)
	if rec.Code != 200 {
		t.Fatalf("events: %d %s", rec.Code, rec.Body.String())
	}
	events := decodeJSON(t, rec.Body.String())["events"].([]any)
	if len(events) != 3 {
		t.Fatalf("events after=1 must skip the created event only: %d", len(events))
	}
	previous := 1.0
	for _, raw := range events {
		seq := raw.(map[string]any)["seq"].(float64)
		if seq <= previous {
			t.Fatalf("seq not strictly increasing: %v", events)
		}
		previous = seq
	}

	c, rec = agentRequest(t, tx, projectId, ownerId, "GET", "/agent/attempts/by-subject?hash="+hash, "")
	AgentAttemptController.BySubject(c)
	card := decodeJSON(t, rec.Body.String())["attempts"].([]any)
	if rec.Code != 200 || len(card) != 1 || len(card[0].(map[string]any)["links"].([]any)) != 2 {
		t.Fatalf("by-subject: %d %s", rec.Code, rec.Body.String())
	}

	otherProject := createChannelTestProject(t, tx, orgId)
	c, rec = agentRequest(t, tx, otherProject, ownerId, "GET", "/agent/attempts/"+attemptId, "")
	c.Params = gin.Params{{Key: "id", Value: attemptId}}
	AgentAttemptController.Get(c)
	if rec.Code != 404 {
		t.Fatalf("an attempt of another project must be a 404, got %d", rec.Code)
	}

	c, rec = agentRequest(t, tx, projectId, ownerId, "POST", "/agent/attempts/"+attemptId+"/messages", `{"body":"go ahead"}`)
	c.Params = gin.Params{{Key: "id", Value: attemptId}}
	AgentAttemptController.PostMessage(c)
	if rec.Code != 201 {
		t.Fatalf("post message: %d %s", rec.Code, rec.Body.String())
	}
	c, rec = agentRequest(t, tx, projectId, ownerId, "POST", "/agent/attempts/"+attemptId+"/cancel", "")
	c.Params = gin.Params{{Key: "id", Value: attemptId}}
	AgentAttemptController.Cancel(c)
	if rec.Code != 200 {
		t.Fatalf("cancel: %d %s", rec.Code, rec.Body.String())
	}
	c, rec = agentRequest(t, tx, projectId, ownerId, "POST", "/agent/attempts/"+attemptId+"/cancel", "")
	c.Params = gin.Params{{Key: "id", Value: attemptId}}
	AgentAttemptController.Cancel(c)
	if rec.Code != 409 {
		t.Fatalf("second cancel must be a 409, got %d", rec.Code)
	}
	c, rec = agentRequest(t, tx, projectId, ownerId, "POST", "/agent/attempts/"+attemptId+"/messages", `{"body":"too late"}`)
	c.Params = gin.Params{{Key: "id", Value: attemptId}}
	AgentAttemptController.PostMessage(c)
	if rec.Code != 422 {
		t.Fatalf("a message on a finished attempt must be a 422, got %d", rec.Code)
	}
}

func TestRunTokenScopesReadsToItsProjectAndNeverWrites(t *testing.T) {
	setupAgentControllerDB(t)
	middleware.InitUseAppAuth()
	middleware.InitRequireProjectAccess()
	middleware.InitRequireWriteAccess()

	tx, err := db.DB.Begin()
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	_, orgId := createSetupTestAccount(t, tx, "owner@example.com", "owner")
	projectId := createChannelTestProject(t, tx, orgId)
	otherProject := createChannelTestProject(t, tx, orgId)
	attemptId := uuid.New()
	now := time.Now().UTC()
	if err := transactional.AgentAttemptRepository.Create(tx, &models.AgentAttempt{Id: attemptId, OrganizationId: orgId, ProjectId: projectId, Number: 1, Kind: models.AttemptKindFix, SubjectKind: models.SubjectKindTracewayException, SubjectRef: "0123456789abcdef", Status: models.AttemptRunning, CreatedAt: now, UpdatedAt: now}); err != nil {
		t.Fatal(err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
	token, _, err := services.GenerateRunToken(attemptId, projectId, agent.RunTokenTTL)
	if err != nil {
		t.Fatal(err)
	}

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.GET("/read", middleware.UseAppAuth, middleware.RequireProjectAccess, func(c *gin.Context) { c.String(200, "ok") })
	router.POST("/write", middleware.UseAppAuth, middleware.RequireProjectAccess, middleware.RequireWriteAccess, func(c *gin.Context) { c.String(200, "wrote") })
	router.POST("/agent/run-token", middleware.UseAppAuth, middleware.Transactional, AgentAttemptController.RenewRunToken)

	call := func(method, path, bearer string) *httptest.ResponseRecorder {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(method, path, strings.NewReader(""))
		req.Header.Set("Authorization", "Bearer "+bearer)
		router.ServeHTTP(rec, req)
		return rec
	}
	if rec := call("GET", "/read?projectId="+projectId.String(), token); rec.Code != 200 {
		t.Fatalf("own project read: %d %s", rec.Code, rec.Body.String())
	}
	if rec := call("GET", "/read?projectId="+otherProject.String(), token); rec.Code != 403 {
		t.Fatalf("another project must be forbidden, got %d", rec.Code)
	}
	if rec := call("POST", "/write?projectId="+projectId.String(), token); rec.Code != 401 {
		t.Fatalf("a run token must never write, got %d", rec.Code)
	}
	expired, _, err := services.GenerateRunToken(attemptId, projectId, -time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	if rec := call("GET", "/read?projectId="+projectId.String(), expired); rec.Code != 401 {
		t.Fatalf("an expired run token must be rejected, got %d", rec.Code)
	}

	rec := call("POST", "/agent/run-token", token)
	if rec.Code != 403 {
		t.Fatalf("a sandbox token cannot renew itself: %d %s", rec.Code, rec.Body.String())
	}
	userToken, err := services.GenerateToken(1, "owner@example.com")
	if err != nil {
		t.Fatal(err)
	}
	if rec := call("POST", "/agent/run-token", userToken); rec.Code != 403 {
		t.Fatalf("a user token cannot mint run tokens, got %d", rec.Code)
	}
}

func TestReadonlyMemberCannotStartOrReply(t *testing.T) {
	setupAgentControllerDB(t)
	middleware.InitUseAppAuth()
	middleware.InitRequireProjectAccess()
	middleware.InitRequireWriteAccess()
	tx, err := db.DB.Begin()
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	_, orgId := createSetupTestAccount(t, tx, "owner@example.com", "owner")
	readerId := addOrgMember(t, tx, orgId, "reader@example.com", "readonly")
	projectId := createChannelTestProject(t, tx, orgId)
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.POST("/agent/attempts", middleware.UseAppAuth, middleware.RequireProjectAccess, middleware.RequireWriteAccess, middleware.Transactional, AgentAttemptController.Start)
	router.GET("/agent/preflight", middleware.UseAppAuth, middleware.RequireProjectAccess, middleware.Transactional, AgentAttemptController.Preflight)
	readerToken, err := services.GenerateToken(readerId, "reader@example.com")
	if err != nil {
		t.Fatal(err)
	}
	call := func(method, path, body string) *httptest.ResponseRecorder {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(method, path, strings.NewReader(body))
		req.Header.Set("Authorization", "Bearer "+readerToken)
		req.Header.Set("Content-Type", "application/json")
		router.ServeHTTP(rec, req)
		return rec
	}
	if rec := call("GET", "/agent/preflight?projectId="+projectId.String()+"&hash=0123456789abcdef", ""); rec.Code != 200 {
		t.Fatalf("a readonly member may read the preflight: %d %s", rec.Code, rec.Body.String())
	}
	if rec := call("POST", "/agent/attempts?projectId="+projectId.String(), `{"hash":"0123456789abcdef"}`); rec.Code != 403 {
		t.Fatalf("a readonly member must not start attempts: %d %s", rec.Code, rec.Body.String())
	}
}
