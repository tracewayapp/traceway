package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/tracewayapp/traceway/backend/app/config"
	"github.com/tracewayapp/traceway/backend/app/traceway"
	"go.opentelemetry.io/contrib/instrumentation/github.com/gin-gonic/gin/otelgin"
	"go.opentelemetry.io/otel/attribute"
	metricnoop "go.opentelemetry.io/otel/metric/noop"
	"go.opentelemetry.io/otel/trace"
)

var untracedRoutes = map[string]bool{
	"/assets/*filepath": true,
	"/_app/*filepath":   true,
	"/favicon.ico":      true,
	"/robots.txt":       true,
	// Held open for up to 25s by design; as endpoint samples they would
	// dominate every latency percentile.
	"/api/runners/poll": true,
}

// SelfMonitoring traces the backend's own requests into the self-monitoring
// project. Requests that carry that project's token are never traced: they
// are the exports of this very tracer, and tracing them would report every
// report back to itself without end.
func SelfMonitoring(selfToken string) []gin.HandlerFunc {
	selfAuth := "Bearer " + selfToken
	return []gin.HandlerFunc{
		otelgin.Middleware(traceway.ServiceName(),
			otelgin.WithTracerProvider(traceway.TracerProvider()),
			otelgin.WithPropagators(traceway.Propagator()),
			// Endpoints are built from the spans; per-route duration
			// histograms would only add metric series nobody reads.
			otelgin.WithMeterProvider(metricnoop.NewMeterProvider()),
			otelgin.WithGinFilter(func(c *gin.Context) bool {
				return shouldTraceRequest(c, selfAuth)
			}),
		),
		recordRequestOutcome,
	}
}

func shouldTraceRequest(c *gin.Context, selfAuth string) bool {
	if c.GetHeader("Authorization") == selfAuth {
		return false
	}
	route := c.FullPath()
	if route == "" || untracedRoutes[route] {
		return false
	}
	if c.Request.Method == http.MethodGet && (route == "/mcp" || strings.HasPrefix(route, "/mcp/")) {
		return false
	}
	return true
}

// recordRequestOutcome runs inside the request span: gin's own Recovery sits
// outside it, so without this a panic would leave the span with no status
// and no exception.
func recordRequestOutcome(c *gin.Context) {
	span := trace.SpanFromContext(c.Request.Context())
	if !span.IsRecording() {
		c.Next()
		return
	}
	if route := c.FullPath(); strings.Contains(route, ":token") {
		span.SetAttributes(attribute.String("url.path", route))
	}

	defer func() {
		r := recover()
		if r == nil {
			return
		}
		if r == http.ErrAbortHandler {
			panic(r)
		}
		formatted := traceway.RecordPanic(span, r)
		config.LogfContext(c, "ERROR: panic recovered: %s", formatted)
		if c.Writer.Written() {
			c.Abort()
		} else {
			c.AbortWithStatus(http.StatusInternalServerError)
		}
	}()

	c.Next()

	for _, ginErr := range c.Errors {
		traceway.RecordError(span, ginErr.Err)
	}
}
