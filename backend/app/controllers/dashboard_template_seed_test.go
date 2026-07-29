//go:build !telemetry_ch && !transactional_pg

package controllers

import (
	"database/sql"
	"testing"

	"github.com/tracewayapp/lit/v2"
	"github.com/tracewayapp/traceway/backend/app/db"
	"github.com/tracewayapp/traceway/backend/app/migrations"
	"github.com/tracewayapp/traceway/backend/app/models"
	"github.com/tracewayapp/traceway/backend/app/repositories/transactional"
	dashboardsvc "github.com/tracewayapp/traceway/backend/app/services/dashboards"
	_ "modernc.org/sqlite"
)

func TestSeededDashboardTemplatesAreValid(t *testing.T) {
	mainDB, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open main sqlite: %v", err)
	}
	mainDB.SetMaxOpenConns(1)

	telemetryDB, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open telemetry sqlite: %v", err)
	}
	telemetryDB.SetMaxOpenConns(1)

	prevDB, prevTelemetry, prevDriver := db.DB, db.TelemetryDB, db.Driver
	db.DB = mainDB
	db.TelemetryDB = telemetryDB
	db.Driver = lit.SQLite
	models.Init(db.Driver)
	t.Cleanup(func() {
		mainDB.Close()
		telemetryDB.Close()
		db.DB = prevDB
		db.TelemetryDB = prevTelemetry
		db.Driver = prevDriver
	})

	if err := migrations.Run("sqlite"); err != nil {
		t.Fatalf("run migrations: %v", err)
	}

	tx, err := mainDB.Begin()
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	defer tx.Rollback()

	templates, err := transactional.DashboardTemplateRepository.FindAll(tx)
	if err != nil {
		t.Fatalf("list templates: %v", err)
	}

	expectedKeys := []string{
		"traceway-clickhouse", "traceway-duckdb",
		"golang", "traceway-otel-agent",
	}
	byKey := map[string]*models.DashboardTemplate{}
	for _, tpl := range templates {
		byKey[tpl.Key] = tpl
	}
	for _, key := range expectedKeys {
		if byKey[key] == nil {
			t.Errorf("template %s is not seeded", key)
		}
	}

	for _, tpl := range templates {
		def, err := dashboardsvc.ParseDefinition(tpl.Definition)
		if err != nil {
			t.Errorf("template %s definition does not parse: %v", tpl.Key, err)
			continue
		}
		if len(def.Widgets) == 0 {
			t.Errorf("template %s has no widgets", tpl.Key)
		}
		if msg := validateDefinitionWidgets(def); msg != "" {
			t.Errorf("template %s definition invalid: %s", tpl.Key, msg)
		}
	}

	for _, framework := range []string{"opentelemetry", "react", "gin", ""} {
		for _, key := range defaultTemplateKeysForFramework(framework) {
			if byKey[key] == nil {
				t.Errorf("default template %s for framework %q is not seeded", key, framework)
			}
		}
	}
}
