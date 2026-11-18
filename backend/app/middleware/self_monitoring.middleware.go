package middleware

import (
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/tracewayapp/traceway/backend/v2/app/config"
	"github.com/tracewayapp/traceway/backend/v2/app/monitoring"
	"github.com/tracewayapp/traceway/backend/v2/app/traceway"
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
	// Every browser polls for configuration, so these would dominate endpoint samples.
	"/api/otel/v1/opamp": true,
}

// Ingest requests stay endpoints, but their statements are not traced: they
// grow with customer traffic, and traceway.ingest.batch.* already times the
// writes.
var ingestRoutes = map[string]bool{
	"/api/report":                      true,
	"/api/recordings":                  true,
	"/api/profiles/ingest":             true,
	"/api/otel/v1/traces":              true,
	"/api/otel/v1/metrics":             true,
	"/api/otel/v1/logs":                true,
	"/api/otel/v1development/profiles": true,
}

func init() {
	traceway.HidePackageFrames(otelgin.Middleware)
	traceway.HideFrames(recordRequestOutcome)
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
	if ingestRoutes[c.FullPath()] {
		start := time.Now()
		defer func() {
			if r := recover(); r != nil {
				monitoring.RecordIngestOutcome(c.FullPath(), http.StatusInternalServerError, time.Since(start))
				panic(r)
			}
			monitoring.RecordIngestOutcome(c.FullPath(), c.Writer.Status(), time.Since(start))
		}()
	}

	span := trace.SpanFromContext(c.Request.Context())
	if !span.IsRecording() {
		c.Next()
		return
	}
	route := c.FullPath()
	if strings.Contains(route, ":token") {
		span.SetAttributes(attribute.String("url.path", route))
	}
	if ingestRoutes[route] {
		c.Request = c.Request.WithContext(traceway.WithoutStatementSpans(c.Request.Context()))
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

	// A stream lasts as long as the client listens (the MCP routes answer
	// POSTs as event streams), so it is kept out of latency percentiles.
	if isEventStream(c.Writer.Header().Get("Content-Type")) {
		span.SetAttributes(attribute.Bool("traceway.is_stream", true))
	}
	for _, ginErr := range c.Errors {
		traceway.RecordError(span, ginErr.Err)
	}
}

func isEventStream(contentType string) bool {
	return strings.HasPrefix(strings.ToLower(strings.TrimSpace(contentType)), "text/event-stream")
}
