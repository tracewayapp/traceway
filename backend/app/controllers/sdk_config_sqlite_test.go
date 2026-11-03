//go:build !transactional_pg && !telemetry_ch && !telemetry_duckdb

package controllers

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/tracewayapp/traceway/backend/app/cache"
	"github.com/tracewayapp/traceway/backend/app/db"
	"github.com/tracewayapp/traceway/backend/app/dbtest"
	"github.com/tracewayapp/traceway/backend/app/middleware"
	"github.com/tracewayapp/traceway/backend/app/models"
	"github.com/tracewayapp/traceway/backend/app/repositories/transactional"
)

type sdkConfigFixture struct {
	t         *testing.T
	router    *gin.Engine
	projectId uuid.UUID
}

func newSdkConfigFixture(t *testing.T) *sdkConfigFixture {
	t.Helper()
	dbtest.SetupSQLite(t)
	gin.SetMode(gin.TestMode)
	orgId := createTestOrg(t, "sdk-org")
	type seeded struct {
		project *models.Project
		user    *models.User
	}
	s, err := db.ExecuteTransaction(func(tx *sql.Tx) (seeded, error) {
		project, err := transactional.ProjectRepository.CreateWithOrganization(tx, "web", "react", orgId)
		if err != nil {
			return seeded{}, err
		}
		user, err := transactional.UserRepository.Create(tx, "dev@example.com", "Dev", "x")
		return seeded{project: project, user: user}, err
	})
	if err != nil {
		t.Fatalf("seed: %v", err)
	}
	f := &sdkConfigFixture{t: t, router: gin.New(), projectId: s.project.Id}
	scope := func(c *gin.Context) {
		c.Set(middleware.ProjectIdContextKey, f.projectId)
		c.Set(middleware.UserIdContextKey, s.user.Id)
	}
	api := f.router.Group("/api", scope, middleware.Transactional)
	api.GET("/sdk-config/rules", SdkConfigController.ListRules)
	api.POST("/sdk-config/rules", SdkConfigController.CreateRule)
	api.PUT("/sdk-config/rules/reorder", SdkConfigController.ReorderRules)
	api.PUT("/sdk-config/rules/:id", SdkConfigController.UpdateRule)
	api.DELETE("/sdk-config/rules/:id", SdkConfigController.DeleteRule)
	api.POST("/sdk-config/preview", SdkConfigController.Preview)
	api.GET("/sdk-config/captures", SdkConfigController.ListCaptures)
	api.POST("/sdk-config/captures", SdkConfigController.CreateCapture)
	api.DELETE("/sdk-config/captures/:id", SdkConfigController.CancelCapture)
	return f
}

