//go:build !telemetry_ch && !transactional_pg && !telemetry_firebolt

package controllers

import (
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/tracewayapp/lit/v2"
	"github.com/tracewayapp/traceway/backend/app/config"
	"github.com/tracewayapp/traceway/backend/app/db"
	"github.com/tracewayapp/traceway/backend/app/middleware"
	"github.com/tracewayapp/traceway/backend/app/migrations"
	"github.com/tracewayapp/traceway/backend/app/models"
	"github.com/tracewayapp/traceway/backend/app/repositories/transactional"
	"github.com/tracewayapp/traceway/backend/app/services"
	_ "modernc.org/sqlite"
)

func setupSetupControllerDB(t *testing.T) {
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

	prevDB, prevTelemetry, prevDriver := db.DB, db.TelemetryDB, db.Driver
	db.DB = mainDB
	db.TelemetryDB = telemetryDB
	db.Driver = lit.SQLite
	models.Init(db.Driver)

	prevConfig := config.Config
	if config.Config == nil {
		config.Init(&config.Cfg{})
	}

	t.Cleanup(func() {
		mainDB.Close()
		telemetryDB.Close()
		db.DB = prevDB
		db.TelemetryDB = prevTelemetry
		db.Driver = prevDriver
		config.Init(prevConfig)
	})

	if err := migrations.Run("sqlite"); err != nil {
		t.Fatalf("run migrations: %v", err)
	}
}

func createSetupTestAccount(t *testing.T, tx *sql.Tx, email, role string) (userId int, orgId int) {
	t.Helper()
	user, err := transactional.UserRepository.Create(tx, email, "Test User", "hashed-password")
	if err != nil {
		t.Fatalf("create user: %v", err)
	}
	org, err := transactional.OrganizationRepository.Create(tx, "Test Org "+email, "UTC")
	if err != nil {
		t.Fatalf("create org: %v", err)
	}
	if _, err := transactional.OrganizationRepository.AddUser(tx, org.Id, user.Id, role); err != nil {
		t.Fatalf("add user to org: %v", err)
	}
	return user.Id, org.Id
}

func newControllerTestContext(t *testing.T, tx *sql.Tx, userId int, method, path, body string) (*gin.Context, *httptest.ResponseRecorder) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	var reader *strings.Reader
	if body == "" {
		reader = strings.NewReader("")
	} else {
		reader = strings.NewReader(body)
	}
	c.Request = httptest.NewRequest(method, path, reader)
	c.Request.Header.Set("Content-Type", "application/json")
	if tx != nil {
		c.Set(db.TransactionContextKey, tx)
	}
	if userId != 0 {
		c.Set(middleware.UserIdContextKey, userId)
	}
	return c, recorder
}

