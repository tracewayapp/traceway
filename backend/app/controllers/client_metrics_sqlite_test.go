//go:build !transactional_pg && !telemetry_ch && !telemetry_duckdb

package controllers

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/tracewayapp/traceway/backend/v2/app/cache"
	"github.com/tracewayapp/traceway/backend/v2/app/config"
	"github.com/tracewayapp/traceway/backend/v2/app/controllers/otelcontrollers"
	"github.com/tracewayapp/traceway/backend/v2/app/db"
	"github.com/tracewayapp/traceway/backend/v2/app/dbtest"
	"github.com/tracewayapp/traceway/backend/v2/app/middleware"
	"github.com/tracewayapp/traceway/backend/v2/app/models"
	"github.com/tracewayapp/traceway/backend/v2/app/repositories/telemetry/shared"
	"github.com/tracewayapp/traceway/backend/v2/app/repositories/transactional"
	"github.com/tracewayapp/traceway/backend/v2/app/services"
)

func setupClientMetrics(t *testing.T, framework string) (*gin.Engine, uuid.UUID) {
	t.Helper()
	dbtest.SetupSQLite(t)
	previous := config.Config
	config.Config = &config.Cfg{}
	t.Cleanup(func() { config.Config = previous })
	gin.SetMode(gin.TestMode)
	project, err := db.ExecuteTransaction(func(tx *sql.Tx) (*models.Project, error) {
		return transactional.ProjectRepository.Create(tx, "client-metrics-test", framework)
	})
	if err != nil {
		t.Fatal(err)
	}
	projectId := project.Id
	withProject := func(c *gin.Context) {
		c.Set(middleware.ProjectIdContextKey, projectId)
		c.Set(middleware.ProjectContextKey, project)
	}
	router := gin.New()
	router.POST("/api/otel/v1/metrics", withProject, otelcontrollers.OtelController.ExportMetrics)
	router.POST("/api/metrics/query", withProject, MetricQueryController.Query)
	router.POST("/api/metrics/exemplars", withProject, MetricQueryController.Exemplars)
	router.GET("/api/metrics/discover", withProject, MetricQueryController.Discover)
	router.GET("/api/metrics/discover/tags", withProject, MetricQueryController.DiscoverTags)
	return router, projectId
}

// lcpPayload is what the browser SDK uploads for one page view: a delta exponential
// histogram at scale 3 and a counter.
func lcpPayload(release, path, rating string, value float64, session string) string {
	at := time.Now().UnixMilli()
	index := shared.HistogramBucketIndex(value)
	exemplar := ""
	if session != "" {
		exemplar = fmt.Sprintf(`,"exemplars":[{"timeUnixNano":"%d000000","asDouble":%v,"filteredAttributes":[{"key":"session.id","value":{"stringValue":%q}}]}]`, at, value, session)
	}
	return fmt.Sprintf(`{"resourceMetrics":[{"resource":{"attributes":[
		{"key":"service.name","value":{"stringValue":"shop"}},
		{"key":"service.version","value":{"stringValue":%q}},
		{"key":"traceway.metrics","value":{"stringValue":"aggregated"}},
		{"key":"traceway.sample_rate","value":{"doubleValue":1}},
		{"key":"traceway.sent_at","value":{"doubleValue":%d}}]},
	"scopeMetrics":[{"scope":{"name":"@tracewayapp/frontend","version":"2.1.0"},"metrics":[
		{"name":"browser.web_vital.lcp","unit":"ms","exponentialHistogram":{"aggregationTemporality":1,"dataPoints":[{
			"attributes":[{"key":"url.path","value":{"stringValue":%q}},{"key":"rating","value":{"stringValue":%q}}],
			"startTimeUnixNano":"%d000000","timeUnixNano":"%d000000","count":1,"sum":%v,"scale":3,"zeroCount":0,"min":%v,"max":%v,
			"positive":{"offset":%d,"bucketCounts":[1]}%s}]}},
		{"name":"browser.page_view","sum":{"aggregationTemporality":1,"isMonotonic":true,"dataPoints":[{
			"attributes":[{"key":"url.path","value":{"stringValue":%q}}],"startTimeUnixNano":"%d000000","timeUnixNano":"%d000000","asDouble":1}]}}
	]}]}]}`, release, at, path, rating, at-1000, at, value, value, value, index, exemplar, path, at-1000, at)
}

