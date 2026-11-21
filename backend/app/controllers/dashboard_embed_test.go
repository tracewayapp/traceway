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
	"github.com/tracewayapp/traceway/backend/v2/app/services"
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
	for _, tc := range []struct {
		body any
		want int
	}{
		{nil, http.StatusCreated},
		{map[string]any{}, http.StatusCreated},
		{json.RawMessage("null"), http.StatusBadRequest},
	} {
		if got := f.do(t, writer, http.MethodPost, issue+f.project.Id.String(), tc.body).Code; got != tc.want {
			t.Errorf("embed options %v: status %d, want %d", tc.body, got, tc.want)
		}
	}
	for _, theme := range []string{"", "light", "dark"} {
		path := issue + f.project.Id.String()
		if theme != "" {
			path += "&theme=" + theme
		}
		rec := f.do(t, writer, http.MethodPost, path, view)
		var link struct{ DashboardUUID, EmbedPath, Token string }
		if err := json.Unmarshal(rec.Body.Bytes(), &link); rec.Code != http.StatusCreated || err != nil || link.Token == "" {
			t.Fatalf("issue dashboard with theme %q: status %d, err %v", theme, rec.Code, err)
		}
		want := "/embed/dashboard?dashboard=" + link.DashboardUUID
		if theme != "" {
			want += "&theme=" + theme
		}
		if link.EmbedPath != want+"#token="+link.Token {
			t.Errorf("incorrect iframe URL for theme %q", theme)
		}
	}
	if rec := f.do(t, writer, http.MethodPost, issue+f.project.Id.String()+"&theme=sepia", view); rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("invalid theme: status %d", rec.Code)
	}
	for name, invalid := range map[string]map[string]any{
		"half a range":       {"from": time.Now().Add(-time.Hour)},
		"reversed range":     {"from": time.Now(), "to": time.Now().Add(-time.Hour)},
		"empty filter value": {"tagFilters": map[string]string{"account": ""}},
		"empty filter tag":   {"tagFilters": map[string]string{"": "one"}},
	} {
		if rec := f.do(t, writer, http.MethodPost, issue+f.project.Id.String(), invalid); rec.Code != 422 {
			t.Fatalf("%s: %d %s", name, rec.Code, rec.Body)
		}
	}
	rec = f.do(t, writer, http.MethodPost, issue+f.project.Id.String(), view)
	var link struct {
		DashboardUUID    uuid.UUID
		Token            string
		ExpiresAt        time.Time
		ExpiresInSeconds int
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &link); rec.Code != 201 || err != nil || link.Token == "" || link.DashboardUUID == uuid.Nil || time.Until(link.ExpiresAt) > 5*time.Minute || time.Until(link.ExpiresAt) < 4*time.Minute || link.ExpiresInSeconds > 300 || link.ExpiresInSeconds < 240 || rec.Header().Get("Cache-Control") != "no-store" {
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

func TestEmbedWidgetQueryMatchesTheDashboardPage(t *testing.T) {
	view := services.DashboardEmbedView{TagFilters: map[string]string{"account": "one"}}
	for _, tc := range []struct {
		widgetType, sources string
		wholeRange          bool
		aggregations        []string
	}{
		{"line_chart", `[{"name":"a","aggregation":"p95"},{"name":"b"}]`, false, []string{"p95", ""}},
		{"single_value", `[{"name":"a","aggregation":"p95"}]`, true, []string{"p95"}},
		{"gauge", `[{"name":"a","aggregation":"avg"}]`, false, []string{"avg"}},
		{"rating_bar", `[{"name":"a","aggregation":"count","groupBy":"rating"}]`, true, []string{"count"}},
		{"breakdown_table", `[{"name":"a","aggregation":"p75","groupBy":"url.path"},{"name":"b"}]`, true, []string{"p75", "count"}},
		{"breakdown_table", `[{"name":"a","aggregation":"sum","groupBy":"url.path"}]`, true, []string{"sum"}},
	} {
		widget := models.DashboardWidget{WidgetType: tc.widgetType, Config: json.RawMessage(`{"sources":` + tc.sources + `}`)}
		req, err := embedWidgetQuery(widget, view)
		if err != nil || req.WholeRange != tc.wholeRange || len(req.Queries) != len(tc.aggregations) {
			t.Fatalf("%s %s: %+v (%v)", tc.widgetType, tc.sources, req, err)
		}
		for i, q := range req.Queries {
			if q.Aggregation != tc.aggregations[i] || q.TagFilters["account"] != "one" {
				t.Fatalf("%s query %d: %+v", tc.widgetType, i, q)
			}
		}
	}
	saved := models.DashboardWidget{WidgetType: "line_chart", Config: json.RawMessage(`{"sources":[{"name":"a","tagFilters":{"account":"two","state":"paid"}}]}`)}
	req, err := embedWidgetQuery(saved, view)
	if err != nil || req.Queries[0].TagFilters["account"] != "one" || req.Queries[0].TagFilters["state"] != "paid" {
		t.Fatalf("signed filter must win over a saved one: %+v (%v)", req, err)
	}
}
