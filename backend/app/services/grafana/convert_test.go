package grafana

import (
	"encoding/json"
	"strings"
	"testing"
)

const nodeExporterFixture = `{
  "title": "Node Overview",
  "description": "Host health",
  "panels": [
    {
      "type": "row",
      "title": "Network",
      "panels": [
        {
          "type": "timeseries",
          "title": "Receive Throughput",
          "gridPos": {"h": 8, "w": 12},
          "fieldConfig": {"defaults": {"unit": "bytes"}},
          "targets": [
            {
              "expr": "sum by (instance) (rate(node_network_receive_bytes_total{device!=\"lo\",job=\"node\"}[5m]))",
              "datasource": {"type": "prometheus"}
            }
          ]
        }
      ]
    },
    {
      "type": "stat",
      "title": "Load 1m",
      "gridPos": {"h": 4, "w": 6},
      "targets": [
        {"expr": "node_load1{instance=\"web-1\"}"}
      ]
    },
    {
      "type": "gauge",
      "title": "Memory Available",
      "gridPos": {"h": 8, "w": 18},
      "fieldConfig": {"defaults": {"unit": "percent"}},
      "targets": [
        {"expr": "avg(node_memory_MemAvailable_percent)"},
        {"expr": "node_hidden_metric", "hide": true}
      ]
    },
    {
      "type": "table",
      "title": "Recent Logs",
      "targets": [
        {"expr": "{app=\"web\"}", "datasource": {"type": "loki"}}
      ]
    },
    {
      "type": "text",
      "title": "Notes"
    }
  ]
}`

type sourceShape struct {
	Type        string            `json:"type"`
	Name        string            `json:"name"`
	Aggregation string            `json:"aggregation"`
	TagFilters  map[string]string `json:"tagFilters"`
	GroupBy     string            `json:"groupBy"`
}

type configShape struct {
	Sources []sourceShape `json:"sources"`
	Unit    string        `json:"unit"`
	ColSpan int           `json:"colSpan"`
	Size    string        `json:"size"`
}

func parseConfig(t *testing.T, raw json.RawMessage) configShape {
	t.Helper()
	var cfg configShape
	if err := json.Unmarshal(raw, &cfg); err != nil {
		t.Fatalf("config does not parse: %v", err)
	}
	return cfg
}

func hasWarningContaining(warnings []string, substr string) bool {
	for _, w := range warnings {
		if strings.Contains(w, substr) {
			return true
		}
	}
	return false
}

