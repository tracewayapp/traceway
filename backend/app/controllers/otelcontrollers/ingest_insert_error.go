package otelcontrollers

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/tracewayapp/traceway/backend/app/db"
	traceway "go.tracewayapp.com"
)

// abortIngestInsertError maps a full telemetry write queue to the same
// 503 + Retry-After the admission gate sends: it is load shedding, not a
// server fault, and OTLP clients retry 503. A mixed request that already
// enqueued other tables is retried whole — duplicates are acceptable under
// OTLP at-least-once semantics. Everything else stays a 500.
func abortIngestInsertError(c *gin.Context, err error, what string) {
	if errors.Is(err, db.ErrIngestQueueFull) {
		db.RecordIngestRejected()
		c.Header("Retry-After", "2")
		c.AbortWithStatusJSON(http.StatusServiceUnavailable, gin.H{"error": "ingest saturated, retry later"})
		return
	}
	c.AbortWithError(500, traceway.NewStackTraceErrorf("error inserting %s: %w", what, err))
}
