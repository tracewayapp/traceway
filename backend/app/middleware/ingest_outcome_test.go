package middleware

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/tracewayapp/traceway/backend/v2/app/traceway"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
)

func TestIngestOutcomesIncludeUnsampledFailuresAndPanics(t *testing.T) {
	reader := sdkmetric.NewManualReader()
	provider := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))
	t.Cleanup(func() { _ = provider.Shutdown(context.Background()) })
	t.Cleanup(traceway.Install(traceway.Providers{MeterProvider: provider}))
	router := gin.New()
	router.Use(gin.RecoveryWithWriter(io.Discard), recordRequestOutcome)
	router.POST("/api/otel/v1/logs", func(c *gin.Context) {
		switch c.Query("outcome") {
		case "panic":
			panic("storage unavailable")
		case "invalid":
			c.AbortWithStatus(http.StatusBadRequest)
		default:
			c.Status(http.StatusOK)
		}
	})
	for _, outcome := range []string{"ok", "invalid", "panic"} {
		recorder := httptest.NewRecorder()
		router.ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, "/api/otel/v1/logs?outcome="+outcome, nil))
	}
	var result metricdata.ResourceMetrics
	if err := reader.Collect(context.Background(), &result); err != nil {
		t.Fatal(err)
	}
	statuses := map[string]int64{}
	for _, scope := range result.ScopeMetrics {
		for _, m := range scope.Metrics {
			if m.Name != "traceway.ingest.requests" {
				continue
			}
			for _, point := range m.Data.(metricdata.Sum[int64]).DataPoints {
				status, _ := point.Attributes.Value("status_class")
				statuses[status.AsString()] += point.Value
				if point.Attributes.Len() != 2 {
					t.Fatalf("unexpected metric labels: %v", point.Attributes)
				}
			}
		}
	}
	for _, status := range []string{"2xx", "4xx", "5xx"} {
		if statuses[status] != 1 {
			t.Fatalf("outcomes=%v", statuses)
		}
	}
}
