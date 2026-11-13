package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/tracewayapp/traceway/backend/v2/app/traceway"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
)

func TestAdmissionSpanEndsBeforeHandlerAndOnRejection(t *testing.T) {
	for _, capacity := range []int{0, 1} {
		recorder := tracetest.NewSpanRecorder()
		provider := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(recorder))
		restore := traceway.Install(traceway.Providers{TracerProvider: provider})
		ctx, parent := provider.Tracer("admission-test").Start(context.Background(), "request")
		router := gin.New()
		handlerRan := false
		router.POST("/upload", newAdmissionGate("test", capacity, 10*time.Millisecond, "busy", nil, "2"), func(c *gin.Context) {
			handlerRan = true
			ended := recorder.Ended()
			if len(ended) != 1 || ended[0].Name() != "admission.wait" {
				t.Error("wait span must end before handler starts")
			}
			c.Status(http.StatusOK)
		})
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/upload", nil).WithContext(ctx))
		ended := recorder.Ended()
		if len(ended) != 1 || ended[0].Name() != "admission.wait" {
			t.Fatalf("missing or duplicate wait span: %v", ended)
		}
		if ended[0].Parent().SpanID() != parent.SpanContext().SpanID() {
			t.Fatal("wait span detached from request")
		}
		if capacity == 0 && (rec.Code != 503 || handlerRan || ended[0].EndTime().Sub(ended[0].StartTime()) < 10*time.Millisecond) {
			t.Fatal("rejected wait must be timed without running handler")
		}
		if capacity == 1 && (rec.Code != 200 || !handlerRan) {
			t.Fatal("available slot must pass through")
		}
		parent.End()
		restore()
		_ = provider.Shutdown(context.Background())
	}
}
