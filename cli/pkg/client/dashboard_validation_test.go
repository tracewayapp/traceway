package client

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestValidateDashboardImport(t *testing.T) {
	valid := `{"schemaVersion":1,"name":"Worker","widgets":[{"id":"queue","title":"Queue","widgetType":"line_chart","config":{"sources":[{"type":"metric","name":"queue.depth","aggregation":"max"}]}}]}`
	for _, data := range []string{valid, `{"schemaVersion":1,"dashboards":[` + valid + `]}`, `{"name":"Empty","widgets":[]}`} {
		if err := ValidateDashboardImport(json.RawMessage(data), "create"); err != nil {
			t.Fatal(err)
		}
	}
	for _, data := range []string{
		`null`, `[]`, `{}`, `{"name":"Missing widgets"}`, `{"schemaVersion":1,"dashboards":[]}`,
		strings.Replace(valid, `"schemaVersion":1`, `"schemaVersion":2`, 1),
		strings.Replace(valid, `"name":"Worker"`, `"name":" "`, 1),
		strings.Replace(valid, `"title":"Queue"`, `"title":" "`, 1),
		strings.Replace(valid, `"line_chart"`, `"pie"`, 1),
		strings.Replace(valid, `"type":"metric"`, `"type":"sql"`, 1),
		strings.Replace(valid, `"name":"queue.depth"`, `"name":""`, 1),
		strings.Replace(valid, `"aggregation":"max"`, `"aggregation":"p95"`, 1),
		strings.Replace(valid, `"sources":[{"type":"metric","name":"queue.depth","aggregation":"max"}]`, `"sources":[]`, 1),
	} {
		if err := ValidateDashboardImport(json.RawMessage(data), "create"); err == nil {
			t.Errorf("accepted %s", data)
		}
	}
	if err := ValidateDashboardImport(json.RawMessage(valid), "replace"); err == nil {
		t.Fatal("accepted invalid mode")
	}
}

func TestValidateDashboardImportLimitsAndChartTypes(t *testing.T) {
	widget := map[string]any{"title": "Signal", "widgetType": "line_chart", "config": map[string]any{"sources": []any{map[string]any{"type": "metric", "name": "signal", "aggregation": "last"}}}}
	doc := map[string]any{"schemaVersion": 1, "name": "Signals", "widgets": []any{widget}}
	check := func(valid bool) {
		t.Helper()
		data, err := json.Marshal(doc)
		if err != nil {
			t.Fatal(err)
		}
		if err := ValidateDashboardImport(data, "upsert"); (err == nil) != valid {
			t.Fatalf("valid=%v: %v", valid, err)
		}
	}
	for _, chart := range []string{"line_chart", "area_chart", "stacked_area", "bar_chart", "single_value", "gauge", "table"} {
		widget["widgetType"] = chart
		check(true)
	}
	widgets := make([]any, 101)
	for i := range widgets {
		widgets[i] = widget
	}
	doc["widgets"] = widgets
	check(false)
	doc["widgets"] = widgets[:100]
	check(true)
	doc["widgets"] = []any{widget}
	doc["name"] = strings.Repeat("x", 101)
	check(false)
	doc["name"] = "Signals"
	widget["config"].(map[string]any)["extra"] = strings.Repeat("x", 32*1024)
	check(false)
}
