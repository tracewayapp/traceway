//go:build !telemetry_ch && !transactional_pg && !telemetry_duckdb

package cmd

import (
	"testing"

	"github.com/tracewayapp/traceway/backend/v2/app/db"
	"github.com/tracewayapp/traceway/backend/v2/app/dbtest"
)

func TestSeedFlutterProjectInstallsPerformanceDashboard(t *testing.T) {
	dbtest.SetupSQLite(t)
	opts := &options{}
	WithDefaultUser("owner@example.test", "test-password")(opts)
	WithDefaultProject("Shop App", "flutter", "shop-token")(opts)
	if err := seed(opts); err != nil {
		t.Fatal(err)
	}
	var count int
	err := db.DB.QueryRow(`SELECT COUNT(*) FROM projects p
		JOIN project_dashboards pd ON pd.project_id = p.id
		JOIN dashboards d ON d.id = pd.dashboard_id
		WHERE p.name = 'Shop App' AND d.template_key = 'flutter-performance'`).Scan(&count)
	if err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("seed installed %d Flutter Performance dashboards, want 1", count)
	}
}

func TestSeedAndroidAndIOSProjectsInstallTheMobilePerformanceDashboard(t *testing.T) {
	for _, framework := range []string{"android", "ios"} {
		t.Run(framework, func(t *testing.T) {
			dbtest.SetupSQLite(t)
			opts := &options{}
			WithDefaultUser("owner@example.test", "test-password")(opts)
			WithDefaultProject("Shop App", framework, "shop-token")(opts)
			if err := seed(opts); err != nil {
				t.Fatal(err)
			}
			var count int
			err := db.DB.QueryRow(`SELECT COUNT(*) FROM projects p
				JOIN project_dashboards pd ON pd.project_id = p.id
				JOIN dashboards d ON d.id = pd.dashboard_id
				WHERE p.name = 'Shop App' AND d.template_key = 'mobile-performance'`).Scan(&count)
			if err != nil {
				t.Fatal(err)
			}
			if count != 1 {
				t.Fatalf("seed installed %d Mobile Performance dashboards for %s, want 1", count, framework)
			}
		})
	}
}

// Dashboard names are unique per organization: the second project on a template gets the template's name followed by
// its own, as in the embedded e2e server that seeds an Android and an iOS app.
func TestSeedNamesASecondDefaultDashboardAfterItsProject(t *testing.T) {
	dbtest.SetupSQLite(t)
	opts := &options{}
	WithDefaultUser("owner@example.test", "test-password")(opts)
	WithDefaultProject("Android App", "android", "android-token")(opts)
	WithDefaultProject("iOS App", "ios", "ios-token")(opts)
	WithDefaultProject("Shop", "react", "shop-token")(opts)
	WithDefaultProject("Admin", "svelte", "admin-token")(opts)
	if err := seed(opts); err != nil {
		t.Fatal(err)
	}
	want := map[string]string{
		"Android App": "Mobile Performance",
		"iOS App":     "Mobile Performance · iOS App",
		"Shop":        "Web Performance",
		"Admin":       "Web Performance · Admin",
	}
	for project, name := range want {
		var got string
		err := db.DB.QueryRow(`SELECT d.name FROM projects p
			JOIN project_dashboards pd ON pd.project_id = p.id
			JOIN dashboards d ON d.id = pd.dashboard_id
			WHERE p.name = ?`, project).Scan(&got)
		if err != nil {
			t.Fatalf("%s: %v", project, err)
		}
		if got != name {
			t.Errorf("%s got the dashboard %q, want %q", project, got, name)
		}
	}
}
