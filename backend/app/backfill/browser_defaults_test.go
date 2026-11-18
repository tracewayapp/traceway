//go:build !telemetry_ch && !transactional_pg && !telemetry_duckdb

package backfill

import (
	"database/sql"
	"testing"

	"github.com/tracewayapp/lit/v2"
	"github.com/tracewayapp/traceway/backend/v2/app/db"
	"github.com/tracewayapp/traceway/backend/v2/app/migrations"
	"github.com/tracewayapp/traceway/backend/v2/app/models"
	"github.com/tracewayapp/traceway/backend/v2/app/repositories/transactional"
	dashboardsvc "github.com/tracewayapp/traceway/backend/v2/app/services/dashboards"
)

func TestBrowserDefaultsCreationMigrationAndDeletion(t *testing.T) {
	mainDB, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	telemetryDB, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	mainDB.SetMaxOpenConns(1)
	prevDB, prevTelemetry, prevDriver := db.DB, db.TelemetryDB, db.Driver
	db.DB, db.TelemetryDB, db.Driver = mainDB, telemetryDB, lit.SQLite
	models.Init(db.Driver)
	t.Cleanup(func() {
		mainDB.Close()
		telemetryDB.Close()
		db.DB, db.TelemetryDB, db.Driver = prevDB, prevTelemetry, prevDriver
	})
	if err := migrations.Run("sqlite"); err != nil {
		t.Fatal(err)
	}
	var projects []*models.Project
	_, err = db.ExecuteTransaction(func(tx *sql.Tx) (struct{}, error) {
		org, err := transactional.OrganizationRepository.Create(tx, "Browser defaults", "UTC")
		if err != nil {
			return struct{}{}, err
		}
		for _, framework := range []string{"react", "vuejs", "svelte", "jquery"} {
			project, err := transactional.ProjectRepository.CreateWithOrganization(tx, framework, framework, org.Id)
			if err != nil {
				return struct{}{}, err
			}
			projects = append(projects, project)
			assigned, err := transactional.DashboardRepository.FindByProject(tx, project.Id)
			if err != nil {
				return struct{}{}, err
			}
			if len(assigned) != 1 {
				t.Fatalf("%s creation installed %d dashboards", framework, len(assigned))
			}
			def, err := dashboardsvc.ParseDefinition(assigned[0].Definition)
			if err != nil {
				return struct{}{}, err
			}
			for _, w := range def.Widgets {
				if w.Id == "" {
					t.Fatal("missing widget ID")
				}
			}
			if err := dashboardsvc.EnsureBrowserDefault(tx, project); err != nil {
				return struct{}{}, err
			}
		}
		return struct{}{}, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	// Simulate a pre-upgrade project whose custom dashboard must survive migration.
	const customDefinition = `{"schemaVersion":1,"widgets":[]}`
	if _, err := db.DB.Exec("UPDATE dashboards SET template_key = NULL, definition = ? WHERE id IN (SELECT dashboard_id FROM project_dashboards WHERE project_id = ?)", customDefinition, projects[0].Id); err != nil {
		t.Fatal(err)
	}
	if err := RunBrowserDefaults(); err != nil {
		t.Fatal(err)
	}
	var count int
	if err := db.DB.QueryRow("SELECT COUNT(*) FROM project_dashboards").Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 5 {
		t.Fatalf("migration produced %d assignments", count)
	}
	var preserved string
	if err := db.DB.QueryRow("SELECT d.definition FROM dashboards d JOIN project_dashboards pd ON pd.dashboard_id = d.id WHERE pd.project_id = ? AND pd.position = 0", projects[0].Id).Scan(&preserved); err != nil {
		t.Fatal(err)
	}
	if preserved != customDefinition {
		t.Fatal("migration changed the custom dashboard or its position")
	}
	if _, err := db.DB.Exec("DELETE FROM project_dashboards WHERE project_id = ?", projects[0].Id); err != nil {
		t.Fatal(err)
	}
	if err := RunBrowserDefaults(); err != nil {
		t.Fatal(err)
	}
	if err := db.DB.QueryRow("SELECT COUNT(*) FROM project_dashboards").Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 3 {
		t.Fatalf("migration restored an intentionally removed dashboard: %d", count)
	}
}
