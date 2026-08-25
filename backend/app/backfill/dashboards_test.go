//go:build !telemetry_ch && !telemetry_firebolt

package backfill

import (
	"database/sql"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/tracewayapp/lit/v2"
	"github.com/tracewayapp/traceway/backend/app/db"
	"github.com/tracewayapp/traceway/backend/app/models"
	_ "modernc.org/sqlite"
)

func setup(t *testing.T) {
	t.Helper()

	mainDB, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open main db: %v", err)
	}
	mainDB.SetMaxOpenConns(1)

	ddl := []string{
		`CREATE TABLE projects (id TEXT PRIMARY KEY, name TEXT NOT NULL, organization_id INTEGER)`,
		`CREATE TABLE widget_groups (id INTEGER PRIMARY KEY AUTOINCREMENT, project_id TEXT NOT NULL, name TEXT NOT NULL, description TEXT DEFAULT '', is_default INTEGER NOT NULL DEFAULT 0, created_by INTEGER, created_at DATETIME NOT NULL, updated_at DATETIME NOT NULL)`,
		`CREATE TABLE widget_group_widgets (id INTEGER PRIMARY KEY AUTOINCREMENT, widget_group_id INTEGER NOT NULL, title TEXT NOT NULL, widget_type TEXT NOT NULL, config TEXT NOT NULL DEFAULT '{}', position INTEGER NOT NULL DEFAULT 0, created_at DATETIME NOT NULL, updated_at DATETIME NOT NULL)`,
		`CREATE TABLE starred_widgets (id INTEGER PRIMARY KEY AUTOINCREMENT, widget_id INTEGER NOT NULL UNIQUE, position INTEGER NOT NULL DEFAULT 0, col_span INTEGER NOT NULL DEFAULT 1, size TEXT NOT NULL DEFAULT 'sm', created_at DATETIME NOT NULL)`,
		`CREATE TABLE dashboards (id INTEGER PRIMARY KEY AUTOINCREMENT, organization_id INTEGER NOT NULL, name TEXT NOT NULL, description TEXT NOT NULL DEFAULT '', definition TEXT NOT NULL DEFAULT '{}', template_key TEXT, created_by INTEGER, created_at DATETIME NOT NULL, updated_at DATETIME NOT NULL)`,
		`CREATE TABLE project_dashboards (id INTEGER PRIMARY KEY AUTOINCREMENT, project_id TEXT NOT NULL, dashboard_id INTEGER NOT NULL, position INTEGER NOT NULL DEFAULT 0, created_at DATETIME NOT NULL, UNIQUE(project_id, dashboard_id))`,
		`CREATE TABLE starred_dashboard_widgets (id INTEGER PRIMARY KEY AUTOINCREMENT, project_id TEXT NOT NULL, dashboard_id INTEGER NOT NULL, widget_id TEXT NOT NULL, position INTEGER NOT NULL DEFAULT 0, col_span INTEGER NOT NULL DEFAULT 1, size TEXT NOT NULL DEFAULT 'sm', created_at DATETIME NOT NULL, UNIQUE(project_id, dashboard_id, widget_id))`,
		`CREATE TABLE backfill_runs (name TEXT PRIMARY KEY, ran_at DATETIME NOT NULL)`,
		`CREATE UNIQUE INDEX idx_dashboards_org_lower_name ON dashboards(organization_id, LOWER(name))`,
	}
	for _, stmt := range ddl {
		if _, err := mainDB.Exec(stmt); err != nil {
			t.Fatalf("ddl: %v", err)
		}
	}

	prevDB, prevDriver := db.DB, db.Driver
	db.DB = mainDB
	db.Driver = lit.SQLite
	models.Init(db.Driver)

	t.Cleanup(func() {
		mainDB.Close()
		db.DB = prevDB
		db.Driver = prevDriver
	})
}

