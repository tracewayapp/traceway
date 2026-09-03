package backfill

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/tracewayapp/lit/v2"
	"github.com/tracewayapp/traceway/backend/app/config"
	"github.com/tracewayapp/traceway/backend/app/db"
	"github.com/tracewayapp/traceway/backend/app/models"
	"github.com/tracewayapp/traceway/backend/app/repositories/transactional"
	dashboardsvc "github.com/tracewayapp/traceway/backend/app/services/dashboards"
)

const otelAgentSourcesBackfillName = "otel-agent-dashboard-sources-v2"

const otelAgentTemplateKey = "traceway-otel-agent"

func RunOtelAgentDashboardSources() error {
	updated, err := db.ExecuteTransaction(func(tx *sql.Tx) (int, error) {
		if !db.IsSQLite() {
			if _, err := tx.Exec("SELECT pg_advisory_xact_lock(824737006)"); err != nil {
				return 0, fmt.Errorf("failed to take backfill lock: %w", err)
			}
		}

		marker, err := lit.SelectSingleNamed[models.CountResult](
			tx,
			"SELECT COUNT(*) as count FROM backfill_runs WHERE name = :name",
			lit.P{"name": otelAgentSourcesBackfillName},
		)
		if err != nil {
			return 0, fmt.Errorf("failed to check backfill marker: %w", err)
		}
		if marker != nil && marker.Count > 0 {
			return 0, nil
		}

		updated := 0
		template, err := transactional.DashboardTemplateRepository.FindByKey(tx, otelAgentTemplateKey)
		if err != nil {
			return 0, fmt.Errorf("failed to load the OTel agent template: %w", err)
		}
		if template != nil {
			templateSources, err := templateSourcesByMetric(template.Definition)
			if err != nil {
				return 0, fmt.Errorf("failed to read the OTel agent template: %w", err)
			}
			dashboards, err := lit.SelectNamed[models.Dashboard](
				tx,
				"SELECT id, organization_id, name, description, definition, template_key, created_by, created_at, updated_at FROM dashboards WHERE template_key = :key",
				lit.P{"key": otelAgentTemplateKey},
			)
			if err != nil {
				return 0, fmt.Errorf("failed to list OTel agent dashboards: %w", err)
			}
			for _, dashboard := range dashboards {
				changed, err := upgradeOtelAgentDashboard(dashboard, templateSources)
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
			lit.P{"name": otelAgentSourcesBackfillName, "ran_at": time.Now().UTC()},
		); err != nil {
			return 0, fmt.Errorf("failed to record backfill marker: %w", err)
		}
		return updated, nil
	})
	if err != nil {
		return err
	}
	if updated > 0 {
		config.Logf("Backfill: updated the widget sources of %d OTel agent dashboard(s)", updated)
	}
	return nil
}

type widgetConfig map[string]json.RawMessage

func templateSourcesByMetric(definition models.JSONText) (map[string]json.RawMessage, error) {
	def, err := dashboardsvc.ParseDefinition([]byte(definition))
	if err != nil {
		return nil, err
	}
	byMetric := make(map[string]json.RawMessage)
	for _, widget := range def.Widgets {
		var cfg widgetConfig
		if err := json.Unmarshal(widget.Config, &cfg); err != nil {
			return nil, err
		}
		sources, ok := cfg["sources"]
		if !ok {
			continue
		}
		var probe []struct {
			Name string `json:"name"`
		}
		if err := json.Unmarshal(sources, &probe); err != nil || len(probe) == 0 || probe[0].Name == "" {
			continue
		}
		byMetric[probe[0].Name] = sources
	}
	return byMetric, nil
}

func originalSource(sources json.RawMessage) (string, bool) {
	var probe []map[string]json.RawMessage
	if err := json.Unmarshal(sources, &probe); err != nil || len(probe) != 1 {
		return "", false
	}
	source := probe[0]
	for key := range source {
		switch key {
		case "type", "name", "aggregation":
		default:
			return "", false
		}
	}
	var name, aggregation string
	if err := json.Unmarshal(source["name"], &name); err != nil || name == "" {
		return "", false
	}
	if raw, ok := source["aggregation"]; ok {
		if err := json.Unmarshal(raw, &aggregation); err != nil {
			return "", false
		}
	}
	if aggregation != "" && aggregation != "avg" {
		return "", false
	}
	return name, true
}

func upgradeOtelAgentDashboard(dashboard *models.Dashboard, templateSources map[string]json.RawMessage) (bool, error) {
	def, err := dashboardsvc.ParseDefinition([]byte(dashboard.Definition))
	if err != nil {
		return false, err
	}
	changed := false
	for i := range def.Widgets {
		var cfg widgetConfig
		if err := json.Unmarshal(def.Widgets[i].Config, &cfg); err != nil {
			return false, err
		}
		sources, ok := cfg["sources"]
		if !ok {
			continue
		}
		name, ok := originalSource(sources)
		if !ok {
			continue
		}
		replacement, ok := templateSources[name]
		if !ok || string(replacement) == string(sources) {
			continue
		}
		cfg["sources"] = replacement
		raw, err := json.Marshal(cfg)
		if err != nil {
			return false, err
		}
		def.Widgets[i].Config = raw
		changed = true
	}
	if !changed {
		return false, nil
	}
	raw, err := dashboardsvc.MarshalDefinition(def)
	if err != nil {
		return false, err
	}
	dashboard.Definition = models.JSONText(raw)
	return true, nil
}
