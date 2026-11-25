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