func seedGroup(t *testing.T, projectId uuid.UUID, name string, isDefault bool, widgetTitles []string) []int {
	t.Helper()
	now := time.Now().UTC()
	group := models.WidgetGroup{
		ProjectId: projectId,
		Name:      name,
		IsDefault: isDefault,
		CreatedAt: now,
		UpdatedAt: now,
	}
	groupId, err := lit.Insert(db.DB, &group)
	if err != nil {
		t.Fatalf("seed widget_group: %v", err)
	}
	widgetIds := make([]int, 0, len(widgetTitles))
	for i, title := range widgetTitles {
		w := models.WidgetGroupWidget{
			WidgetGroupId: groupId,
			Title:         title,
			WidgetType:    "line_chart",
			Config:        models.JSONText(`{"sources":[{"type":"metric","name":"cpu.used_pcnt","aggregation":"avg"}]}`),
			Position:      i,
			CreatedAt:     now,
			UpdatedAt:     now,
		}
		id, err := lit.Insert(db.DB, &w)
		if err != nil {
			t.Fatalf("seed widget: %v", err)
		}
		widgetIds = append(widgetIds, id)
	}
	return widgetIds
}

func TestBackfillConvertsGroupsToDashboards(t *testing.T) {
	setup(t)

	projectId := uuid.New()
	if _, err := db.DB.Exec(`INSERT INTO projects (id, name, organization_id) VALUES (?, 'p1', 7)`, projectId.String()); err != nil {
		t.Fatalf("seed project: %v", err)
	}

	customIds := seedGroup(t, projectId, "Custom", false, []string{"A", "B"})
	seedGroup(t, projectId, "CPU / Mem", true, []string{"CPU Usage"})

	star := models.StarredWidget{WidgetId: customIds[1], Position: 3, ColSpan: 2, Size: "md", CreatedAt: time.Now().UTC()}
	if err := db.DB.QueryRow(`INSERT INTO starred_widgets (widget_id, position, col_span, size, created_at) VALUES (?, ?, ?, ?, ?) RETURNING id`,
		star.WidgetId, star.Position, star.ColSpan, star.Size, star.CreatedAt).Scan(&star.Id); err != nil {
		t.Fatalf("seed starred: %v", err)
	}

	if err := RunDashboards(); err != nil {
		t.Fatalf("RunDashboards: %v", err)
	}

	dashboards, err := lit.SelectNamed[models.Dashboard](db.DB, "SELECT id, organization_id, name, description, definition, template_key, created_by, created_at, updated_at FROM dashboards ORDER BY id ASC", lit.P{})
	if err != nil {
		t.Fatalf("select dashboards: %v", err)
	}
	if len(dashboards) != 2 {
		t.Fatalf("expected 2 dashboards, got %d", len(dashboards))
	}
	for _, d := range dashboards {
		if d.OrganizationId != 7 {
			t.Errorf("dashboard %q has org %d, want 7", d.Name, d.OrganizationId)
		}
	}

	assignments, err := lit.SelectNamed[models.ProjectDashboard](db.DB, "SELECT id, project_id, dashboard_id, position, created_at FROM project_dashboards ORDER BY position ASC", lit.P{})
	if err != nil {
		t.Fatalf("select assignments: %v", err)
	}
	if len(assignments) != 2 {
		t.Fatalf("expected 2 assignments, got %d", len(assignments))
	}

	byId := map[int]*models.Dashboard{}
	for _, d := range dashboards {
		byId[d.Id] = d
	}
	if byId[assignments[0].DashboardId].Name != "CPU / Mem" {
		t.Errorf("first tab should be the default group, got %q", byId[assignments[0].DashboardId].Name)
	}
	if byId[assignments[1].DashboardId].Name != "Custom" {
		t.Errorf("second tab should be Custom, got %q", byId[assignments[1].DashboardId].Name)
	}

	var customDef models.DashboardDefinition
	customDashboardId := assignments[1].DashboardId
	if err := json.Unmarshal(byId[customDashboardId].Definition, &customDef); err != nil {
		t.Fatalf("unmarshal definition: %v", err)
	}
	if len(customDef.Widgets) != 2 || customDef.Widgets[0].Title != "A" || customDef.Widgets[1].Title != "B" {
		t.Fatalf("unexpected widgets in Custom definition: %+v", customDef.Widgets)
	}
	for _, w := range customDef.Widgets {
		if len(w.Id) != 10 || w.Id[:2] != "w_" {
			t.Errorf("widget id %q not in expected format", w.Id)
		}
	}

	starred, err := lit.SelectNamed[models.StarredDashboardWidget](db.DB, "SELECT id, project_id, dashboard_id, widget_id, position, col_span, size, created_at FROM starred_dashboard_widgets", lit.P{})
	if err != nil {
		t.Fatalf("select starred: %v", err)
	}
	if len(starred) != 1 {
		t.Fatalf("expected 1 starred widget, got %d", len(starred))
	}
	if starred[0].DashboardId != customDashboardId || starred[0].WidgetId != customDef.Widgets[1].Id {
		t.Errorf("star should reference widget B of Custom, got dashboard %d widget %s", starred[0].DashboardId, starred[0].WidgetId)
	}
	if starred[0].Position != 3 || starred[0].ColSpan != 2 || starred[0].Size != "md" {
		t.Errorf("star layout not preserved: %+v", starred[0])
	}

	if err := RunDashboards(); err != nil {
		t.Fatalf("second RunDashboards: %v", err)
	}
	again, err := lit.SelectNamed[models.Dashboard](db.DB, "SELECT id, organization_id, name, description, definition, template_key, created_by, created_at, updated_at FROM dashboards", lit.P{})
	if err != nil {
		t.Fatalf("select dashboards after rerun: %v", err)
	}
	if len(again) != 2 {
		t.Fatalf("backfill is not idempotent: got %d dashboards after rerun", len(again))
	}
}

