//go:build !telemetry_ch && !transactional_pg && !telemetry_duckdb

package controllers

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/google/uuid"
	"github.com/tracewayapp/lit/v2"
	"github.com/tracewayapp/traceway/backend/v2/app/clientmetrics"
	"github.com/tracewayapp/traceway/backend/v2/app/db"
	"github.com/tracewayapp/traceway/backend/v2/app/middleware"
	"github.com/tracewayapp/traceway/backend/v2/app/migrations"
	"github.com/tracewayapp/traceway/backend/v2/app/models"
	"github.com/tracewayapp/traceway/backend/v2/app/repositories/transactional"
	dashboardsvc "github.com/tracewayapp/traceway/backend/v2/app/services/dashboards"
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
		"golang", "traceway-otel-agent", "browser-performance", "mobile-performance", "flutter-performance",
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

	if keys := defaultTemplateKeysForFramework("react"); len(keys) != 1 || keys[0] != "browser-performance" {
		t.Errorf("a browser project starts with the Web Performance dashboard, got %v", keys)
	}
	for _, framework := range []string{"ios", "android", "react-native"} {
		if keys := defaultTemplateKeysForFramework(framework); len(keys) != 1 || keys[0] != "mobile-performance" {
			t.Errorf("a %s project starts with the Mobile Performance dashboard, got %v", framework, keys)
		}
	}
	if keys := defaultTemplateKeysForFramework("flutter"); len(keys) != 1 || keys[0] != "flutter-performance" {
		t.Errorf("a flutter project starts with the Flutter Performance dashboard, got %v", keys)
	}

	for _, framework := range []string{"opentelemetry", "react", "gin", "flutter", "ios", ""} {
		for _, key := range defaultTemplateKeysForFramework(framework) {
			if byKey[key] == nil {
				t.Errorf("default template %s for framework %q is not seeded", key, framework)
			}
		}
	}

	if flutter := byKey["flutter-performance"]; flutter != nil {
		if flutter.Name != "Flutter Performance" || flutter.Category != "mobile" {
			t.Errorf("flutter template is %q in %q", flutter.Name, flutter.Category)
		}
		assertClientMetricTemplate(t, flutter)
	}
}

// clientMetricCommonDimensions are kept on every client metric whatever its catalog
// entry lists: the release from the resource, the country from the request and the
// screen from the data point.
var clientMetricCommonDimensions = map[string]bool{"service.version": true, clientmetrics.DimensionCountry: true, "screen": true}

// assertClientMetricTemplate checks that every chart of a client template reads a
// catalog metric with an aggregation the metric query accepts for its shape, grouped
// by a dimension the pipeline keeps. A name outside the catalog would chart nothing.
func assertClientMetricTemplate(t *testing.T, tpl *models.DashboardTemplate) {
	t.Helper()
	def, err := dashboardsvc.ParseDefinition(tpl.Definition)
	if err != nil {
		t.Fatalf("template %s definition does not parse: %v", tpl.Key, err)
	}
	for _, w := range def.Widgets {
		if w.WidgetType != "line_chart" && w.WidgetType != "breakdown_table" {
			t.Errorf("%s: widget %q is a %s", tpl.Key, w.Title, w.WidgetType)
		}
		var cfg struct {
			Sources []struct {
				Type        string `json:"type"`
				Name        string `json:"name"`
				Aggregation string `json:"aggregation"`
				GroupBy     string `json:"groupBy"`
				Label       string `json:"label"`
			} `json:"sources"`
		}
		if err := json.Unmarshal(w.Config, &cfg); err != nil {
			t.Errorf("%s: widget %q config: %v", tpl.Key, w.Title, err)
			continue
		}
		if len(cfg.Sources) == 0 {
			t.Errorf("%s: widget %q has no sources", tpl.Key, w.Title)
		}
		for _, src := range cfg.Sources {
			metric, ok := clientmetrics.Lookup(src.Name)
			if !ok {
				t.Errorf("%s: widget %q charts %s, which is not in the client metric catalog", tpl.Key, w.Title, src.Name)
				continue
			}
			if src.Type != "metric" || src.Label == "" {
				t.Errorf("%s: widget %q source %s is %q labelled %q", tpl.Key, w.Title, src.Name, src.Type, src.Label)
			}
			isPercentile := src.Aggregation == "p75" || src.Aggregation == "p95"
			if metric.Shape == clientmetrics.Counter && src.Aggregation != "sum" || metric.Shape == clientmetrics.Distribution && !isPercentile {
				t.Errorf("%s: widget %q aggregates %s with %s", tpl.Key, w.Title, src.Name, src.Aggregation)
			}
			if _, listed := metric.Dims[src.GroupBy]; src.GroupBy != "" && !listed && !clientMetricCommonDimensions[src.GroupBy] {
				t.Errorf("%s: widget %q groups %s by %s, a dimension it does not carry", tpl.Key, w.Title, src.Name, src.GroupBy)
			}
			// The Flutter SDK has no native code to see an ANR or an out-of-memory kill.
			if tpl.Key == "flutter-performance" && (src.Name == "app.anr" || src.Name == "app.oom") {
				t.Errorf("%s: widget %q charts %s, which the Flutter SDK never sends", tpl.Key, w.Title, src.Name)
			}
		}
	}
}

