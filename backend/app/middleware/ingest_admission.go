package middleware

import (
	"os"
	"runtime"
	"strconv"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

// The telemetry ingest path decompresses and decodes every request body it
// accepts; without a bound, a burst of fat OTLP batches holds decoded copies
// of all of them in memory at once and the kernel OOM-kills the process
// (observed on 8 GB hosts under sustained log ingest — and on the DuckDB
// backend the death is followed by a WAL-replay stall on restart during
// which nothing answers). The gate caps concurrent ingest processing. The
// slot is acquired before the body is read, so a waiting request holds only
// a connection; a request that cannot get a slot within the wait window is
// rejected with 503 + Retry-After, which SDKs and OTLP collectors retry.
func newIngestAdmission(capacity int, wait time.Duration) gin.HandlerFunc {
	slots := make(chan struct{}, capacity)
	return func(c *gin.Context) {
		select {
		case slots <- struct{}{}:
		default:
			timer := time.NewTimer(wait)
			defer timer.Stop()
			select {
			case slots <- struct{}{}:
			case <-timer.C:
				AbortIngestSaturated(c, 2)
				return
			case <-c.Request.Context().Done():
				c.Abort()
				return
			}
		}
		defer func() { <-slots }()
		c.Next()
	}
}

func ingestCapacityFromEnv() int {
	n := runtime.NumCPU() * 2
	if n < 4 {
		n = 4
	}
	if v := os.Getenv("INGEST_MAX_CONCURRENT"); v != "" {
		if parsed, err := strconv.Atoi(v); err == nil && parsed > 0 {
			n = parsed
		}
	}
	return n
}

func ingestWaitFromEnv() time.Duration {
	if v := os.Getenv("INGEST_ADMISSION_WAIT_SECONDS"); v != "" {
		if parsed, err := strconv.Atoi(v); err == nil && parsed >= 0 {
			return time.Duration(parsed) * time.Second
		}
	}
	return 5 * time.Second
}

// The gate is built on first use, not at package init: godotenv.Load() runs
// inside run(), after package-level vars initialize, so reading the env here
// eagerly would silently ignore INGEST_* values set via a .env file.
var ingestAdmission = sync.OnceValue(func() gin.HandlerFunc {
	return newIngestAdmission(ingestCapacityFromEnv(), ingestWaitFromEnv())
})

func IngestAdmission(c *gin.Context) {
	ingestAdmission()(c)
}