func postJSON(t *testing.T, router *gin.Engine, path, body string, out any) int {
	t.Helper()
	recorder := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, path, bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(recorder, req)
	if out != nil && recorder.Code == http.StatusOK {
		if err := json.Unmarshal(recorder.Body.Bytes(), out); err != nil {
			t.Fatalf("%s: %v: %s", path, err, recorder.Body.String())
		}
	}
	return recorder.Code
}

func queryMetric(t *testing.T, router *gin.Engine, item string) (int, MetricQueryResult) {
	t.Helper()
	from, to := time.Now().Add(-time.Hour).UTC().Format(time.RFC3339), time.Now().Add(time.Hour).UTC().Format(time.RFC3339)
	var resp MetricQueryResponse
	code := postJSON(t, router, "/api/metrics/query", fmt.Sprintf(`{"queries":[%s],"from":%q,"to":%q,"intervalMinutes":240}`, item, from, to), &resp)
	if code != http.StatusOK || len(resp.Results) == 0 {
		return code, MetricQueryResult{}
	}
	return code, resp.Results[0]
}

func seriesTotal(result MetricQueryResult, group string) float64 {
	var total float64
	for _, point := range result.Series[group] {
		total += point.Value
	}
	return total
}

func TestClientMetrics_FromUploadToPercentile(t *testing.T) {
	router, _ := setupClientMetrics(t, "react")

	// 100 page views of release 4.2: 75 fast, 25 slow. 20 of release 4.1.
	session := uuid.NewString()
	for i := 0; i < 100; i++ {
		value, rating, exemplar := 1200.0, "good", ""
		if i%4 == 3 {
			value, rating, exemplar = 6000.0, "poor", session
		}
		if code := postJSON(t, router, "/api/otel/v1/metrics", lcpPayload("4.2.0", "/orders/:id", rating, value, exemplar), nil); code != http.StatusOK {
			t.Fatalf("upload %d answered %d", i, code)
		}
	}
	for i := 0; i < 20; i++ {
		postJSON(t, router, "/api/otel/v1/metrics", lcpPayload("4.1.0", "/cart", "good", 900, ""), nil)
	}
	if err := services.ClientMetrics().Flush(context.Background()); err != nil {
		t.Fatal(err)
	}

	code, p75 := queryMetric(t, router, `{"name":"browser.web_vital.lcp","aggregation":"p75","tagFilters":{"service.version":"4.2.0"}}`)
	if code != http.StatusOK || p75.Unit != "ms" {
		t.Fatalf("p75: code %d unit %q", code, p75.Unit)
	}
	if got := p75.Series["__all__"][0].Value; math.Abs(got-1200)/1200 > 0.06 {
		t.Errorf("p75 of release 4.2 = %v, want about 1200", got)
	}
	_, p95 := queryMetric(t, router, `{"name":"browser.web_vital.lcp","aggregation":"p95","tagFilters":{"service.version":"4.2.0"}}`)
	if got := p95.Series["__all__"][0].Value; math.Abs(got-6000)/6000 > 0.06 {
		t.Errorf("p95 of release 4.2 = %v, want about 6000", got)
	}

	// A stat over the whole range is one value, read from the merged buckets.
	var whole MetricQueryResponse
	from, to := time.Now().Add(-26*time.Hour).UTC().Format(time.RFC3339), time.Now().Add(time.Hour).UTC().Format(time.RFC3339)
	if code := postJSON(t, router, "/api/metrics/query", fmt.Sprintf(`{"queries":[{"name":"browser.web_vital.lcp","aggregation":"p95","groupBy":"service.version"}],"from":%q,"to":%q,"wholeRange":true}`, from, to), &whole); code != http.StatusOK {
		t.Fatalf("whole range answered %d", code)
	}
	if points := whole.Results[0].Series["4.2.0"]; len(points) != 1 || math.Abs(points[0].Value-6000)/6000 > 0.06 {
		t.Errorf("whole range p95 of 4.2: %+v", points)
	}

	// The catalog knows what a good LCP is.
	_, good := queryMetric(t, router, `{"name":"browser.web_vital.lcp","aggregation":"good_ratio","tagFilters":{"service.version":"4.2.0"}}`)
	if got := good.Series["__all__"][0].Value; math.Abs(got-0.75) > 0.01 || good.Unit != "1" {
		t.Errorf("good ratio = %v (%q), want 0.75", got, good.Unit)
	}

	_, byRelease := queryMetric(t, router, `{"name":"browser.web_vital.lcp","aggregation":"count","groupBy":"service.version"}`)
	if seriesTotal(byRelease, "4.2.0") != 100 || seriesTotal(byRelease, "4.1.0") != 20 || byRelease.Unit != "count" {
		t.Errorf("count by release: %+v", byRelease.Series)
	}
	_, byRating := queryMetric(t, router, `{"name":"browser.web_vital.lcp","aggregation":"count","groupBy":"rating"}`)
	if seriesTotal(byRating, "good") != 95 || seriesTotal(byRating, "poor") != 25 {
		t.Errorf("count by rating: %+v", byRating.Series)
	}

	// Counters travel as points, one row per series per flush, and sum as before.
	_, views := queryMetric(t, router, `{"name":"browser.page_view","aggregation":"sum","groupBy":"url.path"}`)
	if seriesTotal(views, "/orders/:id") != 100 || seriesTotal(views, "/cart") != 20 {
		t.Errorf("page views: %+v", views.Series)
	}

	for name, item := range map[string]string{
		"percentile of a counter":   `{"name":"browser.page_view","aggregation":"p75"}`,
		"last of a distribution":    `{"name":"browser.web_vital.lcp","aggregation":"last"}`,
		"an aggregation we lack":    `{"name":"browser.web_vital.lcp","aggregation":"median"}`,
		"good_ratio with no target": `{"name":"browser.web_vital.lcp.custom","aggregation":"good_ratio"}`,
	} {
		if code, _ := queryMetric(t, router, item); code != http.StatusUnprocessableEntity {
			t.Errorf("%s answered %d, want 422", name, code)
		}
	}

	// A dashboard is installed before its data: a catalog metric nothing reported yet is an empty chart.
	if code, empty := queryMetric(t, router, `{"name":"browser.page_load.duration","aggregation":"p75"}`); code != http.StatusOK || len(empty.Series) != 0 || empty.Unit != "ms" {
		t.Errorf("a distribution without data answered %d with %+v", code, empty)
	}

	var exemplars struct {
		Exemplars []shared.MetricExemplar `json:"exemplars"`
	}
	from, to = time.Now().Add(-time.Hour).UTC().Format(time.RFC3339), time.Now().Add(time.Hour).UTC().Format(time.RFC3339)
	if code := postJSON(t, router, "/api/metrics/exemplars", fmt.Sprintf(`{"name":"browser.web_vital.lcp","from":%q,"to":%q,"tagFilters":{"rating":"poor"}}`, from, to), &exemplars); code != http.StatusOK {
		t.Fatalf("exemplars answered %d", code)
	}
	if len(exemplars.Exemplars) == 0 || len(exemplars.Exemplars) > 3 || exemplars.Exemplars[0].SessionId != session {
		t.Errorf("exemplars: %+v", exemplars.Exemplars)
	}

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/metrics/discover", nil))
	var discovered DiscoverResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &discovered); err != nil {
		t.Fatal(err)
	}
	types := map[string]string{}
	keys := map[string][]string{}
	for _, m := range discovered.Metrics {
		types[m.Name], keys[m.Name] = m.MetricType, m.TagKeys
	}
	if types["browser.web_vital.lcp"] != models.MetricTypeHistogram || types["browser.page_view"] != models.MetricTypeCounter {
		t.Errorf("discovered types: %v", types)
	}
	if got := strings.Join(keys["browser.web_vital.lcp"], ","); got != "rating,server_name,service.version,url.path" {
		t.Errorf("lcp dimensions: %s", got)
	}

	recorder = httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/metrics/discover/tags?name=browser.web_vital.lcp&key=service.version", nil))
	if body := recorder.Body.String(); !strings.Contains(body, `["4.1.0","4.2.0"]`) {
		t.Errorf("release values: %s", body)
	}
}

