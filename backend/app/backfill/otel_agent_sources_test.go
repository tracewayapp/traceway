package backfill

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/tracewayapp/traceway/backend/v2/app/models"
	dashboardsvc "github.com/tracewayapp/traceway/backend/v2/app/services/dashboards"
)

const otelAgentTemplateFixture = `{"schemaVersion":1,"widgets":[
 {"title":"CPU Utilization","widgetType":"line_chart","config":{"sources":[{"type":"metric","name":"system.cpu.utilization","aggregation":"avg","tagFilters":{"state":"idle"},"groupBy":"server_name","label":"CPU","complement":true}]}},
 {"title":"Network I/O","widgetType":"line_chart","config":{"sources":[{"type":"metric","name":"system.network.io","aggregation":"rate","tagFilters":{"direction":"receive"},"groupBy":"server_name","label":"Receive"},{"type":"metric","name":"system.network.io","aggregation":"rate","tagFilters":{"direction":"transmit"},"groupBy":"server_name","label":"Transmit"}]}}
]}`

func TestUpgradeOtelAgentDashboard(t *testing.T) {
	templateSources, err := templateSourcesByMetric(models.JSONText(otelAgentTemplateFixture))
	if err != nil {
		t.Fatalf("templateSourcesByMetric: %v", err)
	}
	if len(templateSources) != 2 {
		t.Fatalf("expected 2 template metrics, got %d", len(templateSources))
	}

	installed := `{"schemaVersion":1,"widgets":[
	 {"id":"w_cpu","title":"CPU Utilization","widgetType":"line_chart","config":{"sources":[{"type":"metric","name":"system.cpu.utilization","aggregation":"avg"}],"showLegend":true}},
	 {"id":"w_net","title":"Network I/O","widgetType":"line_chart","config":{"sources":[{"type":"metric","name":"system.network.io","aggregation":"avg","groupBy":"device"}]}},
	 {"id":"w_custom","title":"Requests","widgetType":"line_chart","config":{"sources":[{"type":"metric","name":"http.requests","aggregation":"sum"}]}},
	 {"id":"w_text","title":"Notes","widgetType":"text","config":{"text":"hello"}}
	]}`
	dashboard := &models.Dashboard{Id: 1, Definition: models.JSONText(installed)}

	changed, err := upgradeOtelAgentDashboard(dashboard, templateSources)
	if err != nil {
		t.Fatalf("upgradeOtelAgentDashboard: %v", err)
	}
	if !changed {
		t.Fatal("expected the dashboard to change")
	}

	def, err := dashboardsvc.ParseDefinition([]byte(dashboard.Definition))
	if err != nil {
		t.Fatalf("ParseDefinition: %v", err)
	}
	configs := map[string]map[string]json.RawMessage{}
	for _, widget := range def.Widgets {
		var cfg map[string]json.RawMessage
		if err := json.Unmarshal(widget.Config, &cfg); err != nil {
			t.Fatalf("widget %s config: %v", widget.Id, err)
		}
		configs[widget.Id] = cfg
	}

	if !strings.Contains(string(configs["w_cpu"]["sources"]), `"complement":true`) {
		t.Errorf("CPU widget should carry the template's inverted idle source, got %s", configs["w_cpu"]["sources"])
	}
	if string(configs["w_cpu"]["showLegend"]) != "true" {
		t.Errorf("CPU widget lost its other config: %s", configs["w_cpu"]["showLegend"])
	}
	if !strings.Contains(string(configs["w_net"]["sources"]), `"groupBy":"device"`) {
		t.Errorf("edited network widget should be untouched, got %s", configs["w_net"]["sources"])
	}
	if !strings.Contains(string(configs["w_custom"]["sources"]), `"aggregation":"sum"`) {
		t.Errorf("custom widget should be untouched, got %s", configs["w_custom"]["sources"])
	}
	if string(configs["w_text"]["text"]) != `"hello"` {
		t.Errorf("text widget should be untouched, got %s", configs["w_text"]["text"])
	}

	changedAgain, err := upgradeOtelAgentDashboard(dashboard, templateSources)
	if err != nil {
		t.Fatalf("second upgrade: %v", err)
	}
	if changedAgain {
		t.Error("second upgrade should be a no-op")
	}
}

func TestOriginalSource(t *testing.T) {
	cases := []struct {
		sources string
		name    string
		ok      bool
	}{
		{`[{"type":"metric","name":"system.memory.usage","aggregation":"avg"}]`, "system.memory.usage", true},
		{`[{"type":"metric","name":"system.memory.usage"}]`, "system.memory.usage", true},
		{`[{"type":"metric","name":"system.memory.usage","aggregation":"max"}]`, "", false},
		{`[{"type":"metric","name":"system.memory.usage","aggregation":"avg","tagFilters":{"state":"used"}}]`, "", false},
		{`[{"type":"metric","name":"a","aggregation":"avg"},{"type":"metric","name":"b","aggregation":"avg"}]`, "", false},
		{`[]`, "", false},
	}
	for _, c := range cases {
		name, ok := originalSource(json.RawMessage(c.sources))
		if ok != c.ok || name != c.name {
			t.Errorf("originalSource(%s) = (%q, %v), want (%q, %v)", c.sources, name, ok, c.name, c.ok)
		}
	}
}

