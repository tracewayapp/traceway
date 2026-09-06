//go:build !telemetry_ch && !transactional_pg && !telemetry_duckdb

package controllers

import (
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/tracewayapp/traceway/backend/app/cache"
	"github.com/tracewayapp/traceway/backend/app/db"
	"github.com/tracewayapp/traceway/backend/app/middleware"
	"github.com/tracewayapp/traceway/backend/app/models"
	"github.com/tracewayapp/traceway/backend/app/repositories/transactional"
)

// ProjectCache is a process-global that outlives the per-test in-memory
// database, so whatever a test puts in it has to come back out even when the
// test fails before it knows the project's id.
func restoreProjectCacheAfterTest(t *testing.T) {
	t.Helper()
	before := map[uuid.UUID]bool{}
	for _, p := range cache.ProjectCache.GetAll() {
		before[p.Id] = true
	}
	t.Cleanup(func() {
		for _, p := range cache.ProjectCache.GetAll() {
			if !before[p.Id] {
				cache.ProjectCache.RemoveProject(p.Id)
			}
		}
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

// Every field the cache serves has to agree with the row, not just the ones a
// past bug happened to drop.
func assertCachedProjectMatchesRow(t *testing.T, cached, stored *models.Project) {
	t.Helper()

	if cached.Id != stored.Id {
		t.Errorf("cached Id = %v, stored = %v", cached.Id, stored.Id)
	}
	if cached.Name != stored.Name {
		t.Errorf("cached Name = %q, stored = %q", cached.Name, stored.Name)
	}
	if cached.Token != stored.Token {
		t.Errorf("cached Token = %q, stored = %q", cached.Token, stored.Token)
	}
	if cached.Framework != stored.Framework {
		t.Errorf("cached Framework = %q, stored = %q", cached.Framework, stored.Framework)
	}
	if !equalIntPtr(cached.OrganizationId, stored.OrganizationId) {
		t.Errorf("cached OrganizationId = %v, stored = %v", cached.OrganizationId, stored.OrganizationId)
	}
	// Second granularity: the cached copy carries the value the repository
	// built, the row carries whatever survived the round trip through SQLite.
	if cached.CreatedAt.Unix() != stored.CreatedAt.Unix() {
		t.Errorf("cached CreatedAt = %v, stored = %v", cached.CreatedAt, stored.CreatedAt)
	}
	if cached.CreatedAt.IsZero() {
		t.Error("cached CreatedAt is zero, which reorders GetAll()")
	}
	if cached.DropHealthyHealthchecks != stored.DropHealthyHealthchecks {
		t.Errorf("cached DropHealthyHealthchecks = %v, stored = %v; healthy healthchecks would be ingested",
			cached.DropHealthyHealthchecks, stored.DropHealthyHealthchecks)
	}
	if !equalStringPtr(cached.SourceMapToken, stored.SourceMapToken) {
		t.Errorf("cached SourceMapToken = %v, stored = %v", cached.SourceMapToken, stored.SourceMapToken)
	}
	// nil and empty are the same allowlist; copyProject turns one into the other.
	if !slices.Equal(cached.HealthcheckPaths, stored.HealthcheckPaths) {
		t.Errorf("cached HealthcheckPaths = %v, stored = %v", cached.HealthcheckPaths, stored.HealthcheckPaths)
	}
	if !slices.Equal(cached.ProfileLabelAllowlist, stored.ProfileLabelAllowlist) {
		t.Errorf("cached ProfileLabelAllowlist = %v, stored = %v", cached.ProfileLabelAllowlist, stored.ProfileLabelAllowlist)
	}
	if !slices.Equal(cached.AiFlaggedTerms, stored.AiFlaggedTerms) {
		t.Errorf("cached AiFlaggedTerms = %v, stored = %v", cached.AiFlaggedTerms, stored.AiFlaggedTerms)
	}
	if !slices.Equal(cached.AiFlaggedLanguages, stored.AiFlaggedLanguages) {
		t.Errorf("cached AiFlaggedLanguages = %v, stored = %v; ingest would scan the wrong term packs",
			cached.AiFlaggedLanguages, stored.AiFlaggedLanguages)
	}
}

func equalIntPtr(a, b *int) bool {
	if a == nil || b == nil {
		return a == b
	}
	return *a == *b
}

func equalStringPtr(a, b *string) bool {
	if a == nil || b == nil {
		return a == b
	}
	return *a == *b
}

// The register handler used to cache a five-field copy of the project it had
// just created, so the cached ingest settings disagreed with the row. On the
// SQLite builds nothing ever refreshes the cache, so the wrong copy served
// every /api/report request until the process restarted.
func TestRegisterCachesTheProjectItCreated(t *testing.T) {
	setupSetupControllerDB(t)
	initRegistrationJWT(t)
	restoreProjectCacheAfterTest(t)

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

	cached := cache.ProjectCache.GetByToken(stored.Token)
	if cached == nil {
		t.Fatal("project token is not in the cache after a committed registration")
	}
	assertCachedProjectMatchesRow(t, cached, stored)

	// The by-source-map-token index is separate from the by-token one, and it
	// is what a symbol upload authenticates against.
	if stored.SourceMapToken == nil {
		t.Fatal("an ios project should be created with a source map token")
	}
	if cache.ProjectCache.GetBySourceMapToken(*stored.SourceMapToken) == nil {
		t.Error("source map token does not resolve from the cache; symbol upload would 401")
	}
}

// The handler itself must not touch the cache; the write is queued with
// middleware.OnCommit so a registration whose transaction rolls back leaves
// nothing behind. Driving the handler without that middleware runs no commit
// hooks, which is the same observation point as a rollback.
func TestRegisterCachesOnlyViaCommitHook(t *testing.T) {
	setupSetupControllerDB(t)
	initRegistrationJWT(t)
	restoreProjectCacheAfterTest(t)

	tx, err := db.DB.Begin()
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	defer tx.Rollback()

	body := `{"email":"uncommitted@example.com","name":"A","password":"password1","organizationName":"UncommittedOrg","timezone":"UTC","projectName":"Pending App","framework":"react"}`
	c, recorder := newControllerTestContext(t, tx, 0, http.MethodPost, "/api/register", body)
	AuthController.Register(c)
	if recorder.Code != http.StatusCreated {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	// Commit rather than roll back: the harness caps the main DB at one
	// connection, so findProjectByName cannot open its transaction until this
	// one closes, and it needs the row to be visible. Commit hooks still never
	// run, because middleware.Transactional is what runs them.
	if err := tx.Commit(); err != nil {
		t.Fatalf("commit: %v", err)
	}

	stored := findProjectByName(t, "Pending App")
	if stored == nil {
		t.Fatal("project was not created")
	}

	if cache.ProjectCache.GetByToken(stored.Token) != nil {
		t.Error("the handler cached the project inline; a rolled back registration would leave a phantom entry")
	}
}
