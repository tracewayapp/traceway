package middleware

import (
	"errors"
	"net/http"
	"os"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/tracewayapp/traceway/backend/v2/app/traceway"
)

func BodyReadError(err error) (status int, message string, ok bool) {
	var maxBytesErr *http.MaxBytesError
	switch {
	case errors.As(err, &maxBytesErr):
		return http.StatusRequestEntityTooLarge, "Request body too large", true
	case errors.Is(err, ErrBodyTooSlow), errors.Is(err, os.ErrDeadlineExceeded):
		return http.StatusRequestTimeout, "Request body arrived too slowly", true
	}
	return 0, "", false
}

func RejectBindError(c *gin.Context, err error, fallback string) {
	status, message, ok := BodyReadError(err)
	if !ok {
		status, message = http.StatusBadRequest, fallback
	}
	c.JSON(status, gin.H{"error": message})
}

// OTLP exporters retry a 503 but treat a 408 as permanent and drop the batch,
// so a stalled ingest body is turned away as retryable. The Traceway SDKs
// re-queue a failed batch whatever the status, so this changes nothing for them.
func RejectIngestBindError(c *gin.Context, err error, fallback string) {
	reason := "invalid_body"
	var maxBytesErr *http.MaxBytesError
	if errors.As(err, &maxBytesErr) {
		reason = "body_too_large"
		traceway.SetAttribute(c, "ingest.body_limit_bytes", strconv.FormatInt(maxBytesErr.Limit, 10))
	} else if errors.Is(err, ErrBodyTooSlow) || errors.Is(err, os.ErrDeadlineExceeded) {
		reason = "body_too_slow"
	}
	traceway.SetAttribute(c, "ingest.rejection_reason", reason)
	traceway.SetAttribute(c, "ingest.content_length", strconv.FormatInt(c.Request.ContentLength, 10))
	traceway.CaptureCount("traceway.ingest.body_rejected", 1, map[string]string{"reason": reason})
	status, message, ok := BodyReadError(err)
	if !ok {
		status, message = http.StatusBadRequest, fallback
	}
	if status == http.StatusRequestTimeout {
		status = http.StatusServiceUnavailable
		c.Header("Retry-After", "2")
	}
	c.JSON(status, gin.H{"error": message})
}

// DiscardOversizedReport answers an /api/report body over the cap with 200
// instead of 413. The shipped 1.x browser SDK treats every status but 200 as a
// failure, puts the batch back in front of its queue and sends it again merged
// with whatever it collected since, so a 413 never ends: the batch only grows,
// and nothing else from that tab is ever stored. A 200 is the one answer that
// makes it let go of the batch. It reports false, having written nothing, when
// the rest of the upload cannot be read off the wire; the caller then rejects
// the request as before.
func DiscardOversizedReport(c *gin.Context, err error) bool {
	var maxBytesErr *http.MaxBytesError
	if !errors.As(err, &maxBytesErr) {
		return false
	}
	existing, ok := c.Get(ingestBodyMeterKey)
	if !ok {
		return false
	}
	meter, ok := existing.(*ingestBodyMeter)
	if !ok || !meter.discardRest() {
		return false
	}
	traceway.SetAttribute(c, "ingest.rejection_reason", "body_too_large")
	traceway.SetAttribute(c, "ingest.body_limit_bytes", strconv.FormatInt(maxBytesErr.Limit, 10))
	traceway.SetAttribute(c, "ingest.content_length", strconv.FormatInt(c.Request.ContentLength, 10))
	traceway.SetAttribute(c, "ingest.oversized_discarded", "true")
	traceway.CaptureCount("traceway.ingest.oversized_discarded", 1, nil)
	c.Header("X-Traceway-Discarded", "body_too_large")
	c.JSON(http.StatusOK, gin.H{"discarded": "body_too_large"})
	return true
}