// assertFlutterPerformanceDashboard checks that a new Flutter project starts with
// exactly the Flutter Performance dashboard.
func assertFlutterPerformanceDashboard(t *testing.T, tx *sql.Tx, projectId uuid.UUID) {
	t.Helper()
	assignments, err := transactional.DashboardRepository.FindAssignmentsByProject(tx, projectId)
	if err != nil {
		t.Fatalf("list assignments: %v", err)
	}
	if len(assignments) != 1 {
		t.Fatalf("a new flutter project has %d dashboards, want 1", len(assignments))
	}
	dashboard, err := transactional.DashboardRepository.FindById(tx, assignments[0].DashboardId)
	if err != nil || dashboard == nil {
		t.Fatalf("load dashboard: (%v, %v)", dashboard, err)
	}
	if dashboard.TemplateKey == nil || *dashboard.TemplateKey != "flutter-performance" || dashboard.Name != "Flutter Performance" {
		t.Errorf("a new flutter project starts with %q (template %v), want Flutter Performance", dashboard.Name, dashboard.TemplateKey)
	}
}

func createdProjectId(t *testing.T, body []byte, path ...string) uuid.UUID {
	t.Helper()
	var payload map[string]any
	if err := json.Unmarshal(body, &payload); err != nil {
		t.Fatalf("decode response %q: %v", body, err)
	}
	for _, key := range path {
		payload, _ = payload[key].(map[string]any)
	}
	idStr, _ := payload["id"].(string)
	id, err := uuid.Parse(idStr)
	if err != nil {
		t.Fatalf("response %s has no project id: %v", body, err)
	}
	return id
}

