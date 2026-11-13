//go:build !transactional_pg && !telemetry_ch && !telemetry_duckdb

package controllers

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/tracewayapp/traceway/backend/v2/app/models"
)

func TestDashboardAgentImport(t *testing.T) {
	f := setupAgentFixture(t)
	read, write := f.pat(t, models.ScopeRead), f.pat(t, models.ScopeWrite)
	_, readonly, _ := f.member(t, "dashboard-reader@example.com", "readonly")
	path := "/api/dashboards/import?projectId=" + f.project.Id.String()
	doc := map[string]any{"schemaVersion": 1, "name": "Worker health", "widgets": []any{map[string]any{"id": "queue", "title": "Queue depth", "widgetType": "line_chart", "config": map[string]any{"sources": []any{map[string]any{"type": "metric", "name": "queue.depth", "aggregation": "max"}}}}}}
	body := map[string]any{"data": doc, "mode": "create"}
	for _, token := range []string{read, write} {
		if rec := f.do(t, token, http.MethodPost, path, body); rec.Code != http.StatusForbidden {
			t.Fatalf("scope/opt-in: %d %s", rec.Code, rec.Body.String())
		}
	}
	f.setAgentWrites(t, true)
	for _, token := range []string{read, readonly} {
		if rec := f.do(t, token, http.MethodPost, path, body); rec.Code != http.StatusForbidden {
			t.Fatalf("scope/role: %d %s", rec.Code, rec.Body.String())
		}
	}
	for key, value := range map[string]any{"organizationId": *f.otherProject.OrganizationId, "applyToProjectIds": []string{f.otherProject.Id.String()}} {
		body[key] = value
		if rec := f.do(t, write, http.MethodPost, path, body); rec.Code != http.StatusUnprocessableEntity {
			t.Fatalf("override %s: %d %s", key, rec.Code, rec.Body.String())
		}
		delete(body, key)
	}
	rec := f.do(t, write, http.MethodPost, path, body)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create: %d %s", rec.Code, rec.Body.String())
	}
	var result struct {
		Created    int                 `json:"created"`
		Updated    int                 `json:"updated"`
		Dashboards []DashboardListItem `json:"dashboards"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &result); err != nil || result.Created != 1 || len(result.Dashboards) != 1 {
		t.Fatalf("create result: %s (%v)", rec.Body.String(), err)
	}
	id := result.Dashboards[0].Id
	body["mode"] = "upsert"
	doc["description"] = "Updated description"
	rec = f.do(t, write, http.MethodPost, path, body)
	if err := json.Unmarshal(rec.Body.Bytes(), &result); err != nil || rec.Code != http.StatusCreated || result.Updated != 1 || result.Dashboards[0].Id != id {
		t.Fatalf("upsert: %d %s (%v)", rec.Code, rec.Body.String(), err)
	}
	exported := f.do(t, read, http.MethodGet, "/api/dashboards/"+itoa(id)+"/export?projectId="+f.project.Id.String(), nil)
	var saved models.DashboardExport
	if err := json.Unmarshal(exported.Body.Bytes(), &saved); err != nil || exported.Code != http.StatusOK || saved.Description != "Updated description" || len(saved.Widgets) != 1 || saved.Widgets[0].Id != "queue" {
		t.Fatalf("export: %d %s (%v)", exported.Code, exported.Body.String(), err)
	}
	found := false
	for _, event := range f.auditEvents(t) {
		if event.Route == "/api/dashboards/import" && event.StatusCode == http.StatusCreated {
			found = true
		}
	}
	if !found {
		t.Fatal("successful import missing from audit log")
	}
}