func TestConvertNodeExporterDashboard(t *testing.T) {
	result, err := Convert([]byte(nodeExporterFixture))
	if err != nil {
		t.Fatalf("Convert: %v", err)
	}

	if result.Name != "Node Overview" || result.Description != "Host health" {
		t.Errorf("unexpected name/description: %q / %q", result.Name, result.Description)
	}

	if len(result.Definition.Widgets) != 3 {
		names := []string{}
		for _, w := range result.Definition.Widgets {
			names = append(names, w.Title)
		}
		t.Fatalf("expected 3 widgets, got %d: %v", len(result.Definition.Widgets), names)
	}

	network := result.Definition.Widgets[0]
	if network.Title != "Receive Throughput" || network.WidgetType != "line_chart" {
		t.Errorf("unexpected first widget: %+v", network)
	}
	cfg := parseConfig(t, network.Config)
	if len(cfg.Sources) != 1 {
		t.Fatalf("expected 1 source, got %d", len(cfg.Sources))
	}
	src := cfg.Sources[0]
	if src.Name != "node_network_receive_bytes_total" {
		t.Errorf("metric name = %q", src.Name)
	}
	if src.Aggregation != "sum" {
		t.Errorf("aggregation = %q", src.Aggregation)
	}
	if src.GroupBy != "instance" {
		t.Errorf("groupBy = %q", src.GroupBy)
	}
	if src.TagFilters["job"] != "node" {
		t.Errorf("tagFilters = %v", src.TagFilters)
	}
	if _, hasDevice := src.TagFilters["device"]; hasDevice {
		t.Errorf("negative matcher should be dropped, got %v", src.TagFilters)
	}
	if cfg.Unit != "By" {
		t.Errorf("unit = %q", cfg.Unit)
	}
	if cfg.ColSpan != 2 || cfg.Size != "md" {
		t.Errorf("layout = colSpan %d size %q", cfg.ColSpan, cfg.Size)
	}

	stat := result.Definition.Widgets[1]
	if stat.WidgetType != "single_value" {
		t.Errorf("stat widget type = %q", stat.WidgetType)
	}
	statCfg := parseConfig(t, stat.Config)
	if statCfg.Sources[0].Name != "node_load1" || statCfg.Sources[0].Aggregation != "last" {
		t.Errorf("stat source = %+v", statCfg.Sources[0])
	}
	if statCfg.Sources[0].TagFilters["instance"] != "web-1" {
		t.Errorf("stat tagFilters = %v", statCfg.Sources[0].TagFilters)
	}
	if statCfg.ColSpan != 1 || statCfg.Size != "sm" {
		t.Errorf("stat layout = colSpan %d size %q", statCfg.ColSpan, statCfg.Size)
	}

	gauge := result.Definition.Widgets[2]
	if gauge.WidgetType != "gauge" {
		t.Errorf("gauge widget type = %q", gauge.WidgetType)
	}
	gaugeCfg := parseConfig(t, gauge.Config)
	if len(gaugeCfg.Sources) != 1 {
		t.Fatalf("hidden target should be skipped, got %d sources", len(gaugeCfg.Sources))
	}
	if gaugeCfg.Sources[0].Name != "node_memory_MemAvailable_percent" || gaugeCfg.Sources[0].Aggregation != "last" {
		t.Errorf("gauge source = %+v", gaugeCfg.Sources[0])
	}
	if gaugeCfg.Unit != "%" || gaugeCfg.ColSpan != 3 {
		t.Errorf("gauge unit/colSpan = %q / %d", gaugeCfg.Unit, gaugeCfg.ColSpan)
	}

	seen := map[string]bool{}
	for _, w := range result.Definition.Widgets {
		if w.Id == "" || seen[w.Id] {
			t.Errorf("widget %q has missing or duplicate id %q", w.Title, w.Id)
		}
		seen[w.Id] = true
	}

	if !hasWarningContaining(result.Warnings, "rate") {
		t.Errorf("expected a rate() warning, got %v", result.Warnings)
	}
	if !hasWarningContaining(result.Warnings, "Prometheus") {
		t.Errorf("expected a non-Prometheus warning, got %v", result.Warnings)
	}
	if !hasWarningContaining(result.Warnings, "unsupported panel type") {
		t.Errorf("expected an unsupported panel warning, got %v", result.Warnings)
	}
	if !hasWarningContaining(result.Warnings, "only exact matches") {
		t.Errorf("expected a dropped matcher warning, got %v", result.Warnings)
	}
}

func TestConvertApiWrappedDashboard(t *testing.T) {
	wrapped := `{"dashboard": {"title": "Wrapped", "panels": [{"type": "timeseries", "title": "CPU", "targets": [{"expr": "node_cpu_seconds_total"}]}]}}`
	result, err := Convert([]byte(wrapped))
	if err != nil {
		t.Fatalf("Convert: %v", err)
	}
	if result.Name != "Wrapped" || len(result.Definition.Widgets) != 1 {
		t.Errorf("unexpected result: name %q, %d widgets", result.Name, len(result.Definition.Widgets))
	}
}

func TestConvertRejectsInvalidJSON(t *testing.T) {
	if _, err := Convert([]byte("not json")); err == nil {
		t.Fatal("expected an error for invalid JSON")
	}
}
