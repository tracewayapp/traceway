package middleware

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/tracewayapp/traceway/backend/app/db"
	traceway "go.tracewayapp.com"
)

// AbortIngestSaturated is the shared load-shedding response — 503 +
// Retry-After, counted in the ingest-rejected metric — so the admission gate
// and the full-write-queue path answer with one shape.
func AbortIngestSaturated(c *gin.Context, retryAfterSeconds int) {
	db.RecordIngestRejected()
	c.Header("Retry-After", strconv.Itoa(retryAfterSeconds))
	c.AbortWithStatusJSON(http.StatusServiceUnavailable, gin.H{"error": "ingest saturated, retry later"})
}

// AbortIngestInsertError maps a full telemetry write queue to the same
// 503 + Retry-After the admission gate sends: it is load shedding, not a
// server fault, and SDKs and OTLP clients retry 503. A mixed request that
// already enqueued other tables is retried whole — duplicates are acceptable
// under at-least-once ingest semantics. Everything else stays a 500.
func AbortIngestInsertError(c *gin.Context, err error, what string) {
	if errors.Is(err, db.ErrIngestQueueFull) {
		AbortIngestSaturated(c, db.IngestQueueRetryAfterSeconds())
		return
	}
	c.AbortWithError(500, traceway.NewStackTraceErrorf("error inserting %s: %w", what, err))
}
