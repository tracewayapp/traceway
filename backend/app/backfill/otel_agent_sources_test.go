package backfill

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/tracewayapp/traceway/backend/app/models"
	dashboardsvc "github.com/tracewayapp/traceway/backend/app/services/dashboards"
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
