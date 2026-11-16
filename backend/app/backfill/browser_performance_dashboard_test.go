package backfill

import (
	"encoding/json"
	"os"
	"regexp"
	"testing"

	"github.com/tracewayapp/traceway/backend/v2/app/models"
	dashboardsvc "github.com/tracewayapp/traceway/backend/v2/app/services/dashboards"
)

func templateDefinition(t *testing.T, file string) *models.DashboardDefinition {
	t.Helper()
	sql, err := os.ReadFile("../migrations/sqlite/" + file)
	if err != nil {
		t.Fatal(err)
	}
	raw := regexp.MustCompile(`'(\{"schemaVersion.*\})'`).FindSubmatch(sql)
	if raw == nil {
		t.Fatalf("no definition in %s", file)
	}
	def, err := dashboardsvc.ParseDefinition(raw[1])
	if err != nil {
		t.Fatal(err)
	}
	return def
}

func installed(t *testing.T, def *models.DashboardDefinition) *models.Dashboard {
	t.Helper()
	dashboardsvc.EnsureWidgetIds(def)
	raw, err := dashboardsvc.MarshalDefinition(def)
	if err != nil {
		t.Fatal(err)
	}
	return &models.Dashboard{Id: 1, Definition: models.JSONText(raw)}
}

func widgetsByTitle(t *testing.T, dashboard *models.Dashboard) map[string]models.DashboardWidget {
	t.Helper()
	def, err := dashboardsvc.ParseDefinition([]byte(dashboard.Definition))
	if err != nil {
		t.Fatal(err)
	}
	out := make(map[string]models.DashboardWidget, len(def.Widgets))
	for _, widget := range def.Widgets {
		out[widget.Title] = widget
	}
	return out
}

// The seed migration is the source of the originals; if it changes, the map the
// backfill compares against must change with it.
func TestOriginalWidgetsMatchTheSeedMigration(t *testing.T) {
	original := templateDefinition(t, "0086_seed_template_browser_performance.up.sql")
	if len(original.Widgets) != len(originalBrowserPerformanceWidgets) {
		t.Fatalf("seed has %d widgets, the backfill knows %d", len(original.Widgets), len(originalBrowserPerformanceWidgets))
	}
	for _, widget := range original.Widgets {
		if !isOriginalBrowserPerformanceWidget(widget) {
			t.Errorf("seeded widget %q is not recognized as an original", widget.Title)
		}
	}
}

func TestAnUneditedDashboardBecomesTheCurrentTemplate(t *testing.T) {
	current := templateDefinition(t, "0087_update_template_browser_performance.up.sql")
	dashboard := installed(t, templateDefinition(t, "0086_seed_template_browser_performance.up.sql"))

	changed, err := upgradeBrowserPerformanceDashboard(dashboard, current.Widgets)
	if err != nil || !changed {
		t.Fatalf("changed %v err %v", changed, err)
	}
	widgets := widgetsByTitle(t, dashboard)
	if len(widgets) != len(current.Widgets) {
		t.Fatalf("got %d widgets, the template has %d", len(widgets), len(current.Widgets))
	}
	if widgets["LCP Ratings"].WidgetType != "rating_bar" || widgets["Route Change Duration by Page"].WidgetType != "breakdown_table" {
		t.Errorf("new widget types missing: %+v", widgets["LCP Ratings"])
	}
	if _, kept := widgets["Poor LCP Page Views"]; kept {
		t.Error("the widget the rating bar replaces is still there")
	}
	for title, widget := range widgets {
		if widget.Id == "" {
			t.Errorf("%q has no id", title)
		}
	}

	again, err := upgradeBrowserPerformanceDashboard(dashboard, current.Widgets)
	if err != nil || again {
		t.Errorf("a second pass changed the dashboard again (%v, %v)", again, err)
	}
}

func TestAnEditedDashboardKeepsWhatTheUserChanged(t *testing.T) {
	current := templateDefinition(t, "0087_update_template_browser_performance.up.sql")
	def := templateDefinition(t, "0086_seed_template_browser_performance.up.sql")
	dashboardsvc.EnsureWidgetIds(def)
	var lcpId, poorId string
	for i := range def.Widgets {
		switch def.Widgets[i].Title {
		case "Largest Contentful Paint (ms)":
			lcpId = def.Widgets[i].Id
		case "Poor LCP Page Views":
			poorId = def.Widgets[i].Id
		case "Interaction to Next Paint (ms)":
			def.Widgets[i].Config = json.RawMessage(`{"sources":[{"type":"metric","name":"browser.web_vital.inp","aggregation":"max"}],"showLegend":true}`)
		case "Long Tasks":
			def.Widgets[i].Title = "Blocks"
		}
	}
	def.Widgets = append(def.Widgets, models.DashboardWidget{Id: "w_mine", Title: "Checkout", WidgetType: "line_chart",
		Config: json.RawMessage(`{"sources":[{"type":"metric","name":"checkout.duration","aggregation":"avg"}]}`)})
	dashboard := installed(t, def)

	changed, err := upgradeBrowserPerformanceDashboard(dashboard, current.Widgets)
	if err != nil || !changed {
		t.Fatalf("changed %v err %v", changed, err)
	}
	widgets := widgetsByTitle(t, dashboard)
	if len(widgets) != 14 {
		t.Fatalf("an edited dashboard keeps its own widget list, got %d widgets", len(widgets))
	}
	lcp := widgets["Largest Contentful Paint"]
	if lcp.Id != lcpId || !regexp.MustCompile(`"aggregation":"p75"`).Match(lcp.Config) {
		t.Errorf("the untouched LCP widget should be upgraded in place: %s", lcp.Config)
	}
	if ratings := widgets["LCP Ratings"]; ratings.Id != poorId || ratings.WidgetType != "rating_bar" {
		t.Errorf("the poor page views widget should become the rating bar in place: %+v", ratings)
	}
	if inp := widgets["Interaction to Next Paint (ms)"]; !regexp.MustCompile(`"aggregation":"max"`).Match(inp.Config) {
		t.Errorf("an edited widget was overwritten: %s", inp.Config)
	}
	if _, ok := widgets["Blocks"]; !ok {
		t.Error("a renamed widget was replaced")
	}
	if _, ok := widgets["Checkout"]; !ok {
		t.Error("the user's own widget is gone")
	}
}