func TestClientMetrics_OversizedUploadIsRefused(t *testing.T) {
	router, _ := setupClientMetrics(t, "ios")
	body := `{"resourceMetrics":[],"padding":"` + strings.Repeat("x", 70<<10) + `"}`
	if code := postJSON(t, router, "/api/otel/v1/metrics", body, nil); code != http.StatusRequestEntityTooLarge {
		t.Errorf("a 70 KB client upload answered %d, want 413", code)
	}
}

func TestBackendHistogramsAnswerPercentiles(t *testing.T) {
	router, _ := setupClientMetrics(t, "opentelemetry")
	at := time.Now().UnixNano()
	body := fmt.Sprintf(`{"resourceMetrics":[{"resource":{"attributes":[{"key":"service.name","value":{"stringValue":"api"}}]},"scopeMetrics":[{"metrics":[
		{"name":"http.server.request.duration","unit":"ms","histogram":{"aggregationTemporality":1,"dataPoints":[{
			"attributes":[{"key":"http.route","value":{"stringValue":"/users"}}],"timeUnixNano":"%d","count":"100","sum":3000,
			"explicitBounds":[10,25,50,100],"bucketCounts":["10","30","40","15","5"]}]}}]}]}]}`, at)
	if code := postJSON(t, router, "/api/otel/v1/metrics", body, nil); code != http.StatusOK {
		t.Fatalf("upload answered %d", code)
	}
	if err := services.ClientMetrics().Flush(context.Background()); err != nil {
		t.Fatal(err)
	}
	code, p50 := queryMetric(t, router, `{"name":"http.server.request.duration","aggregation":"p50","groupBy":"http.route"}`)
	if code != http.StatusOK {
		t.Fatalf("p50 answered %d", code)
	}
	if got := p50.Series["/users"][0].Value; got < 25 || got > 50 {
		t.Errorf("p50 = %v, want inside the 25..50 bucket", got)
	}
	// Installed widgets still find the flattened series.
	if code, avg := queryMetric(t, router, `{"name":"http.server.request.duration.avg","aggregation":"avg"}`); code != http.StatusOK || avg.Series["__all__"][0].Value != 30 {
		t.Errorf(".avg series: code %d %+v", code, avg.Series)
	}
}

