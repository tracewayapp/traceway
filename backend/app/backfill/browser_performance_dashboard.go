package backfill

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/tracewayapp/lit/v2"
	"github.com/tracewayapp/traceway/backend/v2/app/config"
	"github.com/tracewayapp/traceway/backend/v2/app/db"
	"github.com/tracewayapp/traceway/backend/v2/app/models"
	"github.com/tracewayapp/traceway/backend/v2/app/repositories/transactional"
	dashboardsvc "github.com/tracewayapp/traceway/backend/v2/app/services/dashboards"
)

const browserPerformanceBackfillName = "browser-performance-dashboard-percentiles"

const browserPerformanceTemplateKey = "browser-performance"

type originalWidget struct {
	widgetType string
	config     string
}

// originalBrowserPerformanceWidgets are the widgets of the template as first seeded
// (averages and maxima over metric points), by title, with their config as canonical
// JSON. An installed widget equal to one of these was never edited, so it can take
// the percentile version; anything else is the user's and is left alone.
var originalBrowserPerformanceWidgets = map[string]originalWidget{
	"Largest Contentful Paint (ms)":      {widgetType: "line_chart", config: `{"sources":[{"aggregation":"avg","label":"Average","name":"browser.web_vital.lcp","type":"metric"},{"aggregation":"max","label":"Slowest","name":"browser.web_vital.lcp","type":"metric"}]}`},
	"Interaction to Next Paint (ms)":     {widgetType: "line_chart", config: `{"sources":[{"aggregation":"avg","label":"Average","name":"browser.web_vital.inp","type":"metric"},{"aggregation":"max","label":"Slowest","name":"browser.web_vital.inp","type":"metric"}]}`},
	"Cumulative Layout Shift":            {widgetType: "line_chart", config: `{"sources":[{"aggregation":"avg","label":"Average","name":"browser.web_vital.cls","type":"metric"},{"aggregation":"max","label":"Worst","name":"browser.web_vital.cls","type":"metric"}]}`},
	"First Contentful Paint (ms)":        {widgetType: "line_chart", config: `{"sources":[{"aggregation":"avg","label":"Average","name":"browser.web_vital.fcp","type":"metric"}]}`},
	"Time to First Byte (ms)":            {widgetType: "line_chart", config: `{"sources":[{"aggregation":"avg","label":"Average","name":"browser.web_vital.ttfb","type":"metric"}]}`},
	"Poor LCP Page Views":                {widgetType: "bar_chart", config: `{"sources":[{"aggregation":"count","label":"Poor","name":"browser.web_vital.lcp","tagFilters":{"rating":"poor"},"type":"metric"}]}`},
	"LCP by Browser (ms)":                {widgetType: "line_chart", config: `{"sources":[{"aggregation":"avg","groupBy":"browser.name","name":"browser.web_vital.lcp","type":"metric"}]}`},
	"Route Change Duration (ms)":         {widgetType: "line_chart", config: `{"sources":[{"aggregation":"avg","label":"Average","name":"browser.route_timing.duration","type":"metric"},{"aggregation":"max","label":"Slowest","name":"browser.route_timing.duration","type":"metric"}]}`},
	"Route Change Duration by Page (ms)": {widgetType: "line_chart", config: `{"sources":[{"aggregation":"avg","groupBy":"url.path","name":"browser.route_timing.duration","type":"metric"}]}`},
	"Long Tasks":                         {widgetType: "bar_chart", config: `{"sources":[{"aggregation":"count","label":"Blocks of 50 ms or more","name":"browser.long_task.duration","type":"metric"}]}`},
	"Longest Main-Thread Block (ms)":     {widgetType: "line_chart", config: `{"sources":[{"aggregation":"max","label":"Longest","name":"browser.long_task.duration","type":"metric"}]}`},
	"Long Tasks by Page":                 {widgetType: "bar_chart", config: `{"sources":[{"aggregation":"count","groupBy":"url.path","name":"browser.long_task.duration","type":"metric"}]}`},
	"Page Load (ms)":                     {widgetType: "line_chart", config: `{"sources":[{"aggregation":"avg","label":"Load event","name":"browser.page_load.duration","type":"metric"},{"aggregation":"avg","label":"DOM content loaded","name":"browser.page_load.dom_content_loaded","type":"metric"}]}`},
}

// renamedBrowserPerformanceWidgets maps an original title to the template widget
// that replaces it where the title changed with the meaning.
var renamedBrowserPerformanceWidgets = map[string]string{
	"Poor LCP Page Views": "LCP Ratings",
}

