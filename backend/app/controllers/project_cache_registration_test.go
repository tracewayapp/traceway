//go:build !telemetry_ch && !transactional_pg && !telemetry_duckdb

package controllers

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/tracewayapp/traceway/backend/app/cache"
	"github.com/tracewayapp/traceway/backend/app/config"
	"github.com/tracewayapp/traceway/backend/app/db"
	"github.com/tracewayapp/traceway/backend/app/middleware"
	"github.com/tracewayapp/traceway/backend/app/models"
	"github.com/tracewayapp/traceway/backend/app/repositories/transactional"
	"github.com/tracewayapp/traceway/backend/app/services"
)

func initRegistrationJWT(t *testing.T) {
	t.Helper()
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
}

func findProjectByName(t *testing.T, name string) *models.Project {
	t.Helper()
	tx, err := db.DB.Begin()
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	defer tx.Rollback()

	projects, err := transactional.ProjectRepository.FindAll(tx)
	if err != nil {
		t.Fatalf("find all projects: %v", err)
	}
	for _, p := range projects {
		if p.Name == name {
			return p
		}
	}
	return nil
}

// The register handler used to cache a five-field copy of the project it had
// just created, so the cached ingest settings disagreed with the row. On the
// SQLite builds nothing ever refreshes the cache, so the wrong copy served
// every /api/report request until the process restarted.
func TestRegisterCachesTheProjectItCreated(t *testing.T) {
	setupSetupControllerDB(t)
	initRegistrationJWT(t)

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.POST("/api/register", middleware.Transactional, AuthController.Register)

	body := `{"email":"cached@example.com","name":"A","password":"password1","organizationName":"CachedOrg","timezone":"UTC","projectName":"Mobile App","framework":"ios"}`
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/register", strings.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusCreated {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
	}

	stored := findProjectByName(t, "Mobile App")
	if stored == nil {
		t.Fatal("project was not created")
	}
	t.Cleanup(func() { cache.ProjectCache.RemoveProject(stored.Id) })

	cached := cache.ProjectCache.GetByToken(stored.Token)
	if cached == nil {
		t.Fatal("project token is not in the cache after a committed registration")
	}

	if cached.DropHealthyHealthchecks != stored.DropHealthyHealthchecks {
		t.Errorf("cached DropHealthyHealthchecks = %v, stored = %v; healthy healthchecks would be ingested",
			cached.DropHealthyHealthchecks, stored.DropHealthyHealthchecks)
	}
	if stored.SourceMapToken == nil {
		t.Fatal("an ios project should be created with a source map token")
	}
	if cached.SourceMapToken == nil || *cached.SourceMapToken != *stored.SourceMapToken {
		t.Errorf("cached SourceMapToken = %v, stored = %v", cached.SourceMapToken, *stored.SourceMapToken)
	}
	if cache.ProjectCache.GetBySourceMapToken(*stored.SourceMapToken) == nil {
		t.Error("source map token does not resolve from the cache; symbol upload would 401")
	}
	if len(cached.AiFlaggedLanguages) != len(stored.AiFlaggedLanguages) {
		t.Errorf("cached AiFlaggedLanguages = %v, stored = %v", cached.AiFlaggedLanguages, stored.AiFlaggedLanguages)
	}
	if cached.CreatedAt.IsZero() {
		t.Error("cached CreatedAt is zero, which reorders GetAll()")
	}
}

// The cache write is queued with middleware.OnCommit, so a registration whose
// transaction never commits must not leave the project behind. Calling the
// handler without the Transactional middleware runs no commit hooks, which is
// the same observation point as a rollback.
func TestRegisterDoesNotCacheBeforeCommit(t *testing.T) {
	setupSetupControllerDB(t)
	initRegistrationJWT(t)

	tx, err := db.DB.Begin()
	if err != nil {
		t.Fatalf("begin: %v", err)
	}

	body := `{"email":"uncommitted@example.com","name":"A","password":"password1","organizationName":"UncommittedOrg","timezone":"UTC","projectName":"Pending App","framework":"react"}`
	c, recorder := newControllerTestContext(t, tx, 0, http.MethodPost, "/api/register", body)
	AuthController.Register(c)
	if recorder.Code != http.StatusCreated {
		tx.Rollback()
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	if err := tx.Commit(); err != nil {
		t.Fatalf("commit: %v", err)
	}

	stored := findProjectByName(t, "Pending App")
	if stored == nil {
		t.Fatal("project was not created")
	}
	t.Cleanup(func() { cache.ProjectCache.RemoveProject(stored.Id) })

	if cache.ProjectCache.GetByToken(stored.Token) != nil {
		t.Error("the handler cached the project inline; a rolled back registration would leave a phantom entry")
	}
}
