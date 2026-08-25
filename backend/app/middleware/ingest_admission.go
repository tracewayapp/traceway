package middleware

import (
	"net/http"
	"os"
	"runtime"
	"strconv"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/tracewayapp/traceway/backend/app/db"
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
	return newAdmissionGate(capacity, wait, "ingest saturated, retry later", db.RecordIngestRejected)
}

func newAdmissionGate(capacity int, wait time.Duration, message string, onReject func()) gin.HandlerFunc {
	slots := make(chan struct{}, capacity)
	waiters := make(chan struct{}, max(4*capacity, 16))
	reject := func(c *gin.Context) {
		if onReject != nil {
			onReject()
		}
		c.Header("Retry-After", "2")
		c.AbortWithStatusJSON(http.StatusServiceUnavailable, gin.H{"error": message})
	}
	return func(c *gin.Context) {
		select {
		case slots <- struct{}{}:
		default:
			select {
			case waiters <- struct{}{}:
			default:
				reject(c)
				return
			}
			timer := time.NewTimer(wait)
			select {
			case slots <- struct{}{}:
				<-waiters
				timer.Stop()
			case <-timer.C:
				<-waiters
				reject(c)
				return
			case <-c.Request.Context().Done():
				<-waiters
				timer.Stop()
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

var uploadAdmission = sync.OnceValue(func() gin.HandlerFunc {
	return newAdmissionGate(uploadCapacityFromEnv(), 30*time.Second, "upload queue saturated, retry later", nil)
})

func uploadCapacityFromEnv() int {
	n := 4
	if v := os.Getenv("UPLOAD_MAX_CONCURRENT"); v != "" {
		if parsed, err := strconv.Atoi(v); err == nil && parsed > 0 {
			n = parsed
		}
	}
	return n
}

func UploadAdmission(c *gin.Context) {
	uploadAdmission()(c)
}