// RunBrowserPerformanceDashboards moves installed copies of the Web Performance
// template to percentiles. The first version charted averages because the query API
// had nothing else; the same metric names are distributions now, so a p75 is what the
// web vitals are defined by. A dashboard nobody edited becomes the current template;
// in an edited one only the untouched widgets are replaced.
func RunBrowserPerformanceDashboards() error {
	updated, err := db.ExecuteTransaction(func(tx *sql.Tx) (int, error) {
		if !db.IsSQLite() {
			if _, err := tx.Exec("SELECT pg_advisory_xact_lock(824737007)"); err != nil {
				return 0, fmt.Errorf("failed to take backfill lock: %w", err)
			}
		}
		marker, err := lit.SelectSingleNamed[models.CountResult](
			tx,
			"SELECT COUNT(*) as count FROM backfill_runs WHERE name = :name",
			lit.P{"name": browserPerformanceBackfillName},
		)
		if err != nil {
			return 0, fmt.Errorf("failed to check backfill marker: %w", err)
		}
		if marker != nil && marker.Count > 0 {
			return 0, nil
		}

		updated := 0
		template, err := transactional.DashboardTemplateRepository.FindByKey(tx, browserPerformanceTemplateKey)
		if err != nil {
			return 0, fmt.Errorf("failed to load the Web Performance template: %w", err)
		}
		if template != nil {
			current, err := dashboardsvc.ParseDefinition([]byte(template.Definition))
			if err != nil {
				return 0, fmt.Errorf("failed to read the Web Performance template: %w", err)
			}
			dashboards, err := lit.SelectNamed[models.Dashboard](
				tx,
				"SELECT id, organization_id, name, description, definition, template_key, created_by, created_at, updated_at FROM dashboards WHERE template_key = :key",
				lit.P{"key": browserPerformanceTemplateKey},
			)
			if err != nil {
				return 0, fmt.Errorf("failed to list Web Performance dashboards: %w", err)
			}
			for _, dashboard := range dashboards {
				changed, err := upgradeBrowserPerformanceDashboard(dashboard, current.Widgets)
				if err != nil {
					return 0, fmt.Errorf("dashboard %d: %w", dashboard.Id, err)
				}
				if !changed {
					continue
				}
				dashboard.UpdatedAt = time.Now().UTC()
				if err := transactional.DashboardRepository.Update(tx, dashboard); err != nil {
					return 0, fmt.Errorf("failed to update dashboard %d: %w", dashboard.Id, err)
				}
				updated++
			}
		}

		if err := lit.DeleteNamed(
			db.Driver,
			tx,
			"INSERT INTO backfill_runs (name, ran_at) VALUES (:name, :ran_at)",
			lit.P{"name": browserPerformanceBackfillName, "ran_at": time.Now().UTC()},
		); err != nil {
			return 0, fmt.Errorf("failed to record backfill marker: %w", err)
		}
		return updated, nil
	})
	if err != nil {
		return err
	}
	if updated > 0 {
		config.Logf("Backfill: moved %d Web Performance dashboard(s) to percentiles", updated)
	}
	return nil
}

// canonicalJSON re-encodes a JSON value with sorted keys, so two configs compare
// equal whatever order their fields were stored in.
func canonicalJSON(raw json.RawMessage) (string, error) {
	var value any
	if err := json.Unmarshal(raw, &value); err != nil {
		return "", err
	}
	out, err := json.Marshal(value)
	return string(out), err
}

func isOriginalBrowserPerformanceWidget(widget models.DashboardWidget) bool {
	original, ok := originalBrowserPerformanceWidgets[widget.Title]
	if !ok || original.widgetType != widget.WidgetType {
		return false
	}
	config, err := canonicalJSON(widget.Config)
	return err == nil && config == original.config
}

func upgradeBrowserPerformanceDashboard(dashboard *models.Dashboard, template []models.DashboardWidget) (bool, error) {
	def, err := dashboardsvc.ParseDefinition([]byte(dashboard.Definition))
	if err != nil {
		return false, err
	}
	before, err := dashboardsvc.MarshalDefinition(def)
	if err != nil {
		return false, err
	}
	byTitle := make(map[string]models.DashboardWidget, len(template))
	for _, widget := range template {
		byTitle[widget.Title] = widget
	}

	untouched := 0
	for _, widget := range def.Widgets {
		if isOriginalBrowserPerformanceWidget(widget) {
			untouched++
		}
	}
	if untouched == 0 {
		return false, nil
	}

	if untouched == len(def.Widgets) && len(def.Widgets) == len(originalBrowserPerformanceWidgets) {
		// Nobody edited it: it becomes the template as it is now, new widgets included.
		def.Widgets = make([]models.DashboardWidget, len(template))
		for i, widget := range template {
			widget.Id = dashboardsvc.NewWidgetId()
			def.Widgets[i] = widget
		}
	} else {
		for i, widget := range def.Widgets {
			if !isOriginalBrowserPerformanceWidget(widget) {
				continue
			}
			title := strings.TrimSuffix(widget.Title, " (ms)")
			if renamed, ok := renamedBrowserPerformanceWidgets[widget.Title]; ok {
				title = renamed
			}
			replacement, ok := byTitle[title]
			if !ok {
				continue
			}
			replacement.Id = widget.Id
			def.Widgets[i] = replacement
		}
	}

	raw, err := dashboardsvc.MarshalDefinition(def)
	if err != nil {
		return false, err
	}
	// A widget the two versions share is an original and its own replacement.
	if string(raw) == string(before) {
		return false, nil
	}
	dashboard.Definition = models.JSONText(raw)
	return true, nil
}
