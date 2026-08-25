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

type admissionGate struct {
	slots    chan struct{}
	waiters  chan struct{}
	wait     time.Duration
	message  string
	onReject func()
	keyOf    func(*gin.Context) string
	perKey   int
	keyed    keyedSlots
}

func newAdmissionGate(capacity int, wait time.Duration, message string, onReject func()) gin.HandlerFunc {
	g := &admissionGate{
		slots:    make(chan struct{}, capacity),
		waiters:  make(chan struct{}, maxWaitersFor(capacity)),
		wait:     wait,
		message:  message,
		onReject: onReject,
		keyOf:    projectAdmissionKey,
		perKey:   perKeyMaxFor(capacity),
	}
	return g.handle
}

func maxWaitersFor(capacity int) int {
	n := 4 * capacity
	if n < 16 {
		n = 16
	}
	return n
}

func perKeyMaxFor(capacity int) int {
	reserve := capacity / 4
	if reserve < 1 {
		reserve = 1
	}
	n := capacity - reserve
	if n < 1 {
		n = 1
	}
	return n
}

func projectAdmissionKey(c *gin.Context) string {
	if id, err := GetProjectId(c); err == nil {
		return id.String()
	}
	return ""
}

func (g *admissionGate) handle(c *gin.Context) {
	deadline := time.Now().Add(g.wait)

	if g.keyOf != nil && g.perKey > 0 {
		if key := g.keyOf(c); key != "" {
			keySlots := g.keyed.ref(key, g.perKey)
			defer g.keyed.unref(key)
			if !g.acquire(c, keySlots, deadline) {
				return
			}
			defer func() { <-keySlots }()
		}
	}

	if !g.acquire(c, g.slots, deadline) {
		return
	}
	defer func() { <-g.slots }()
	c.Next()
}

func (g *admissionGate) acquire(c *gin.Context, ch chan struct{}, deadline time.Time) bool {
	select {
	case ch <- struct{}{}:
		return true
	default:
	}

	select {
	case g.waiters <- struct{}{}:
	default:
		g.reject(c)
		return false
	}
	defer func() { <-g.waiters }()

	timer := time.NewTimer(time.Until(deadline))
	defer timer.Stop()
	select {
	case ch <- struct{}{}:
		return true
	case <-timer.C:
		g.reject(c)
		return false
	case <-c.Request.Context().Done():
		c.Abort()
		return false
	}
}

func (g *admissionGate) reject(c *gin.Context) {
	if g.onReject != nil {
		g.onReject()
	}
	c.Header("Retry-After", "2")
	c.AbortWithStatusJSON(http.StatusServiceUnavailable, gin.H{"error": g.message})
}

type keyedSlots struct {
	mu   sync.Mutex
	keys map[string]*keySlotsEntry
}

type keySlotsEntry struct {
	ch   chan struct{}
	refs int
}

func (k *keyedSlots) ref(key string, size int) chan struct{} {
	k.mu.Lock()
	defer k.mu.Unlock()
	if k.keys == nil {
		k.keys = map[string]*keySlotsEntry{}
	}
	entry := k.keys[key]
	if entry == nil {
		entry = &keySlotsEntry{ch: make(chan struct{}, size)}
		k.keys[key] = entry
	}
	entry.refs++
	return entry.ch
}

func (k *keyedSlots) unref(key string) {
	k.mu.Lock()
	defer k.mu.Unlock()
	entry := k.keys[key]
	if entry == nil {
		return
	}
	entry.refs--
	if entry.refs <= 0 {
		delete(k.keys, key)
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
