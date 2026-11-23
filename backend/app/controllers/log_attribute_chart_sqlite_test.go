//go:build !transactional_pg && !telemetry_ch && !telemetry_duckdb

package controllers

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/tracewayapp/traceway/backend/v2/app/dbtest"
	"github.com/tracewayapp/traceway/backend/v2/app/middleware"
	"github.com/tracewayapp/traceway/backend/v2/app/models"
	"github.com/tracewayapp/traceway/backend/v2/app/repositories/telemetry"
)

func TestLogAttributeChartEndpoints(t *testing.T) {
	dbtest.SetupSQLite(t)
	gin.SetMode(gin.TestMode)
	projectId := uuid.New()
	base := time.Now().UTC().Truncate(time.Hour).Add(-time.Hour)

	var records []models.LogRecord
	for i := 0; i < 12; i++ {
		attrs := map[string]string{"TimerSend": strconv.Itoa(10 * (i + 1)), "NodeId": strconv.Itoa(i % 3)}
		if i == 11 {
			attrs["TimerSend"] = "timeout"
		}
		records = append(records, models.LogRecord{
			Id:            uuid.New(),
			ProjectId:     projectId,
			Timestamp:     base.Add(time.Duration(i) * 10 * time.Minute),
			ServiceName:   "plc-gateway",
			Body:          "telegram sent",
			LogAttributes: attrs,
		})
	}
	if err := telemetry.LogRecordRepository.InsertAsync(context.Background(), records); err != nil {
		t.Fatalf("InsertAsync: %v", err)
	}

	router := gin.New()
	scope := func(c *gin.Context) { c.Set(middleware.ProjectIdContextKey, projectId) }
	router.POST("/api/logs/attribute-series", scope, LogController.AttributeSeries)
	router.POST("/api/logs/attribute-distribution", scope, LogController.AttributeDistribution)
	call := func(path string, body map[string]any) (int, map[string]any) {
		t.Helper()
		raw, _ := json.Marshal(body)
		recorder := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, path, bytes.NewReader(raw))
		req.Header.Set("Content-Type", "application/json")
		router.ServeHTTP(recorder, req)
		var out map[string]any
		_ = json.Unmarshal(recorder.Body.Bytes(), &out)
		return recorder.Code, out
	}
	selection := func(extra map[string]any) map[string]any {
		body := map[string]any{
			"fromDate":  base.Format(time.RFC3339),
			"toDate":    base.Add(2 * time.Hour).Format(time.RFC3339),
			"attribute": map[string]any{"scope": "log", "key": "TimerSend"},
		}
		for k, v := range extra {
			body[k] = v
		}
		return body
	}

	for name, body := range map[string]map[string]any{
		"unknown scope":    selection(map[string]any{"attribute": map[string]any{"scope": "span", "key": "TimerSend"}}),
		"no key":           selection(map[string]any{"attribute": map[string]any{"scope": "log", "key": ""}}),
		"range backwards":  selection(map[string]any{"toDate": base.Add(-time.Hour).Format(time.RFC3339)}),
		"key too long":     selection(map[string]any{"attribute": map[string]any{"scope": "log", "key": string(make([]byte, 257))}}),
		"retired selector": selection(map[string]any{"excludeTraceId": "abc"}),
	} {
		for _, path := range []string{"/api/logs/attribute-series", "/api/logs/attribute-distribution"} {
			code, out := call(path, body)
			want := http.StatusUnprocessableEntity
			if name == "retired selector" {
				want = http.StatusBadRequest
			}
			if code != want {
				t.Errorf("%s %s: got %d %v, want %d", path, name, code, out, want)
			}
		}
	}

	code, series := call("/api/logs/attribute-series", selection(map[string]any{"maxBuckets": 13}))
	if code != http.StatusOK {
		t.Fatalf("series = %d %v", code, series)
	}
	if series["intervalSeconds"].(float64) != 600 || series["present"].(float64) != 12 || series["numeric"].(float64) != 11 {
		t.Errorf("series header = %v, want 10 minute buckets, 12 present and 11 numeric", series)
	}
	buckets := series["buckets"].([]any)
	if len(buckets) != 11 {
		t.Fatalf("got %d buckets, want 11 (the timeout leaves its bucket empty)", len(buckets))
	}
	if first := buckets[0].(map[string]any); first["avg"].(float64) != 10 || first["count"].(float64) != 1 {
		t.Errorf("first bucket = %v, want one value of 10", first)
	}

	code, filtered := call("/api/logs/attribute-series", selection(map[string]any{
		"attributeFilters": []map[string]any{{"scope": "log", "key": "NodeId", "value": "0"}},
	}))
	if code != http.StatusOK || filtered["numeric"].(float64) != 4 {
		t.Errorf("NodeId=0 series = %d %v, want the 4 values of node 0", code, filtered)
	}

	code, dist := call("/api/logs/attribute-distribution", selection(map[string]any{"bins": 10}))
	if code != http.StatusOK {
		t.Fatalf("distribution = %d %v", code, dist)
	}
	if dist["numeric"].(float64) != 11 || dist["min"].(float64) != 10 || dist["max"].(float64) != 110 || dist["integral"] != true {
		t.Errorf("distribution stats = %v, want 11 whole numbers from 10 to 110", dist)
	}
	if dist["avg"].(float64) != 60 || dist["p50"].(float64) != 60 {
		t.Errorf("avg %v p50 %v, want 60 and 60", dist["avg"], dist["p50"])
	}
	var total float64
	bins := dist["bins"].([]any)
	for _, b := range bins {
		total += b.(map[string]any)["count"].(float64)
	}
	if total != 11 || dist["binWidth"].(float64) != 10 || len(bins) != 11 {
		t.Errorf("bins %v width %v hold %v values, want 11 bins of 10 holding all 11", len(bins), dist["binWidth"], total)
	}

	code, empty := call("/api/logs/attribute-distribution", selection(map[string]any{"attribute": map[string]any{"scope": "resource", "key": "TimerSend"}}))
	if code != http.StatusOK || empty["numeric"].(float64) != 0 || len(empty["bins"].([]any)) != 0 {
		t.Errorf("distribution of a key no log carries = %d %v, want empty bins", code, empty)
	}
}