// A project upgraded from the release before histograms still has its web vitals
// registered as gauges until a client reports again. Its dashboard must not answer 422.
func TestClientMetrics_GaugeRowOfAnEarlierReleaseStillAnswersPercentiles(t *testing.T) {
	router, projectId := setupClientMetrics(t, "react")
	project, err := db.ExecuteTransaction(func(tx *sql.Tx) (*models.Project, error) {
		if _, err := tx.Exec(
			"INSERT INTO metric_registry (project_id, name, metric_type, unit) VALUES (?, 'browser.web_vital.lcp', 'gauge', 'ms')",
			projectId.String(),
		); err != nil {
			return nil, err
		}
		return transactional.ProjectRepository.FindById(tx, projectId)
	})
	if err != nil {
		t.Fatal(err)
	}
	cache.ProjectCache.AddProject(project)
	t.Cleanup(func() { cache.ProjectCache.RemoveProject(projectId) })

	code, p75 := queryMetric(t, router, `{"name":"browser.web_vital.lcp","aggregation":"p75"}`)
	if code != http.StatusOK {
		t.Fatalf("p75 of a metric registered as a gauge answered %d, want 200", code)
	}
	if len(p75.Series["__all__"]) != 0 {
		t.Errorf("no histogram is stored yet, got %+v", p75.Series)
	}
}