func TestBackfillSurvivesLegacyNullsAndLongNames(t *testing.T) {
	setup(t)

	projectId := uuid.New()
	if _, err := db.DB.Exec(`INSERT INTO projects (id, name, organization_id) VALUES (?, 'p1', 3)`, projectId.String()); err != nil {
		t.Fatalf("seed project: %v", err)
	}

	longName := strings.Repeat("x", 150)
	now := time.Now().UTC()
	if _, err := db.DB.Exec(
		`INSERT INTO widget_groups (project_id, name, description, is_default, created_at, updated_at) VALUES (?, ?, NULL, 0, ?, ?)`,
		projectId.String(), longName, now, now,
	); err != nil {
		t.Fatalf("seed legacy group: %v", err)
	}
	var groupId int
	if err := db.DB.QueryRow(`SELECT id FROM widget_groups`).Scan(&groupId); err != nil {
		t.Fatalf("read group id: %v", err)
	}
	if _, err := db.DB.Exec(
		`INSERT INTO widget_group_widgets (widget_group_id, title, widget_type, config, position, created_at, updated_at) VALUES (?, 'W', 'line_chart', '{"sources":[{"type":"metric","name":"m","aggregation":"avg"}]}', 0, ?, ?)`,
		groupId, now, now,
	); err != nil {
		t.Fatalf("seed widget: %v", err)
	}

	if err := RunDashboards(); err != nil {
		t.Fatalf("RunDashboards: %v", err)
	}

	dashboards, err := lit.SelectNamed[models.Dashboard](db.DB, "SELECT id, organization_id, name, description, definition, template_key, created_by, created_at, updated_at FROM dashboards", lit.P{})
	if err != nil {
		t.Fatalf("select dashboards: %v", err)
	}
	if len(dashboards) != 1 {
		t.Fatalf("expected 1 dashboard, got %d", len(dashboards))
	}
	if len(dashboards[0].Name) != 100 {
		t.Errorf("name should be truncated to 100 chars, got %d", len(dashboards[0].Name))
	}
	if dashboards[0].Description != "" {
		t.Errorf("NULL description should become empty string, got %q", dashboards[0].Description)
	}
}

func TestBackfillSkipsWhenNoGroups(t *testing.T) {
	setup(t)
	if err := RunDashboards(); err != nil {
		t.Fatalf("RunDashboards: %v", err)
	}
	var n int
	if err := db.DB.QueryRow("SELECT count(*) FROM dashboards").Scan(&n); err != nil {
		t.Fatalf("count: %v", err)
	}
	if n != 0 {
		t.Fatalf("expected no dashboards, got %d", n)
	}
	if err := db.DB.QueryRow("SELECT count(*) FROM backfill_runs WHERE name = 'dashboards'").Scan(&n); err != nil {
		t.Fatalf("count marker: %v", err)
	}
	if n != 1 {
		t.Fatalf("expected marker row even with no groups, got %d", n)
	}
}

