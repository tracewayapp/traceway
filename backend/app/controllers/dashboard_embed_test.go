//go:build !transactional_pg && !telemetry_ch && !telemetry_duckdb

package controllers

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/tracewayapp/traceway/backend/v2/app/db"
	"github.com/tracewayapp/traceway/backend/v2/app/models"
	"github.com/tracewayapp/traceway/backend/v2/app/repositories/telemetry"
	"github.com/tracewayapp/traceway/backend/v2/app/repositories/transactional"
)

func TestEmbeddedDashboard(t *testing.T) {
	f := setupAgentFixture(t)
	f.setAgentWrites(t, true)
	writer := f.pat(t, models.ScopeWrite)
	doc := json.RawMessage(`{"schemaVersion":1,"name":"Orders","widgets":[{"id":"orders","title":"Paid orders","widgetType":"single_value","config":{"sources":[{"name":"orders","aggregation":"sum","tagFilters":{"state":"paid"}}]}}]}`)
	rec := f.do(t, writer, http.MethodPost, "/api/dashboards/import?projectId="+f.project.Id.String(), map[string]any{"data": doc})
	var imported struct{ Dashboards []DashboardListItem }
	if err := json.Unmarshal(rec.Body.Bytes(), &imported); rec.Code != 201 || err != nil || len(imported.Dashboards) != 1 {
		t.Fatalf("import: %d %s", rec.Code, rec.Body)
	}
	id := imported.Dashboards[0].Id
	issue := "/api/dashboards/" + itoa(id) + "/embed-token?projectId="
	view := map[string]any{"tagFilters": map[string]string{"account": "one"}}
	if rec := f.do(t, writer, http.MethodPost, issue+f.otherProject.Id.String(), view); rec.Code != 403 {
		t.Fatalf("foreign project issuance: %d", rec.Code)
	}
	if rec := f.do(t, f.pat(t, models.ScopeRead), http.MethodPost, issue+f.project.Id.String(), view); rec.Code != 403 {
		t.Fatalf("read-only issuance: %d", rec.Code)
	}
	if rec := f.do(t, writer, http.MethodPost, issue+f.project.Id.String(), map[string]any{"filters": map[string]string{"account": "one"}}); rec.Code != 400 {
		t.Fatalf("unknown filter accepted: %d", rec.Code)
	}
	rec = f.do(t, writer, http.MethodPost, issue+f.project.Id.String(), view)
	var link struct {
		DashboardUUID uuid.UUID
		Token         string
		ExpiresAt     time.Time
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &link); rec.Code != 201 || err != nil || link.Token == "" || link.DashboardUUID == uuid.Nil || time.Until(link.ExpiresAt) > 5*time.Minute || time.Until(link.ExpiresAt) < 4*time.Minute || rec.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("bad issuance response: %d (%v)", rec.Code, err)
	}

	path := "/api/embedded-dashboards/" + link.DashboardUUID.String()
	for _, tc := range []struct {
		token, path string
		status      int
	}{
		{link.Token, path, 200}, {writer, path, 401}, {"", path, 401}, {link.Token, "/api/projects", 401},
		{link.Token, "/api/embedded-dashboards/" + uuid.NewString(), 401}, {link.Token, path + "/widgets/missing", 404},
	} {
		if rec := f.do(t, tc.token, http.MethodGet, tc.path, nil); rec.Code != tc.status {
			t.Fatalf("GET %s: %d, want %d", tc.path, rec.Code, tc.status)
		}
	}

	now := time.Now().Add(-time.Minute)
	point := models.MetricPoint{ProjectId: f.project.Id, Name: "orders", Value: 7, Tags: map[string]string{"state": "paid", "account": "one"}, RecordedAt: now}
	points := []models.MetricPoint{point}
	for _, tags := range []map[string]string{{"state": "paid", "account": "two"}, {"state": "pending", "account": "one"}} {
		other := point
		other.Tags, other.Value = tags, 123
		points = append(points, other)
	}
	point.ProjectId = f.otherProject.Id
	points = append(points, point)
	if err := telemetry.MetricPointRepository.InsertAsync(t.Context(), points); err != nil {
		t.Fatal(err)
	}
	rec = f.do(t, link.Token, http.MethodGet, path+"/widgets/orders?projectId="+f.otherProject.Id.String()+"&account=two", nil)
	var result MetricQueryResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &result); rec.Code != 200 || err != nil || len(result.Results) != 1 {
		t.Fatalf("widget response: %d %s", rec.Code, rec.Body)
	}
	series := result.Results[0].Series
	if len(series) != 1 || len(series["__all__"]) != 1 || series["__all__"][0].Value != 7 {
		t.Fatalf("unexpected filtered results: %+v", series)
	}
	if _, err := db.ExecuteTransaction(func(tx *sql.Tx) (int, error) {
		return 0, transactional.DashboardRepository.DeleteAssignment(tx, f.project.Id, id)
	}); err != nil {
		t.Fatal(err)
	}
	for _, suffix := range []string{"", "/widgets/orders"} {
		if rec := f.do(t, link.Token, http.MethodGet, path+suffix, nil); rec.Code != 404 {
			t.Fatalf("unassigned dashboard: %d", rec.Code)
		}
	}
}
