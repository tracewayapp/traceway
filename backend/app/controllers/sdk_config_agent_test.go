//go:build !transactional_pg && !telemetry_ch && !telemetry_duckdb

package controllers

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/tracewayapp/traceway/backend/app/db"
	"github.com/tracewayapp/traceway/backend/app/models"
)

func TestSDKConfigAgentPermissionsAndLifecycle(t *testing.T) {
	f := setupAgentFixture(t)
	if _, err := db.DB.Exec("UPDATE projects SET framework = ? WHERE id = ?", "svelte", f.project.Id); err != nil {
		t.Fatal(err)
	}
	read, write := f.pat(t, models.ScopeRead), f.pat(t, models.ScopeWrite)
	_, readonly, _ := f.member(t, "sdk-reader@example.com", "readonly")
	query := "?projectId=" + f.project.Id.String()
	base := "/api/sdk-config/rules"
	rule := map[string]any{
		"name": "Full sessions", "enabled": true, "conditions": map[string]any{},
		"settings": map[string]any{"sessions": map[string]any{"enabled": true, "baselineSampleRate": 1}, "replay": map[string]any{"mode": "stream", "sessionSampleRate": 1, "maskAllInputs": true}},
	}
	for _, path := range []string{base, "/api/sdk-config/preview"} {
		method := http.MethodGet
		if path != base {
			method = http.MethodPost
		}
		if rec := f.do(t, read, method, path+query, map[string]any{}); rec.Code != http.StatusOK {
			t.Fatalf("read %s: %d %s", path, rec.Code, rec.Body.String())
		}
	}
	mutations := []struct{ method, path string }{
		{http.MethodPost, base}, {http.MethodPut, base + "/1"},
		{http.MethodPut, base + "/reorder"}, {http.MethodDelete, base + "/1"},
	}
	for _, m := range mutations {
		for _, token := range []string{read, write} {
			if rec := f.do(t, token, m.method, m.path+query, rule); rec.Code != http.StatusForbidden {
				t.Fatalf("write without scope/opt-in %s %s: %d", m.method, m.path, rec.Code)
			}
		}
	}
	f.setAgentWrites(t, true)
	for _, m := range mutations {
		for _, token := range []string{read, readonly} {
			if rec := f.do(t, token, m.method, m.path+query, rule); rec.Code != http.StatusForbidden {
				t.Fatalf("write without scope/role %s %s: %d", m.method, m.path, rec.Code)
			}
		}
		if rec := f.do(t, write, m.method, m.path+"?projectId="+f.otherProject.Id.String(), rule); rec.Code != http.StatusForbidden {
			t.Fatalf("foreign project %s: %d", m.path, rec.Code)
		}
	}
	created := f.do(t, write, http.MethodPost, base+query, rule)
	if created.Code != http.StatusCreated {
		t.Fatalf("create: %d %s", created.Code, created.Body.String())
	}
	var saved models.SdkConfigRule
	if err := json.Unmarshal(created.Body.Bytes(), &saved); err != nil {
		t.Fatal(err)
	}
	path := base + "/" + itoa(saved.Id) + query
	if rec := f.do(t, write, http.MethodPut, path, rule); rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("agent update without version: %d %s", rec.Code, rec.Body.String())
	}
	rule["expectedUpdatedAt"] = saved.UpdatedAt
	rule["enabled"] = false
	if rec := f.do(t, write, http.MethodPut, path, rule); rec.Code != http.StatusOK {
		t.Fatalf("update: %d %s", rec.Code, rec.Body.String())
	}
	if rec := f.do(t, write, http.MethodPut, path, rule); rec.Code != http.StatusConflict {
		t.Fatalf("stale update: %d %s", rec.Code, rec.Body.String())
	}
	preview := f.do(t, read, http.MethodPost, "/api/sdk-config/preview"+query, map[string]any{})
	var out struct {
		MatchedRules []any `json:"matchedRules"`
	}
	if err := json.Unmarshal(preview.Body.Bytes(), &out); err != nil || preview.Code != http.StatusOK || len(out.MatchedRules) != 0 {
		t.Fatalf("disabled rule still matches: %d %s (%v)", preview.Code, preview.Body.String(), err)
	}
	if rec := f.do(t, write, http.MethodPut, base+"/reorder"+query, map[string]any{"ids": []int{saved.Id}}); rec.Code != http.StatusOK {
		t.Fatalf("reorder: %d %s", rec.Code, rec.Body.String())
	}
	if rec := f.do(t, write, http.MethodDelete, path, nil); rec.Code != http.StatusOK {
		t.Fatalf("delete: %d %s", rec.Code, rec.Body.String())
	}
	accepted := map[string]bool{}
	for _, event := range f.auditEvents(t) {
		if event.StatusCode == http.StatusOK || event.StatusCode == http.StatusCreated {
			accepted[event.Method+" "+event.Route] = true
		}
	}
	for _, key := range []string{"POST " + base, "PUT " + base + "/:id", "PUT " + base + "/reorder", "DELETE " + base + "/:id"} {
		if !accepted[key] {
			t.Errorf("missing successful audit event: %s", key)
		}
	}
}