// Every way a project is created installs its framework's default dashboards.
func TestNewFlutterProjectStartsWithFlutterPerformanceDashboard(t *testing.T) {
	const flutterProject = `"projectName":"Shop App","framework":"flutter"`

	t.Run("batch", func(t *testing.T) {
		setupSetupControllerDB(t)
		tx, err := db.DB.Begin()
		if err != nil {
			t.Fatalf("begin: %v", err)
		}
		defer tx.Rollback()
		userId, orgId := createSetupTestAccount(t, tx, "batch@example.com", "owner")

		results, err := batchCreateProjects(tx, orgId, userId, []BatchProjectInput{{Name: "Shop App", Framework: "flutter"}})
		if err != nil {
			t.Fatalf("batch create: %v", err)
		}
		assertFlutterPerformanceDashboard(t, tx, results[0].Project.Id)
	})

	t.Run("project page", func(t *testing.T) {
		setupSetupControllerDB(t)
		tx, err := db.DB.Begin()
		if err != nil {
			t.Fatalf("begin: %v", err)
		}
		userId, orgId := createSetupTestAccount(t, tx, "ui@example.com", "owner")
		current, err := transactional.ProjectRepository.CreateWithOrganization(tx, "Backend", "custom", orgId)
		if err != nil {
			t.Fatalf("create current project: %v", err)
		}
		if err := tx.Commit(); err != nil {
			t.Fatalf("commit: %v", err)
		}

		// CreateProject runs its own transaction.
		c, recorder := newControllerTestContext(t, nil, userId, http.MethodPost, "/api/projects", `{"name":"Shop App","framework":"flutter"}`)
		c.Set(middleware.ProjectIdContextKey, current.Id)
		ProjectController.CreateProject(c)
		if recorder.Code != http.StatusCreated {
			t.Fatalf("create project: status = %d, body = %s", recorder.Code, recorder.Body.String())
		}
		projectId := createdProjectId(t, recorder.Body.Bytes())

		tx, err = db.DB.Begin()
		if err != nil {
			t.Fatalf("begin: %v", err)
		}
		defer tx.Rollback()
		assertFlutterPerformanceDashboard(t, tx, projectId)

	})

	t.Run("react-native from the project page", func(t *testing.T) {
		// React Native has no Dashboards page: nothing is installed for it.
		setupSetupControllerDB(t)
		tx, err := db.DB.Begin()
		if err != nil {
			t.Fatalf("begin: %v", err)
		}
		userId, orgId := createSetupTestAccount(t, tx, "rn@example.com", "owner")
		current, err := transactional.ProjectRepository.CreateWithOrganization(tx, "Backend", "custom", orgId)
		if err != nil {
			t.Fatalf("create current project: %v", err)
		}
		if err := tx.Commit(); err != nil {
			t.Fatalf("commit: %v", err)
		}
		c, recorder := newControllerTestContext(t, nil, userId, http.MethodPost, "/api/projects", `{"name":"RN App","framework":"react-native"}`)
		c.Set(middleware.ProjectIdContextKey, current.Id)
		ProjectController.CreateProject(c)
		if recorder.Code != http.StatusCreated {
			t.Fatalf("create project: status = %d, body = %s", recorder.Code, recorder.Body.String())
		}
		projectId := createdProjectId(t, recorder.Body.Bytes())
		tx, err = db.DB.Begin()
		if err != nil {
			t.Fatalf("begin: %v", err)
		}
		defer tx.Rollback()
		assignments, err := transactional.DashboardRepository.FindAssignmentsByProject(tx, projectId)
		if err != nil {
			t.Fatalf("load assignments: %v", err)
		}
		if len(assignments) != 0 {
			t.Errorf("a new react-native project has %d dashboards, want none", len(assignments))
		}
	})

	t.Run("registration", func(t *testing.T) {
		setupSetupControllerDB(t)
		initRegistrationJWT(t)
		tx, err := db.DB.Begin()
		if err != nil {
			t.Fatalf("begin: %v", err)
		}
		defer tx.Rollback()

		c, recorder := newControllerTestContext(t, tx, 0, http.MethodPost, "/api/register",
			`{"email":"new@example.com","name":"A","password":"password1","organizationName":"Shop","timezone":"UTC",`+flutterProject+`}`)
		AuthController.Register(c)
		if recorder.Code != http.StatusCreated {
			t.Fatalf("register: status = %d, body = %s", recorder.Code, recorder.Body.String())
		}
		assertFlutterPerformanceDashboard(t, tx, createdProjectId(t, recorder.Body.Bytes(), "project"))
	})

	t.Run("oauth signup", func(t *testing.T) {
		setupSetupControllerDB(t)
		initRegistrationJWT(t)
		tx, err := db.DB.Begin()
		if err != nil {
			t.Fatalf("begin: %v", err)
		}
		defer tx.Rollback()
		user, err := transactional.UserRepository.Create(tx, "oauth@example.com", "OAuth User", "hashed-password")
		if err != nil {
			t.Fatalf("create user: %v", err)
		}

		c, recorder := newControllerTestContext(t, tx, user.Id, http.MethodPost, "/api/oauth/setup",
			`{"organizationName":"Shop","timezone":"UTC",`+flutterProject+`}`)
		OAuthController.FinishSetup(c)
		if recorder.Code != http.StatusCreated {
			t.Fatalf("finish setup: status = %d, body = %s", recorder.Code, recorder.Body.String())
		}
		assertFlutterPerformanceDashboard(t, tx, createdProjectId(t, recorder.Body.Bytes(), "project"))
	})
}

// A second project on the same template names its copy after itself instead of "Mobile Performance 2".
func TestSecondProjectDefaultDashboardIsNamedAfterTheProject(t *testing.T) {
	setupSetupControllerDB(t)
	tx, err := db.DB.Begin()
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	defer tx.Rollback()
	userId, orgId := createSetupTestAccount(t, tx, "names@example.com", "owner")

	results, err := batchCreateProjects(tx, orgId, userId, []BatchProjectInput{
		{Name: "Android App", Framework: "android"},
		{Name: "iOS App", Framework: "ios"},
	})
	if err != nil {
		t.Fatalf("batch create: %v", err)
	}
	for i, want := range []string{"Mobile Performance", "Mobile Performance · iOS App"} {
		assignments, err := transactional.DashboardRepository.FindAssignmentsByProject(tx, results[i].Project.Id)
		if err != nil || len(assignments) != 1 {
			t.Fatalf("%s assignments: %d, %v", results[i].Project.Name, len(assignments), err)
		}
		dashboard, err := transactional.DashboardRepository.FindById(tx, assignments[0].DashboardId)
		if err != nil || dashboard == nil {
			t.Fatalf("load dashboard: (%v, %v)", dashboard, err)
		}
		if dashboard.Name != want {
			t.Errorf("%s got the dashboard %q, want %q", results[i].Project.Name, dashboard.Name, want)
		}
	}
}