func TestBackfillMarkerPreventsResurrectionAfterDeletes(t *testing.T) {
	setup(t)

	projectId := uuid.New()
	if _, err := db.DB.Exec(`INSERT INTO projects (id, name, organization_id) VALUES (?, 'p1', 7)`, projectId.String()); err != nil {
		t.Fatalf("seed project: %v", err)
	}
	seedGroup(t, projectId, "Custom", false, []string{"A"})

	if err := RunDashboards(); err != nil {
		t.Fatalf("RunDashboards: %v", err)
	}
	var n int
	if err := db.DB.QueryRow("SELECT count(*) FROM dashboards").Scan(&n); err != nil {
		t.Fatalf("count: %v", err)
	}
	if n != 1 {
		t.Fatalf("expected 1 dashboard after first run, got %d", n)
	}

	if _, err := db.DB.Exec("DELETE FROM starred_dashboard_widgets"); err != nil {
		t.Fatalf("delete starred: %v", err)
	}
	if _, err := db.DB.Exec("DELETE FROM project_dashboards"); err != nil {
		t.Fatalf("delete assignments: %v", err)
	}
	if _, err := db.DB.Exec("DELETE FROM dashboards"); err != nil {
		t.Fatalf("delete dashboards: %v", err)
	}

	if err := RunDashboards(); err != nil {
		t.Fatalf("second RunDashboards: %v", err)
	}
	if err := db.DB.QueryRow("SELECT count(*) FROM dashboards").Scan(&n); err != nil {
		t.Fatalf("count after rerun: %v", err)
	}
	if n != 0 {
		t.Fatalf("deleted dashboards were resurrected: got %d", n)
	}
	if err := db.DB.QueryRow("SELECT count(*) FROM backfill_runs WHERE name = 'dashboards'").Scan(&n); err != nil {
		t.Fatalf("count marker: %v", err)
	}
	if n != 1 {
		t.Fatalf("expected exactly 1 marker row, got %d", n)
	}
}

func TestBackfillDedupesNamesWithinOrg(t *testing.T) {
	setup(t)

	p1 := uuid.MustParse("00000000-0000-0000-0000-000000000001")
	p2 := uuid.MustParse("00000000-0000-0000-0000-000000000002")
	p3 := uuid.MustParse("00000000-0000-0000-0000-000000000003")
	for _, p := range []struct {
		id   uuid.UUID
		name string
	}{{p1, "Alpha"}, {p2, "Beta"}, {p3, "Beta"}} {
		if _, err := db.DB.Exec(`INSERT INTO projects (id, name, organization_id) VALUES (?, ?, 9)`, p.id.String(), p.name); err != nil {
			t.Fatalf("seed project: %v", err)
		}
	}
	seedGroup(t, p1, "CPU / Mem", false, []string{"A"})
	seedGroup(t, p2, "cpu / mem", false, []string{"B"})
	seedGroup(t, p3, "CPU / Mem", false, []string{"C"})

	if err := RunDashboards(); err != nil {
		t.Fatalf("RunDashboards: %v", err)
	}

	assignments, err := lit.SelectNamed[models.ProjectDashboard](db.DB, "SELECT id, project_id, dashboard_id, position, created_at FROM project_dashboards", lit.P{})
	if err != nil {
		t.Fatalf("select assignments: %v", err)
	}
	dashboards, err := lit.SelectNamed[models.Dashboard](db.DB, "SELECT id, organization_id, name, description, definition, template_key, created_by, created_at, updated_at FROM dashboards", lit.P{})
	if err != nil {
		t.Fatalf("select dashboards: %v", err)
	}
	if len(dashboards) != 3 {
		t.Fatalf("expected 3 dashboards, got %d", len(dashboards))
	}

	nameById := map[int]string{}
	for _, d := range dashboards {
		nameById[d.Id] = d.Name
	}
	nameByProject := map[uuid.UUID]string{}
	for _, a := range assignments {
		nameByProject[a.ProjectId] = nameById[a.DashboardId]
	}

	if nameByProject[p1] != "CPU / Mem" {
		t.Errorf("first project should keep the plain name, got %q", nameByProject[p1])
	}
	if nameByProject[p2] != "cpu / mem (Beta)" {
		t.Errorf("case-insensitive collision should get the project suffix, got %q", nameByProject[p2])
	}
	if nameByProject[p3] != "CPU / Mem (Beta) 2" {
		t.Errorf("repeat collision on the same project name should get a numeral, got %q", nameByProject[p3])
	}
}
