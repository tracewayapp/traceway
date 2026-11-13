package client

import (
	"encoding/json"
	"fmt"
	"slices"
	"strings"
	"unicode/utf8"
)

// ValidateDashboardImport checks portable dashboard documents before a write.
// Config extensions are preserved; the server remains authoritative for access and persistence.
func ValidateDashboardImport(data json.RawMessage, mode string) error {
	if mode != "create" && mode != "upsert" {
		return fmt.Errorf("mode must be create or upsert")
	}
	var root map[string]json.RawMessage
	if err := json.Unmarshal(data, &root); err != nil || root == nil {
		return fmt.Errorf("dashboard data must be a JSON object")
	}
	if len(data) > 5<<20 {
		return fmt.Errorf("dashboard data exceeds 5 MiB")
	}
	docs := []json.RawMessage{data}
	if bundle, ok := root["dashboards"]; ok {
		var version int
		if err := json.Unmarshal(root["schemaVersion"], &version); err != nil || version != 1 {
			return fmt.Errorf("bundle schemaVersion must be 1")
		}
		if err := json.Unmarshal(bundle, &docs); err != nil || len(docs) == 0 {
			return fmt.Errorf("dashboards must be a nonempty array")
		}
	}
	for i, raw := range docs {
		if err := validateDashboard(raw); err != nil {
			return fmt.Errorf("dashboard %d: %w", i+1, err)
		}
	}
	return nil
}

func validateDashboard(raw json.RawMessage) error {
	var doc struct {
		SchemaVersion int    `json:"schemaVersion"`
		Name          string `json:"name"`
		Description   string `json:"description"`
		Widgets       []struct {
			ID         string          `json:"id"`
			Title      string          `json:"title"`
			WidgetType string          `json:"widgetType"`
			Config     json.RawMessage `json:"config"`
		} `json:"widgets"`
	}
	if err := json.Unmarshal(raw, &doc); err != nil {
		return fmt.Errorf("invalid document: %w", err)
	}
	if doc.SchemaVersion != 0 && doc.SchemaVersion != 1 {
		return fmt.Errorf("unsupported schemaVersion %d", doc.SchemaVersion)
	}
	if strings.TrimSpace(doc.Name) == "" || utf8.RuneCountInString(strings.TrimSpace(doc.Name)) > 100 {
		return fmt.Errorf("name must contain 1 to 100 characters")
	}
	if doc.Widgets == nil || len(doc.Widgets) > 100 {
		return fmt.Errorf("widgets must be an array of at most 100 widgets (use [] for an empty dashboard)")
	}
	for i, w := range doc.Widgets {
		if strings.TrimSpace(w.Title) == "" {
			return fmt.Errorf("widget %d needs a title", i+1)
		}
		if !slices.Contains([]string{"line_chart", "area_chart", "stacked_area", "bar_chart", "single_value", "gauge", "table"}, w.WidgetType) {
			return fmt.Errorf("widget %d has unsupported widgetType %q", i+1, w.WidgetType)
		}
		if len(w.Config) > 32*1024 {
			return fmt.Errorf("widget %d config exceeds 32 KiB", i+1)
		}
		var cfg struct {
			Sources []struct {
				Type        string            `json:"type"`
				Name        string            `json:"name"`
				Aggregation string            `json:"aggregation"`
				TagFilters  map[string]string `json:"tagFilters"`
				GroupBy     string            `json:"groupBy"`
				Complement  bool              `json:"complement"`
			} `json:"sources"`
		}
		if err := json.Unmarshal(w.Config, &cfg); err != nil || len(cfg.Sources) == 0 {
			return fmt.Errorf("widget %d needs config.sources with at least one metric", i+1)
		}
		for j, s := range cfg.Sources {
			if s.Type != "metric" || strings.TrimSpace(s.Name) == "" {
				return fmt.Errorf("widget %d source %d needs type metric and a name", i+1, j+1)
			}
			if s.Aggregation != "" && !slices.Contains(MetricAggregationsExact, s.Aggregation) {
				return fmt.Errorf("widget %d source %d: unsupported aggregation %q; use avg, sum, count, min, max, rate or last (metric percentiles are unavailable)", i+1, j+1, s.Aggregation)
			}
		}
	}
	return nil
}