func TestBatchCreateProjectsCreatesAndIsIdempotent(t *testing.T) {
	setupSetupControllerDB(t)

	tx, err := db.DB.Begin()
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	defer tx.Rollback()

	userId, orgId := createSetupTestAccount(t, tx, "batch@example.com", "owner")

	inputs := []BatchProjectInput{
		{Name: "Product Backend", Framework: "opentelemetry"},
		{Name: "Product Web", Framework: "react"},
	}
	results, err := batchCreateProjects(tx, orgId, userId, inputs)
	if err != nil {
		t.Fatalf("batch create: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("results = %d, want 2", len(results))
	}
	for i, r := range results {
		if r.Status != "created" {
			t.Errorf("results[%d].Status = %q, want created", i, r.Status)
		}
		if r.Project.Token == "" {
			t.Errorf("results[%d] has no token", i)
		}
		if len(r.Project.AiFlaggedLanguages) == 0 {
			t.Errorf("results[%d] has empty AiFlaggedLanguages", i)
		}
	}

	assignments, err := transactional.DashboardRepository.FindAssignmentsByProject(tx, results[0].Project.Id)
	if err != nil {
		t.Fatalf("list assignments: %v", err)
	}
	if len(assignments) == 0 {
		t.Error("opentelemetry project should get default dashboards")
	}

	rerun, err := batchCreateProjects(tx, orgId, userId, inputs)
	if err != nil {
		t.Fatalf("rerun batch create: %v", err)
	}
	for i, r := range rerun {
		if r.Status != "existing" {
			t.Errorf("rerun[%d].Status = %q, want existing", i, r.Status)
		}
		if r.Project.Id != results[i].Project.Id {
			t.Errorf("rerun[%d] returned a different project id", i)
		}
	}

	folded, err := batchCreateProjects(tx, orgId, userId, []BatchProjectInput{
		{Name: "Workers", Framework: "opentelemetry"},
		{Name: "Workers", Framework: "opentelemetry"},
	})
	if err != nil {
		t.Fatalf("folded batch create: %v", err)
	}
	if folded[0].Status != "created" || folded[1].Status != "existing" || folded[0].Project.Id != folded[1].Project.Id {
		t.Errorf("duplicate names should fold onto one project: %+v", folded)
	}

	_, err = batchCreateProjects(tx, orgId, userId, []BatchProjectInput{
		{Name: "Product Web", Framework: "opentelemetry"},
	})
	var validationErr *batchValidationError
	if !errors.As(err, &validationErr) || !strings.Contains(validationErr.Message, "already exists with framework react") {
		t.Fatalf("framework mismatch = %v, want a validation error naming the existing framework", err)
	}
}

func TestBatchCreateProjectsValidation(t *testing.T) {
	setupSetupControllerDB(t)

	tx, err := db.DB.Begin()
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	defer tx.Rollback()

	userId, orgId := createSetupTestAccount(t, tx, "batch@example.com", "owner")

	cases := map[string][]BatchProjectInput{
		"empty":             {},
		"invalid framework": {{Name: "App", Framework: "nope"}},
		"bad name":          {{Name: "Bad!Name", Framework: "opentelemetry"}},
		"empty name":        {{Name: "   ", Framework: "opentelemetry"}},
	}
	tooMany := make([]BatchProjectInput, maxBatchProjects+1)
	for i := range tooMany {
		tooMany[i] = BatchProjectInput{Name: "P" + strings.Repeat("x", i%5), Framework: "opentelemetry"}
	}
	cases["too many"] = tooMany

	for name, inputs := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := batchCreateProjects(tx, orgId, userId, inputs)
			var validationErr *batchValidationError
			if !errors.As(err, &validationErr) {
				t.Errorf("expected batchValidationError, got %v", err)
			}
		})
	}
}

func TestBatchCreateProjectsLimitHookAbortsBatch(t *testing.T) {
	setupSetupControllerDB(t)

	tx, err := db.DB.Begin()
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	defer tx.Rollback()

	userId, orgId := createSetupTestAccount(t, tx, "batch@example.com", "owner")

	calls := 0
	prevHook := ProjectLimitHook
	ProjectLimitHook = func(tx *sql.Tx, orgId int) error {
		calls++
		if calls > 1 {
			return &LimitExceededError{Message: "plan limit reached"}
		}
		return nil
	}
	defer func() { ProjectLimitHook = prevHook }()

	_, err = batchCreateProjects(tx, orgId, userId, []BatchProjectInput{
		{Name: "One", Framework: "opentelemetry"},
		{Name: "Two", Framework: "opentelemetry"},
	})
	var limitErr *LimitExceededError
	if !errors.As(err, &limitErr) {
		t.Fatalf("expected LimitExceededError, got %v", err)
	}
}

func TestRegisterWithoutProject(t *testing.T) {
	setupSetupControllerDB(t)

	prevSecret := config.Config.JWTSecret
	config.Config.JWTSecret = strings.Repeat("s", 32)
	if err := services.InitJWT(); err != nil {
		t.Fatalf("init jwt: %v", err)
	}
	services.InitTurnstile()
	t.Cleanup(func() {
		config.Config.JWTSecret = prevSecret
		_ = services.InitJWT()
	})

	register := func(t *testing.T, body string) (*httptest.ResponseRecorder, map[string]any) {
		t.Helper()
		tx, err := db.DB.Begin()
		if err != nil {
			t.Fatalf("begin: %v", err)
		}
		c, recorder := newControllerTestContext(t, tx, 0, http.MethodPost, "/api/register", body)
		AuthController.Register(c)
		if recorder.Code >= 200 && recorder.Code < 300 {
			if err := tx.Commit(); err != nil {
				t.Fatalf("commit: %v", err)
			}
		} else {
			tx.Rollback()
		}
		var payload map[string]any
		_ = json.Unmarshal(recorder.Body.Bytes(), &payload)
		return recorder, payload
	}

	recorder, _ := register(t, `{"email":"half@example.com","name":"A","password":"password1","organizationName":"HalfOrg","timezone":"UTC","projectName":"OnlyName"}`)
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("half project fields: status = %d, want 400", recorder.Code)
	}

	recorder, payload := register(t, `{"email":"nop@example.com","name":"A","password":"password1","organizationName":"NopOrg","timezone":"UTC"}`)
	if recorder.Code != http.StatusCreated {
		t.Fatalf("no project: status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	if _, exists := payload["project"]; exists {
		t.Error("response must omit the project key when no project was created")
	}
	if projects, ok := payload["projects"].([]any); !ok || len(projects) != 0 {
		t.Errorf("projects = %v, want empty list", payload["projects"])
	}

	var count int
	if err := db.DB.QueryRow("SELECT COUNT(*) FROM projects").Scan(&count); err != nil {
		t.Fatalf("count projects: %v", err)
	}
	if count != 0 {
		t.Fatalf("projects in db = %d, want 0", count)
	}
}

