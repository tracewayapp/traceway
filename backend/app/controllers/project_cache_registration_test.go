//go:build !telemetry_ch && !transactional_pg && !telemetry_duckdb

package controllers

import (
	"database/sql"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/tracewayapp/traceway/backend/v2/app/cache"
	"github.com/tracewayapp/traceway/backend/v2/app/db"
	"github.com/tracewayapp/traceway/backend/v2/app/middleware"
	"github.com/tracewayapp/traceway/backend/v2/app/models"
	"github.com/tracewayapp/traceway/backend/v2/app/repositories/transactional"
)

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

func TestRegistrationCachesProjectAfterCommit(t *testing.T) {
	for _, tc := range []struct {
		path    string
		body    string
		handler gin.HandlerFunc
		sso     bool
	}{
		{
			path:    "/register",
			body:    `{"email":"cached@example.com","name":"A","password":"password1","organizationName":"CachedOrg","timezone":"UTC","projectName":"Mobile App","framework":"ios"}`,
			handler: AuthController.Register,
		},
		{
			path:    "/auth/finish-setup",
			body:    `{"organizationName":"CachedOrg","timezone":"UTC","projectName":"Mobile App","framework":"ios"}`,
			handler: OAuthController.FinishSetup,
			sso:     true,
		},
	} {
		t.Run(strings.TrimPrefix(tc.path, "/"), func(t *testing.T) {
			setupSetupControllerDB(t)
			initRegistrationJWT(t)

			gin.SetMode(gin.TestMode)
			router := gin.New()
			if tc.sso {
				user, err := db.ExecuteTransaction(func(tx *sql.Tx) (*models.User, error) {
					return transactional.UserRepository.CreateOAuth(tx, "cached@example.com", "A", "oidc", "test-user", "")
				})
				if err != nil {
					t.Fatal(err)
				}
				router.Use(func(c *gin.Context) { c.Set(middleware.UserIdContextKey, user.Id) })
			}

			var created *models.Project
			router.POST(tc.path, middleware.Transactional, func(c *gin.Context) {
				tc.handler(c)
				if c.Writer.Status() != http.StatusCreated {
					return
				}
				projects, err := transactional.ProjectRepository.FindAll(db.GetTx(c))
				if err != nil || len(projects) != 1 {
					t.Fatalf("expected one project in the registration transaction, got %d (err %v)", len(projects), err)
				}
				created = projects[0]
				t.Cleanup(func() { cache.ProjectCache.RemoveProject(created.Id) })
				if cache.ProjectCache.GetByToken(created.Token) != nil {
					t.Error("project token was visible before commit")
				}
			})

			recorder := httptest.NewRecorder()
			request := httptest.NewRequest(http.MethodPost, tc.path, strings.NewReader(tc.body))
			request.Header.Set("Content-Type", "application/json")
			router.ServeHTTP(recorder, request)
			if recorder.Code != http.StatusCreated {
				t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
			}
			cached := cache.ProjectCache.GetByToken(created.Token)
			if cached == nil {
				t.Fatal("project token missing after commit")
			}
			assertCachedProjectMatchesRow(t, cached, created)
			if created.SourceMapToken == nil {
				t.Fatal("iOS project missing symbol upload token")
			}
			if p := cache.ProjectCache.GetBySourceMapToken(*created.SourceMapToken); p == nil || p.Id != created.Id {
				t.Error("symbol upload token did not resolve to the registered project")
			}
		})
	}
}
