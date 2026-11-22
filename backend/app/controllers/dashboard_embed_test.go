//go:build !transactional_pg && !telemetry_ch && !telemetry_duckdb

package controllers

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"strings"
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
	t.Run("share expiry and viewer-relative presets", func(t *testing.T) {
		issueURL := issue + f.project.Id.String() + "&timeframe=7d"
		deadline := time.Now().UTC().Add(72 * time.Hour).Truncate(time.Second)
		for _, invalid := range []any{
			map[string]any{"expiresAt": time.Now().Add(-time.Hour)},
			map[string]any{"expiresInDays": 0},
			map[string]any{"expiresInDays": -1},
			map[string]any{"expiresInDays": 99999999},
			map[string]any{"expiresInDays": embedMaxDays + 1},
			map[string]any{"expiresAt": time.Now().AddDate(0, 0, embedMaxDays).Add(time.Hour)},
			map[string]any{"expiresAt": deadline, "expiresInDays": 3},
		} {
			if rec := f.do(t, writer, http.MethodPost, issueURL, invalid); rec.Code != http.StatusUnprocessableEntity {
				t.Errorf("invalid expiry %v: status %d", invalid, rec.Code)
			}
		}
		rec := f.do(t, writer, http.MethodPost, issueURL, map[string]any{"expiresInDays": embedMaxDays})
		var longest struct{ ExpiresAt time.Time }
		if err := json.Unmarshal(rec.Body.Bytes(), &longest); err != nil || rec.Code != http.StatusCreated || longest.ExpiresAt.Sub(time.Now().AddDate(0, 0, embedMaxDays)).Abs() > time.Minute {
			t.Fatalf("longest share: status %d, expires %v", rec.Code, longest.ExpiresAt)
		}
		for _, opts := range []map[string]any{{"expiresAt": deadline}, {"expiresInDays": 3}} {
			opts["tagFilters"] = map[string]string{"account": "one"}
			rec := f.do(t, writer, http.MethodPost, issueURL, opts)
			var issued struct {
				DashboardUUID, Token string
				ExpiresAt            time.Time
			}
			if err := json.Unmarshal(rec.Body.Bytes(), &issued); err != nil || rec.Code != http.StatusCreated || issued.ExpiresAt.Sub(deadline).Abs() > time.Second {
				t.Fatalf("three-day share: status %d, expires %v", rec.Code, issued.ExpiresAt)
			}
			var previous time.Time
			for range 2 {
				opened := time.Now()
				rec = f.do(t, issued.Token, http.MethodGet, "/api/embedded-dashboards/"+issued.DashboardUUID, nil)
				var meta struct {
					From, To, ExpiresAt time.Time
					WidgetToken         string
				}
				if err := json.Unmarshal(rec.Body.Bytes(), &meta); err != nil || rec.Code != http.StatusOK || meta.To.Before(opened) || !meta.To.After(previous) || meta.To.Sub(meta.From) != 7*24*time.Hour {
					t.Fatalf("opened share: status %d, range %v to %v", rec.Code, meta.From, meta.To)
				}
				previous = meta.To
				widget, err := services.ValidateDashboardEmbed(meta.WidgetToken, issued.DashboardUUID)
				if err != nil || widget.Rolling || !widget.From.Equal(meta.From) || !widget.To.Equal(meta.To) || !widget.ExpiresAt.Equal(issued.ExpiresAt) || !meta.ExpiresAt.Equal(issued.ExpiresAt) || widget.TagFilters["account"] != "one" || widget.ProjectId != f.project.Id {
					t.Fatal("widget token did not preserve the resolved view and original expiry")
				}
			}
		}
	})
	fixed := time.Now().UTC().Truncate(time.Second).Add(-7 * 24 * time.Hour)
	fixedView := map[string]any{"from": fixed.Add(-time.Hour), "to": fixed}
	for _, tc := range []struct {
		query  string
		body   any
		window time.Duration
	}{
		{"", nil, 42 * time.Hour},
		{"&timeframe=30m", view, 30 * time.Minute},
		{"&timeframe=last42h", view, 42 * time.Hour},
		{"&timeframe=7d", view, 7 * 24 * time.Hour},
		{"&timeframe=731d", nil, metricQueryMaxRange},
		{"", fixedView, time.Hour},
		{"", map[string]any{"from": fixed.Add(-time.Hour), "to": fixed, "expiresInDays": 7}, time.Hour},
		{"&timeframe=7d", fixedView, 0},
		{"&timeframe=1h&timeframe=2h", nil, 0},
		{"&timeframe=", nil, 0},
		{"&timeframe=0m", nil, 0},
		{"&timeframe=-1h", nil, 0},
		{"&timeframe=week", nil, 0},
		{"&timeframe=732d", nil, 0},
		{"&timeframe=999999999999999999999d", nil, 0},
	} {
		before := time.Now()
		rec := f.do(t, writer, http.MethodPost, issue+f.project.Id.String()+tc.query, tc.body)
		if tc.window == 0 {
			if rec.Code != http.StatusUnprocessableEntity {
				t.Errorf("invalid timeframe %q: status %d", tc.query, rec.Code)
			}
			continue
		}
		var issued struct{ DashboardUUID, Token string }
		if err := json.Unmarshal(rec.Body.Bytes(), &issued); err != nil || rec.Code != http.StatusCreated {
			t.Fatalf("issue timeframe %q: status %d, err %v", tc.query, rec.Code, err)
		}
		rec = f.do(t, issued.Token, http.MethodGet, "/api/embedded-dashboards/"+issued.DashboardUUID, nil)
		var actual services.DashboardEmbedView
		if err := json.Unmarshal(rec.Body.Bytes(), &actual); rec.Code != http.StatusOK || err != nil || actual.To.Sub(actual.From) != tc.window {
			t.Fatalf("read timeframe %q: status %d, range %v to %v", tc.query, rec.Code, actual.From, actual.To)
		}
		if tc.body != nil && tc.query == "" {
			if !actual.To.Equal(fixed) {
				t.Errorf("fixed end = %v, want %v", actual.To, fixed)
			}
		} else if actual.To.Before(before) || actual.To.After(time.Now()) {
			t.Errorf("relative timeframe ended outside issuance: %v", actual.To)
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
	rec = f.do(t, writer, http.MethodPost, issue+f.project.Id.String()+"&timeframe=1h", view)
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

	// The viewer is anonymous: what a widget queries stays on the server.
	meta := f.do(t, link.Token, http.MethodGet, path, nil).Body.String()
	for _, hidden := range []string{"tagFilters", `"paid"`, `"name":"orders"`} {
		if strings.Contains(meta, hidden) {
			t.Fatalf("shared dashboard exposes %s: %s", hidden, meta)
		}
	}
	if !strings.Contains(meta, `"aggregation":"sum"`) || !strings.Contains(meta, `"title":"Paid orders"`) {
		t.Fatalf("shared dashboard lost what the page draws with: %s", meta)
	}

	now := time.Now().Add(-time.Minute)
	point := models.MetricPoint{ProjectId: f.project.Id, Name: "orders", Value: 7, Tags: map[string]string{"state": "paid", "account": "one"}, RecordedAt: now}
	older := point
	older.RecordedAt = now.Add(-2 * time.Hour)
	points := []models.MetricPoint{point, older}
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

func TestEmbedWidgetsKeepOnlyWhatThePageDraws(t *testing.T) {
	config := `{"unit":"ms","thresholds":[{"value":1,"color":"red"}],"sources":[{"type":"metric","name":"checkout.duration","aggregation":"p95","groupBy":"region","label":"Checkout","complement":true,"threshold":250,"tagFilters":{"account":"two"}}]}`
	shown, err := embedWidgets([]models.DashboardWidget{
		{Id: "w_1", Title: "Checkout", WidgetType: "line_chart", Config: json.RawMessage(config)},
		{Id: "w_2", Title: "Empty", WidgetType: "line_chart", Config: json.RawMessage(`{}`)},
	})
	if err != nil {
		t.Fatal(err)
	}
	want := `{"sources":[{"aggregation":"p95","groupBy":"region","label":"Checkout","complement":true}],"thresholds":[{"value":1,"color":"red"}],"unit":"ms"}`
	if string(shown[0].Config) != want || shown[0].Title != "Checkout" || string(shown[1].Config) != `{}` {
		t.Fatalf("shown widgets: %s, %s", shown[0].Config, shown[1].Config)
	}
	if _, err := embedWidgets([]models.DashboardWidget{{Id: "w_3", Config: json.RawMessage(`{"sources":"checkout.duration"}`)}}); err == nil {
		t.Fatal("unreadable sources must not be passed through")
	}
}

func TestEmbeddedDashboardReadsAreRateLimited(t *testing.T) {
	f := setupAgentFixture(t)
	for _, tc := range []struct {
		path  string
		limit int
	}{
		{"/api/embedded-dashboards/" + uuid.NewString(), embedOpensPerMinute},
		{"/api/embedded-dashboards/" + uuid.NewString() + "/widgets/w_1", embedWidgetReadsPerMinute},
	} {
		for range tc.limit {
			if rec := f.do(t, "", http.MethodGet, tc.path, nil); rec.Code != http.StatusUnauthorized {
				t.Fatalf("GET %s within the limit: %d", tc.path, rec.Code)
			}
		}
		rec := f.do(t, "", http.MethodGet, tc.path, nil)
		if rec.Code != http.StatusTooManyRequests || rec.Header().Get("Retry-After") == "" {
			t.Fatalf("GET %s past the limit: %d", tc.path, rec.Code)
		}
	}
}