func (f *sdkConfigFixture) do(method, path, body string) (int, map[string]any) {
	f.t.Helper()
	recorder := httptest.NewRecorder()
	req := httptest.NewRequest(method, path, bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	f.router.ServeHTTP(recorder, req)
	var out map[string]any
	_ = json.Unmarshal(recorder.Body.Bytes(), &out)
	return recorder.Code, out
}

func TestSdkConfigRulesLifecycleAndPreview(t *testing.T) {
	f := newSdkConfigFixture(t)

	code, created := f.do("POST", "/api/sdk-config/rules", `{"name":"Quiet console","settings":{"signals":{"console":{"mode":"off"}}}}`)
	if code != http.StatusCreated {
		t.Fatalf("create = %d %v", code, created)
	}
	firstId := int(created["id"].(float64))
	code, created = f.do("POST", "/api/sdk-config/rules", `{"name":"Replay old release","conditions":{"all":[{"key":"service.version","op":"version","value":"<2"}]},"settings":{"replay":{"mode":"stream"}}}`)
	if code != http.StatusCreated {
		t.Fatalf("create second = %d %v", code, created)
	}
	secondId := int(created["id"].(float64))

	for name, body := range map[string]string{
		"bad op":       `{"name":"x","conditions":{"all":[{"key":"a","op":"like","value":"b"}]}}`,
		"bad settings": `{"name":"x","settings":{"replay":{"mode":"always"}}}`,
		"no name":      `{"name":" ","settings":{}}`,
	} {
		if code, out := f.do("POST", "/api/sdk-config/rules", body); code != http.StatusUnprocessableEntity {
			t.Errorf("%s: got %d %v, want 422", name, code, out)
		}
	}

	code, preview := f.do("POST", "/api/sdk-config/preview", `{"attributes":{"service.version":"1.4.0"}}`)
	if code != http.StatusOK {
		t.Fatalf("preview = %d %v", code, preview)
	}
	if len(preview["matchedRules"].([]any)) != 2 {
		t.Fatalf("preview matched %v", preview["matchedRules"])
	}
	settings := preview["settings"].(map[string]any)
	if settings["replay"].(map[string]any)["mode"] != "stream" || preview["configHash"] == "" {
		t.Fatalf("unexpected preview settings: %v", preview)
	}

	if code, out := f.do("PUT", "/api/sdk-config/rules/reorder", `{"ids":[`+itoa(secondId)+`]}`); code != http.StatusUnprocessableEntity {
		t.Fatalf("partial reorder = %d %v, want 422", code, out)
	}
	if code, out := f.do("PUT", "/api/sdk-config/rules/reorder", `{"ids":[`+itoa(secondId)+`,`+itoa(firstId)+`]}`); code != http.StatusOK {
		t.Fatalf("reorder = %d %v", code, out)
	}
	code, list := f.do("GET", "/api/sdk-config/rules", "")
	rules := list["data"].([]any)
	if code != http.StatusOK || int(rules[0].(map[string]any)["id"].(float64)) != secondId {
		t.Fatalf("list after reorder = %d %v", code, list)
	}

	if code, out := f.do("PUT", "/api/sdk-config/rules/"+itoa(firstId), `{"name":"Quiet console","enabled":false,"settings":{"signals":{"console":{"mode":"off"}}}}`); code != http.StatusOK {
		t.Fatalf("update = %d %v", code, out)
	}
	_, preview = f.do("POST", "/api/sdk-config/preview", `{"attributes":{"service.version":"3.0.0"}}`)
	if len(preview["matchedRules"].([]any)) != 0 {
		t.Fatalf("disabled rule still applied: %v", preview)
	}
	if code, _ := f.do("DELETE", "/api/sdk-config/rules/"+itoa(firstId), ""); code != http.StatusOK {
		t.Fatalf("delete = %d", code)
	}
	if code, _ := f.do("DELETE", "/api/sdk-config/rules/"+itoa(firstId), ""); code != http.StatusNotFound {
		t.Fatalf("second delete = %d, want 404", code)
	}

	entry, err := cache.SdkConfigCache.Get(f.projectId)
	if err != nil || len(entry.Compiled.Rules) != 1 {
		t.Fatalf("cache sees %+v (%v), want the one remaining rule", entry, err)
	}
}

func TestSdkCapturesTargetOneClient(t *testing.T) {
	f := newSdkConfigFixture(t)
	instance := "0195f2a0-0000-7000-8000-0000000000aa"

	code, capture := f.do("POST", "/api/sdk-config/captures", `{"target":{"kind":"instance","value":"`+instance+`"},"durationMinutes":30}`)
	if code != http.StatusCreated || capture["active"] != true {
		t.Fatalf("create capture = %d %v", code, capture)
	}
	if code, out := f.do("POST", "/api/sdk-config/captures", `{"target":{"kind":"planet","value":"x"},"durationMinutes":30}`); code != http.StatusUnprocessableEntity {
		t.Fatalf("bad target = %d %v", code, out)
	}

	_, preview := f.do("POST", "/api/sdk-config/preview", `{"instanceUid":"`+instance+`"}`)
	captures := preview["settings"].(map[string]any)["captures"].([]any)
	if len(captures) != 1 || captures[0].(map[string]any)["id"] != capture["id"] {
		t.Fatalf("capture not delivered to its client: %v", preview)
	}
	_, other := f.do("POST", "/api/sdk-config/preview", `{"instanceUid":"0195f2a0-0000-7000-8000-0000000000bb"}`)
	if _, ok := other["settings"].(map[string]any)["captures"]; ok {
		t.Fatalf("capture leaked to another client: %v", other)
	}

	if code, _ := f.do("DELETE", "/api/sdk-config/captures/"+capture["id"].(string), ""); code != http.StatusOK {
		t.Fatalf("cancel = %d", code)
	}
	_, list := f.do("GET", "/api/sdk-config/captures", "")
	rows := list["data"].([]any)
	if len(rows) != 1 || rows[0].(map[string]any)["status"] != "Cancelled" || rows[0].(map[string]any)["active"] != false {
		t.Fatalf("cancelled capture history incorrect: %v", list)
	}
}

func TestSdkCapturesDoNotLoseActiveRowsBehindExpiredHistory(t *testing.T) {
	f := newSdkConfigFixture(t)
	now := time.Now().UTC()
	_, err := db.ExecuteTransaction(func(tx *sql.Tx) (struct{}, error) {
		for i := 0; i < 230; i++ {
			expires := now.Add(-time.Minute)
			if i < 20 {
				expires = now.Add(time.Hour)
			}
			capture := &models.SdkCapture{Id: uuid.New(), ProjectId: f.projectId, Label: "test", Conditions: models.JSONText(`{}`), Actions: models.JSONText(`{}`), ExpiresAt: expires, CreatedAt: now.Add(time.Duration(i-230) * time.Minute)}
			if err := transactional.SdkConfigRepository.CreateCapture(tx, capture); err != nil {
				return struct{}{}, err
			}
		}
		return struct{}{}, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	cache.SdkConfigCache.Invalidate(f.projectId)
	entry, err := cache.SdkConfigCache.Get(f.projectId)
	if err != nil {
		t.Fatal(err)
	}
	if len(entry.Compiled.Captures) != 20 {
		t.Fatalf("active captures = %d, want 20", len(entry.Compiled.Captures))
	}
	_, preview := f.do("POST", "/api/sdk-config/preview", `{}`)
	if got := len(preview["matchedCaptures"].([]any)); got != 20 {
		t.Fatalf("preview captures = %d", got)
	}
	_, list := f.do("GET", "/api/sdk-config/captures", "")
	active := 0
	for _, row := range list["data"].([]any) {
		if row.(map[string]any)["active"] == true {
			active++
		}
	}
	if active != 20 {
		t.Fatalf("listed active captures = %d", active)
	}
	code, _ := f.do("POST", "/api/sdk-config/captures", `{"target":{"kind":"user","value":"u"},"durationMinutes":30}`)
	if code != http.StatusUnprocessableEntity {
		t.Fatalf("capture limit = %d, want 422", code)
	}
}

func TestSdkConfigRejectsNonBrowserProjects(t *testing.T) {
	f := newSdkConfigFixture(t)
	for _, framework := range []string{"go", "nestjs", "nextjs", "react-native", "flutter", "ios", "android"} {
		t.Run(framework, func(t *testing.T) {
			if _, err := db.DB.Exec("UPDATE projects SET framework = ? WHERE id = ?", framework, f.projectId); err != nil {
				t.Fatal(err)
			}
			for _, path := range []string{"/api/sdk-config/rules", "/api/sdk-config/captures"} {
				code, body := f.do("GET", path, "")
				if code != http.StatusForbidden {
					t.Fatalf("%s: %d %v", path, code, body)
				}
			}
		})
	}
}
