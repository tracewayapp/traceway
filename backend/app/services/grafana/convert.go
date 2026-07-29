package grafana

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"github.com/tracewayapp/traceway/backend/app/models"
	dashboardsvc "github.com/tracewayapp/traceway/backend/app/services/dashboards"
)

type ConvertResult struct {
	Name        string
	Description string
	Definition  *models.DashboardDefinition
	Warnings    []string
}

type grafanaDashboard struct {
	Dashboard   *grafanaDashboard `json:"dashboard"`
	Title       string            `json:"title"`
	Description string            `json:"description"`
	Panels      []grafanaPanel    `json:"panels"`
}

type grafanaPanel struct {
	Type        string          `json:"type"`
	Title       string          `json:"title"`
	Description string          `json:"description"`
	GridPos     grafanaGridPos  `json:"gridPos"`
	FieldConfig json.RawMessage `json:"fieldConfig"`
	Targets     []grafanaTarget `json:"targets"`
	Panels      []grafanaPanel  `json:"panels"`
}

type grafanaGridPos struct {
	H int `json:"h"`
	W int `json:"w"`
}

type grafanaTarget struct {
	Expr         string          `json:"expr"`
	Hide         bool            `json:"hide"`
	LegendFormat string          `json:"legendFormat"`
	Datasource   json.RawMessage `json:"datasource"`
}

var panelTypeMap = map[string]string{
	"timeseries":  "line_chart",
	"graph":       "line_chart",
	"stat":        "single_value",
	"singlestat":  "single_value",
	"gauge":       "gauge",
	"bargauge":    "gauge",
	"barchart":    "bar_chart",
	"bar chart":   "bar_chart",
	"table":       "table",
	"table-old":   "table",
	"piechart":    "bar_chart",
	"state-timeline": "line_chart",
}

var unitMap = map[string]string{
	"percent":     "%",
	"percentunit": "%",
	"bytes":       "By",
	"decbytes":    "By",
	"deckbytes":   "KiBy",
	"decmbytes":   "MiBy",
	"decgbytes":   "GiBy",
	"seconds":     "s",
	"s":           "s",
	"ms":          "ms",
	"ns":          "ns",
	"ops":         "ops",
	"reqps":       "req/s",
	"short":       "",
	"none":        "",
}

func Convert(raw []byte) (*ConvertResult, error) {
	var doc grafanaDashboard
	if err := json.Unmarshal(raw, &doc); err != nil {
		return nil, fmt.Errorf("invalid Grafana dashboard JSON: %w", err)
	}
	root := &doc
	if doc.Dashboard != nil {
		root = doc.Dashboard
	}

	result := &ConvertResult{
		Name:        strings.TrimSpace(root.Title),
		Description: strings.TrimSpace(root.Description),
		Definition:  &models.DashboardDefinition{SchemaVersion: dashboardsvc.SchemaVersion},
		Warnings:    []string{},
	}
	if result.Name == "" {
		result.Name = "Imported dashboard"
	}

	panels := flattenPanels(root.Panels)
	if len(panels) == 0 {
		result.Warnings = append(result.Warnings, "The dashboard has no panels.")
	}

	for _, panel := range panels {
		widget, warnings := convertPanel(panel)
		result.Warnings = append(result.Warnings, warnings...)
		if widget != nil {
			result.Definition.Widgets = append(result.Definition.Widgets, *widget)
		}
	}

	dashboardsvc.EnsureWidgetIds(result.Definition)
	return result, nil
}

func flattenPanels(panels []grafanaPanel) []grafanaPanel {
	flat := []grafanaPanel{}
	for _, p := range panels {
		if p.Type == "row" {
			flat = append(flat, flattenPanels(p.Panels)...)
			continue
		}
		flat = append(flat, p)
	}
	return flat
}

func panelLabel(panel grafanaPanel) string {
	title := strings.TrimSpace(panel.Title)
	if title == "" {
		return "untitled panel"
	}
	return fmt.Sprintf("panel %q", title)
}

func convertPanel(panel grafanaPanel) (*models.DashboardWidget, []string) {
	warnings := []string{}

	widgetType, ok := panelTypeMap[panel.Type]
	if !ok {
		return nil, []string{fmt.Sprintf("Skipped %s: unsupported panel type %q.", panelLabel(panel), panel.Type)}
	}
	if panel.Type == "piechart" {
		warnings = append(warnings, fmt.Sprintf("Converted %s from a pie chart to a bar chart.", panelLabel(panel)))
	}

	sources := []map[string]any{}
	for _, target := range panel.Targets {
		if target.Hide {
			continue
		}
		expr := strings.TrimSpace(target.Expr)
		if expr == "" {
			continue
		}
		if !isPrometheusTarget(target) {
			warnings = append(warnings, fmt.Sprintf("Skipped a query of %s: only Prometheus queries can be converted.", panelLabel(panel)))
			continue
		}
		source, sourceWarnings := convertQuery(expr, target.LegendFormat, panelLabel(panel))
		warnings = append(warnings, sourceWarnings...)
		if source != nil {
			sources = append(sources, source)
		}
	}

	if len(sources) == 0 {
		warnings = append(warnings, fmt.Sprintf("Skipped %s: no convertible queries.", panelLabel(panel)))
		return nil, warnings
	}

	if widgetType == "single_value" || widgetType == "gauge" {
		for _, s := range sources {
			if s["aggregation"] == "avg" {
				s["aggregation"] = "last"
			}
		}
	}

	config := map[string]any{
		"sources": sources,
		"colSpan": colSpanFromWidth(panel.GridPos.W),
		"size":    sizeFromHeight(panel.GridPos.H),
	}
	if unit := extractUnit(panel.FieldConfig); unit != "" {
		config["unit"] = unit
	}

	configJSON, err := json.Marshal(config)
	if err != nil {
		return nil, append(warnings, fmt.Sprintf("Skipped %s: %v.", panelLabel(panel), err))
	}

	title := strings.TrimSpace(panel.Title)
	if title == "" {
		title = "Untitled"
	}

	return &models.DashboardWidget{
		Title:      title,
		WidgetType: widgetType,
		Config:     configJSON,
	}, warnings
}

