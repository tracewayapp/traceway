//go:build !transactional_pg && !telemetry_ch && !telemetry_duckdb

package controllers

import (
	"database/sql"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/tracewayapp/traceway/backend/v2/app/db"
	"github.com/tracewayapp/traceway/backend/v2/app/dbtest"
	"github.com/tracewayapp/traceway/backend/v2/app/middleware"
	"github.com/tracewayapp/traceway/backend/v2/app/models"
	"github.com/tracewayapp/traceway/backend/v2/app/repositories/transactional"
)

func newSavedFilterFixture(t *testing.T) (*sdkConfigFixture, uuid.UUID) {
	t.Helper()
	dbtest.SetupSQLite(t)
	gin.SetMode(gin.TestMode)
	orgId := createTestOrg(t, "filters-org")
	type seeded struct {
		project, other *models.Project
		user           *models.User
	}
	s, err := db.ExecuteTransaction(func(tx *sql.Tx) (seeded, error) {
		project, err := transactional.ProjectRepository.CreateWithOrganization(tx, "plant", "opentelemetry", orgId)
		if err != nil {
			return seeded{}, err
		}
		other, err := transactional.ProjectRepository.CreateWithOrganization(tx, "other", "opentelemetry", orgId)
		if err != nil {
			return seeded{}, err
		}
		user, err := transactional.UserRepository.Create(tx, "ops@example.com", "Ops Engineer", "x")
		return seeded{project: project, other: other, user: user}, err
	})
	if err != nil {
		t.Fatalf("seed: %v", err)
	}
	f := &sdkConfigFixture{t: t, router: gin.New(), projectId: s.project.Id}
	scope := func(c *gin.Context) {
		projectId := f.projectId
		if raw := c.Query("projectId"); raw != "" {
			projectId = uuid.MustParse(raw)
		}
		c.Set(middleware.ProjectIdContextKey, projectId)
		c.Set(middleware.UserIdContextKey, s.user.Id)
	}
	api := f.router.Group("/api", scope, middleware.Transactional)
	api.GET("/saved-filters", SavedFilterController.List)
	api.POST("/saved-filters", SavedFilterController.Create)
	api.PUT("/saved-filters/:id", SavedFilterController.Update)
	api.DELETE("/saved-filters/:id", SavedFilterController.Delete)
	return f, s.other.Id
}

func TestSavedFiltersLifecycle(t *testing.T) {
	f, otherProject := newSavedFilterFixture(t)

	query := url.Values{
		"projectId": {f.projectId.String()},
		"preset":    {"24h"},
		"attr":      {"log.NodeId=7", "resource.service.name!=simulator"},
		"chart":     {"log.TimerSend"},
		"unknown":   {"dropped"},
	}.Encode()
	code, created := f.do("POST", "/api/saved-filters", `{"page":"logs","name":"  Node 7 timers ","query":`+strconv.Quote(query)+`}`)
	if code != http.StatusCreated {
		t.Fatalf("create = %d %v", code, created)
	}
	if created["name"] != "Node 7 timers" {
		t.Errorf("name = %q, want it trimmed", created["name"])
	}
	stored := created["query"].(string)
	if strings.Contains(stored, "projectId") || strings.Contains(stored, "unknown") {
		t.Errorf("stored query %q keeps params the page does not own", stored)
	}
	if want := "attr=log.NodeId%3D7&attr=resource.service.name%21%3Dsimulator&chart=log.TimerSend&preset=24h"; stored != want {
		t.Errorf("stored query = %q, want %q (sorted keys, attr order kept)", stored, want)
	}
	id := int(created["id"].(float64))

	for name, body := range map[string]string{
		"duplicate name, other case": `{"page":"logs","name":"node 7 TIMERS","query":"preset=1h"}`,
		"no name":                    `{"page":"logs","name":"  ","query":"preset=1h"}`,
		"long name":                  `{"page":"logs","name":"` + strings.Repeat("n", 101) + `","query":"preset=1h"}`,
		"unknown page":               `{"page":"spans","name":"x","query":"preset=1h"}`,
		"nothing to save":            `{"page":"logs","name":"x","query":"projectId=abc&other=1"}`,
		"bad query":                  `{"page":"logs","name":"x","query":"attr=%zz"}`,
		"too long":                   `{"page":"logs","name":"x","query":"search=` + strings.Repeat("a", 9000) + `"}`,
	} {
		if code, out := f.do("POST", "/api/saved-filters", body); code != http.StatusUnprocessableEntity {
			t.Errorf("%s: got %d %v, want 422", name, code, out)
		}
	}

	if code, out := f.do("POST", "/api/saved-filters", `{"page":"logs","name":"Errors","query":"?minSeverity=17"}`); code != http.StatusCreated {
		t.Fatalf("second create = %d %v", code, out)
	}

	code, listed := f.do("GET", "/api/saved-filters?page=logs", "")
	if code != http.StatusOK {
		t.Fatalf("list = %d %v", code, listed)
	}
	items := listed["data"].([]any)
	if len(items) != 2 || items[0].(map[string]any)["name"] != "Errors" || items[1].(map[string]any)["name"] != "Node 7 timers" {
		t.Fatalf("list = %v, want both filters by name", items)
	}
	if items[1].(map[string]any)["createdByName"] != "Ops Engineer" {
		t.Errorf("createdByName = %v, want the author", items[1].(map[string]any)["createdByName"])
	}
	if code, out := f.do("GET", "/api/saved-filters?page=nope", ""); code != http.StatusBadRequest {
		t.Errorf("list of an unknown page = %d %v, want 400", code, out)
	}
	if code, out := f.do("GET", "/api/saved-filters?page=logs&projectId="+otherProject.String(), ""); code != http.StatusOK || len(out["data"].([]any)) != 0 {
		t.Errorf("another project sees %v, want nothing", out)
	}

	path := "/api/saved-filters/" + strconv.Itoa(id)
	if code, out := f.do("PUT", path, `{"name":"errors"}`); code != http.StatusUnprocessableEntity {
		t.Errorf("rename onto another filter's name = %d %v, want 422", code, out)
	}
	code, updated := f.do("PUT", path, `{"name":"Node 7 slow timers","query":"preset=6h&attr=log.NodeId%3D7"}`)
	if code != http.StatusOK || updated["name"] != "Node 7 slow timers" || updated["query"] != "attr=log.NodeId%3D7&preset=6h" {
		t.Fatalf("update = %d %v", code, updated)
	}
	if code, out := f.do("PUT", path, `{"name":"node 7 SLOW timers"}`); code != http.StatusOK {
		t.Errorf("changing only the case of its own name = %d %v, want 200", code, out)
	}
	if code, out := f.do("PUT", path+"?projectId="+otherProject.String(), `{"name":"stolen"}`); code != http.StatusNotFound {
		t.Errorf("update from another project = %d %v, want 404", code, out)
	}
	if code, out := f.do("DELETE", path+"?projectId="+otherProject.String(), ""); code != http.StatusNotFound {
		t.Errorf("delete from another project = %d %v, want 404", code, out)
	}
	if code, out := f.do("DELETE", path, ""); code != http.StatusOK {
		t.Fatalf("delete = %d %v", code, out)
	}
	if code, out := f.do("DELETE", path, ""); code != http.StatusNotFound {
		t.Errorf("second delete = %d %v, want 404", code, out)
	}
	if code, out := f.do("GET", "/api/saved-filters?page=logs", ""); code != http.StatusOK || len(out["data"].([]any)) != 1 {
		t.Errorf("after delete the list = %v, want one filter", out)
	}
}
