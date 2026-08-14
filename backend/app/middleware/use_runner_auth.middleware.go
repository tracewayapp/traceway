package middleware

import (
	"crypto/subtle"
	"database/sql"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/tracewayapp/traceway/backend/app/config"
	"github.com/tracewayapp/traceway/backend/app/db"
	"github.com/tracewayapp/traceway/backend/app/models"
	"github.com/tracewayapp/traceway/backend/app/repositories/transactional"
	traceway "go.tracewayapp.com"
)

const RunnerContextKey = "syntheticRunner"

const runnerTouchInterval = time.Minute

// RunnerVersionHeader lets the runner binary report its version.
const RunnerVersionHeader = "X-Traceway-Runner-Version"

// RunnerNameHeader identifies the runner instance. Runners self-register
// under this name on first contact; it doubles as their claim identity.
const RunnerNameHeader = "X-Traceway-Runner-Name"

var UseRunnerAuth func(c *gin.Context)

type runnerCacheEntry struct {
	runner    *models.SyntheticRunner
	touchedAt time.Time
}

var (
	runnerCacheMu sync.Mutex
	runnerCache   = map[string]*runnerCacheEntry{}
)

// InitUseRunnerAuth authenticates runners with the shared
// SYNTHETICS_RUNNER_SECRET. Runners are operator infrastructure configured at
// deploy time, not tenant-managed identities, so there are no per-runner
// credentials: any holder of the secret may poll, and the runner
// self-registers a liveness row under its reported name (touched at most once
// a minute).
func InitUseRunnerAuth() {
	secret := config.Config.SyntheticsRunnerSecret
	UseRunnerAuth = func(c *gin.Context) {
		if secret == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Runner support is not enabled on this instance: set SYNTHETICS_RUNNER_SECRET"})
			return
		}
		header := c.GetHeader("Authorization")
		presented, ok := strings.CutPrefix(header, "Bearer ")
		if !ok || subtle.ConstantTimeCompare([]byte(presented), []byte(secret)) != 1 {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Invalid runner secret"})
			return
		}
		runner := resolveRunner(c.GetHeader(RunnerNameHeader), c.GetHeader(RunnerVersionHeader))
		c.Set(RunnerContextKey, runner)
		c.Next()
	}
}

// resolveRunner returns the liveness row for a runner name, registering or
// touching it when the cached copy is stale. A DB failure degrades to a
// transient in-memory row: liveness reporting must never block claiming.
func resolveRunner(name string, version string) *models.SyntheticRunner {
	name = strings.TrimSpace(name)
	if name == "" {
		name = "unnamed"
	}
	if len(name) > 200 {
		name = name[:200]
	}
	now := time.Now().UTC()

	runnerCacheMu.Lock()
	entry, cached := runnerCache[name]
	runnerCacheMu.Unlock()
	if cached && now.Sub(entry.touchedAt) < runnerTouchInterval &&
		(version == "" || version == entry.runner.Version) {
		return entry.runner
	}

	runner, err := db.ExecuteTransaction(func(tx *sql.Tx) (*models.SyntheticRunner, error) {
		return transactional.SyntheticRunnerRepository.UpsertSeen(tx, name, version, now)
	})
	if err != nil {
		traceway.CaptureException(traceway.NewStackTraceErrorf("failed to register runner %q: %w", name, err))
		if cached {
			return entry.runner
		}
		return &models.SyntheticRunner{Name: name, Version: version, FirstSeenAt: now, LastSeenAt: now}
	}
	runnerCacheMu.Lock()
	runnerCache[name] = &runnerCacheEntry{runner: runner, touchedAt: now}
	runnerCacheMu.Unlock()
	return runner
}

// GetRunner returns the authenticated runner set by UseRunnerAuth.
func GetRunner(c *gin.Context) (*models.SyntheticRunner, bool) {
	value, exists := c.Get(RunnerContextKey)
	if !exists {
		return nil, false
	}
	runner, ok := value.(*models.SyntheticRunner)
	return runner, ok
}