func TestFixOtelAgentFilesystemSource(t *testing.T) {
	current := templateDefinition(t, "0096_update_template_traceway_otel_agent_filesystem.up.sql")
	currentRaw, err := dashboardsvc.MarshalDefinition(current)
	if err != nil {
		t.Fatal(err)
	}
	templateSources, err := templateSourcesByMetric(models.JSONText(currentRaw))
	if err != nil {
		t.Fatalf("templateSourcesByMetric: %v", err)
	}
	if strings.Contains(string(templateSources["system.filesystem.utilization"]), `"state"`) {
		t.Fatalf("the current template still filters filesystem utilization on state: %s", templateSources["system.filesystem.utilization"])
	}

	dashboard := installed(t, templateDefinition(t, "0076_update_template_traceway_otel_agent_sources.up.sql"))
	before := widgetsByTitle(t, dashboard)

	changed, err := fixOtelAgentFilesystemSource(dashboard, templateSources)
	if err != nil {
		t.Fatalf("fixOtelAgentFilesystemSource: %v", err)
	}
	if !changed {
		t.Fatal("an unedited copy of the previous template should change")
	}
	after := widgetsByTitle(t, dashboard)
	for title, widget := range before {
		got := after[title]
		if got.Id != widget.Id {
			t.Errorf("%s lost its widget id: %s -> %s", title, widget.Id, got.Id)
		}
		if title == "Filesystem Utilization" {
			if !strings.Contains(string(got.Config), `"tagFilters":{"mode":"rw"}`) || strings.Contains(string(got.Config), `"state"`) {
				t.Errorf("filesystem utilization should take the template's source, got %s", got.Config)
			}
			continue
		}
		if string(got.Config) != string(widget.Config) {
			t.Errorf("%s should be untouched:\n got %s\nwant %s", title, got.Config, widget.Config)
		}
	}

	again, err := fixOtelAgentFilesystemSource(dashboard, templateSources)
	if err != nil || again {
		t.Errorf("second run should be a no-op, got changed=%v err=%v", again, err)
	}
}

func TestFixOtelAgentFilesystemSourceMatchesJSONBAndSkipsEdits(t *testing.T) {
	templateSources := map[string]json.RawMessage{
		"system.filesystem.utilization": json.RawMessage(`[{"type":"metric","name":"system.filesystem.utilization","aggregation":"max","tagFilters":{"mode":"rw"},"groupBy":"server_name","label":"Used"}]`),
	}
	installed := `{"schemaVersion":1,"widgets":[
	 {"id":"w_jsonb","title":"Filesystem Utilization","widgetType":"line_chart","config":{"sources": [{"name": "system.filesystem.utilization", "type": "metric", "label": "Used", "groupBy": "server_name", "tagFilters": {"state": "used"}, "aggregation": "max"}]}},
	 {"id":"w_edited","title":"Root disk","widgetType":"line_chart","config":{"sources":[{"type":"metric","name":"system.filesystem.utilization","aggregation":"max","tagFilters":{"state":"used"},"groupBy":"mountpoint","label":"Used"}]}},
	 {"id":"w_usage","title":"Filesystem Usage","widgetType":"area_chart","config":{"sources":[{"type":"metric","name":"system.filesystem.usage","aggregation":"max","tagFilters":{"state":"used"},"groupBy":"server_name","label":"Used"}]}}
	]}`
	dashboard := &models.Dashboard{Id: 1, Definition: models.JSONText(installed)}

	changed, err := fixOtelAgentFilesystemSource(dashboard, templateSources)
	if err != nil || !changed {
		t.Fatalf("expected the JSONB-ordered copy to change, got changed=%v err=%v", changed, err)
	}
	def, err := dashboardsvc.ParseDefinition([]byte(dashboard.Definition))
	if err != nil {
		t.Fatal(err)
	}
	configs := map[string]string{}
	for _, widget := range def.Widgets {
		configs[widget.Id] = string(widget.Config)
	}
	if !strings.Contains(configs["w_jsonb"], `"mode":"rw"`) {
		t.Errorf("JSONB-ordered widget should be fixed, got %s", configs["w_jsonb"])
	}
	if !strings.Contains(configs["w_edited"], `"groupBy":"mountpoint"`) || !strings.Contains(configs["w_edited"], `"state":"used"`) {
		t.Errorf("edited widget should be untouched, got %s", configs["w_edited"])
	}
	if !strings.Contains(configs["w_usage"], `"state":"used"`) {
		t.Errorf("filesystem usage keeps its state filter, got %s", configs["w_usage"])
	}
}