func submitTestPlan(t *testing.T, tx *sql.Tx, userId, orgId int, payload string) {
	t.Helper()
	c, recorder := newControllerTestContext(t, tx, userId, http.MethodPut, "/api/setup/plan", payload)
	c.Set(middleware.SetupOrganizationIdContextKey, orgId)
	c.Set(middleware.SetupScopeContextKey, true)
	SetupController.SubmitPlan(c)
	if recorder.Code != http.StatusOK {
		t.Fatalf("submit plan: status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
}

func latestDraftId(t *testing.T, tx *sql.Tx, userId, orgId int) string {
	t.Helper()
	plan, err := transactional.SetupPlanRepository.FindLatestByUserAndOrganization(tx, userId, orgId)
	if err != nil || plan == nil {
		t.Fatalf("load latest plan: (%v, %v)", plan, err)
	}
	return plan.Id
}

func TestSetupDraftLifecycle(t *testing.T) {
	setupSetupControllerDB(t)

	tx, err := db.DB.Begin()
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	defer tx.Rollback()

	userId, orgId := createSetupTestAccount(t, tx, "draft@example.com", "owner")

	planBody := `{"projects":[
		{"name":"Product Backend","framework":"opentelemetry","envFile":"backend/.env","envVar":"TRACEWAY_BACKEND_TOKEN"},
		{"name":"Product Web","framework":"react","envFile":"web/.env","envVar":"VITE_TRACEWAY_CONNECTION_STRING","envFormat":"connectionString",
		 "deployment":{"platform":"Vercel","instructions":"vercel env add VITE_TRACEWAY_CONNECTION_STRING <token>"}}
	]}`
	submitTestPlan(t, tx, userId, orgId, planBody)

	c, recorder := newControllerTestContext(t, tx, userId, http.MethodGet, "/api/setup/plan", "")
	c.Set(middleware.SetupOrganizationIdContextKey, orgId)
	SetupController.GetPlan(c)
	if recorder.Code != http.StatusOK || !strings.Contains(recorder.Body.String(), `"pending"`) {
		t.Fatalf("get plan: status = %d, body = %s", recorder.Code, recorder.Body.String())
	}

	c, recorder = newControllerTestContext(t, tx, userId, http.MethodGet, "/api/setup/drafts?organizationId="+strconv.Itoa(orgId), "")
	SetupController.ListDraft(c)
	if recorder.Code != http.StatusOK || !strings.Contains(recorder.Body.String(), "Product Backend") {
		t.Fatalf("list draft: status = %d, body = %s", recorder.Code, recorder.Body.String())
	}

	draftId := latestDraftId(t, tx, userId, orgId)
	c, recorder = newControllerTestContext(t, tx, userId, http.MethodPost, "/api/setup/drafts/"+draftId+"/approve", "")
	c.Params = gin.Params{{Key: "id", Value: draftId}}
	SetupController.ApproveDraft(c)
	if recorder.Code != http.StatusOK {
		t.Fatalf("approve: status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	var approveResponse struct {
		Projects []BatchProjectResponseItem `json:"projects"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &approveResponse); err != nil {
		t.Fatalf("parse approve response: %v", err)
	}
	if len(approveResponse.Projects) != 2 || approveResponse.Projects[0].Token == "" {
		t.Fatalf("approve response = %+v", approveResponse)
	}

	readonly, err := transactional.UserRepository.Create(tx, "readonly@example.com", "Read Only", "hashed-password")
	if err != nil {
		t.Fatalf("create read-only user: %v", err)
	}
	if _, err := transactional.OrganizationRepository.AddUser(tx, orgId, readonly.Id, "readonly"); err != nil {
		t.Fatalf("add read-only user: %v", err)
	}
	c, recorder = newControllerTestContext(t, tx, readonly.Id, http.MethodGet, "/api/setup/drafts?organizationId="+strconv.Itoa(orgId), "")
	SetupController.ListDraft(c)
	if recorder.Code != http.StatusForbidden {
		t.Fatalf("read-only list draft: status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	if strings.Contains(recorder.Body.String(), approveResponse.Projects[0].Token) {
		t.Fatal("read-only draft response exposed a project token")
	}

	c, recorder = newControllerTestContext(t, tx, userId, http.MethodPost, "/api/setup/drafts/"+draftId+"/approve", "")
	c.Params = gin.Params{{Key: "id", Value: draftId}}
	SetupController.ApproveDraft(c)
	if recorder.Code != http.StatusConflict {
		t.Fatalf("second approve: status = %d, want 409", recorder.Code)
	}

	c, recorder = newControllerTestContext(t, tx, userId, http.MethodGet, "/api/setup/plan", "")
	c.Set(middleware.SetupOrganizationIdContextKey, orgId)
	SetupController.GetPlan(c)
	if !strings.Contains(recorder.Body.String(), `"approved"`) || !strings.Contains(recorder.Body.String(), `"token"`) {
		t.Fatalf("approved plan body = %s", recorder.Body.String())
	}

	submitTestPlan(t, tx, userId, orgId, `{"projects":[{"name":"Another","framework":"opentelemetry"}]}`)
	draftId = latestDraftId(t, tx, userId, orgId)
	c, recorder = newControllerTestContext(t, tx, userId, http.MethodPost, "/api/setup/drafts/"+draftId+"/reject", `{"reason":"wrong project split"}`)
	c.Params = gin.Params{{Key: "id", Value: draftId}}
	SetupController.RejectDraft(c)
	if recorder.Code != http.StatusOK {
		t.Fatalf("reject: status = %d, body = %s", recorder.Code, recorder.Body.String())
	}

	c, recorder = newControllerTestContext(t, tx, userId, http.MethodGet, "/api/setup/plan", "")
	c.Set(middleware.SetupOrganizationIdContextKey, orgId)
	SetupController.GetPlan(c)
	if !strings.Contains(recorder.Body.String(), `"rejected"`) || !strings.Contains(recorder.Body.String(), "wrong project split") {
		t.Fatalf("rejected plan body = %s", recorder.Body.String())
	}
}
func TestSubmitPlanValidation(t *testing.T) {
	setupSetupControllerDB(t)

	tx, err := db.DB.Begin()
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	defer tx.Rollback()

	userId, orgId := createSetupTestAccount(t, tx, "plan@example.com", "owner")

	cases := map[string]string{
		"empty projects":    `{"projects":[]}`,
		"bad framework":     `{"projects":[{"name":"App","framework":"laravel-x"}]}`,
		"duplicate names":   `{"projects":[{"name":"App","framework":"react"},{"name":"App","framework":"svelte"}]}`,
		"bad env var":       `{"projects":[{"name":"App","framework":"react","envVar":"1BAD"}]}`,
		"bad env format":    `{"projects":[{"name":"App","framework":"react","envFormat":"base64"}]}`,
		"long instructions": `{"projects":[{"name":"App","framework":"react","deployment":{"platform":"Fly","instructions":"` + strings.Repeat("x", 2100) + `"}}]}`,
	}
	for name, body := range cases {
		t.Run(name, func(t *testing.T) {
			c, recorder := newControllerTestContext(t, tx, userId, http.MethodPut, "/api/setup/plan", body)
			c.Set(middleware.SetupOrganizationIdContextKey, orgId)
			SetupController.SubmitPlan(c)
			if recorder.Code != http.StatusUnprocessableEntity {
				t.Errorf("status = %d, want 422 (body: %s)", recorder.Code, recorder.Body.String())
			}
		})
	}
}