func isPrometheusTarget(target grafanaTarget) bool {
	if len(target.Datasource) == 0 {
		return true
	}
	var typed struct {
		Type string `json:"type"`
	}
	if err := json.Unmarshal(target.Datasource, &typed); err == nil && typed.Type != "" {
		return typed.Type == "prometheus"
	}
	return true
}

var (
	aggregationPrefixRe = regexp.MustCompile(`^(sum|avg|max|min|count)\s*(by\s*\(([^)]*)\)\s*)?\(`)
	aggregationByRe     = regexp.MustCompile(`\)\s*by\s*\(([^)]*)\)\s*$`)
	rateWrapperRe       = regexp.MustCompile(`^(rate|irate|increase|delta|deriv)\s*\(`)
	metricNameRe        = regexp.MustCompile(`^([a-zA-Z_:][a-zA-Z0-9_:.]*)`)
	labelMatcherRe      = regexp.MustCompile(`([a-zA-Z_][a-zA-Z0-9_]*)\s*(=~|!~|!=|=)\s*"((?:[^"\\]|\\.)*)"`)
)

func convertQuery(expr, legendFormat, panelName string) (map[string]any, []string) {
	warnings := []string{}
	rest := expr
	aggregation := "avg"
	groupBy := ""

	for {
		if m := aggregationPrefixRe.FindStringSubmatch(rest); m != nil {
			aggregation = m[1]
			if m[3] != "" {
				groupBy = firstGroupByLabel(m[3])
			}
			rest = strings.TrimSpace(rest[len(m[0]):])
			rest = strings.TrimSuffix(rest, ")")
			if bm := aggregationByRe.FindStringSubmatch(expr); bm != nil && groupBy == "" {
				groupBy = firstGroupByLabel(bm[1])
			}
			continue
		}
		if m := rateWrapperRe.FindStringSubmatch(rest); m != nil {
			warnings = append(warnings, fmt.Sprintf("Approximated %s() in %s with the %s aggregation over time buckets.", m[1], panelName, aggregation))
			rest = strings.TrimSpace(rest[len(m[0]):])
			rest = strings.TrimSuffix(rest, ")")
			if idx := strings.LastIndex(rest, "["); idx >= 0 {
				rest = rest[:idx]
			}
			continue
		}
		break
	}

	rest = strings.TrimSpace(rest)
	nameMatch := metricNameRe.FindStringSubmatch(rest)
	if nameMatch == nil {
		return nil, append(warnings, fmt.Sprintf("Skipped a query of %s: could not extract a metric name from %q.", panelName, expr))
	}
	metricName := nameMatch[1]

	tagFilters := map[string]string{}
	if start := strings.Index(rest, "{"); start >= 0 {
		end := strings.Index(rest[start:], "}")
		if end > 0 {
			for _, m := range labelMatcherRe.FindAllStringSubmatch(rest[start:start+end+1], -1) {
				if m[2] != "=" {
					warnings = append(warnings, fmt.Sprintf("Dropped the %s%s\"%s\" matcher in %s: only exact matches are supported.", m[1], m[2], m[3], panelName))
					continue
				}
				tagFilters[m[1]] = m[3]
			}
		}
	}

	source := map[string]any{
		"type":        "metric",
		"name":        metricName,
		"aggregation": aggregation,
	}
	if len(tagFilters) > 0 {
		source["tagFilters"] = tagFilters
	}
	if groupBy != "" {
		source["groupBy"] = groupBy
	}
	if label := strings.TrimSpace(legendFormat); label != "" && !strings.Contains(label, "{{") {
		source["label"] = label
	}
	return source, warnings
}

func firstGroupByLabel(list string) string {
	for _, part := range strings.Split(list, ",") {
		label := strings.TrimSpace(part)
		if label != "" {
			return label
		}
	}
	return ""
}

func extractUnit(fieldConfig json.RawMessage) string {
	if len(fieldConfig) == 0 {
		return ""
	}
	var cfg struct {
		Defaults struct {
			Unit string `json:"unit"`
		} `json:"defaults"`
	}
	if err := json.Unmarshal(fieldConfig, &cfg); err != nil {
		return ""
	}
	if mapped, ok := unitMap[cfg.Defaults.Unit]; ok {
		return mapped
	}
	return cfg.Defaults.Unit
}

func colSpanFromWidth(w int) int {
	switch {
	case w >= 16:
		return 3
	case w >= 10:
		return 2
	default:
		return 1
	}
}

func sizeFromHeight(h int) string {
	switch {
	case h >= 12:
		return "lg"
	case h >= 8:
		return "md"
	default:
		return "sm"
	}
}
