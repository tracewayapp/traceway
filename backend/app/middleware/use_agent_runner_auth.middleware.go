package middleware

import (
	"database/sql"
	"encoding/json"
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

const AgentRunnerContextKey = "agentRunner"

// AgentRunnerCapabilitiesHeader carries the runner's capabilities as JSON on
// every request; they are stored on its row when they change.
const AgentRunnerCapabilitiesHeader = "X-Traceway-Runner-Capabilities"

const maxCapabilitiesBytes = 4096

var UseAgentRunnerAuth func(c *gin.Context)

type agentRunnerCacheEntry struct {
	runner       *models.AgentRunner
	touchedAt    time.Time
	capabilities string
}

var (
	agentRunnerCacheMu sync.Mutex
	agentRunnerCache   = map[string]*agentRunnerCacheEntry{}
)

// InitUseAgentRunnerAuth authenticates fix-agent runners with the shared
// AGENT_RUNNER_SECRET. Like the synthetics runners they are operator
// infrastructure: any holder of the secret may poll, and the runner
// self-registers a liveness row under its reported name, touched at most
// once a minute unless its version or capabilities changed.
func InitUseAgentRunnerAuth() {
	secret := config.Config.AgentRunnerSecret
	UseAgentRunnerAuth = func(c *gin.Context) {
		if secret == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Agent runners are not enabled on this instance: set AGENT_RUNNER_SECRET"})
			return
		}
		if !bearerMatches(c.GetHeader("Authorization"), secret) {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Invalid runner secret"})
			return
		}
		runner := resolveAgentRunner(c.GetHeader(RunnerNameHeader), c.GetHeader(RunnerVersionHeader), c.GetHeader(AgentRunnerCapabilitiesHeader))
		c.Set(AgentRunnerContextKey, runner)
		c.Next()
	}
}

func resolveAgentRunner(name string, version string, capabilities string) *models.AgentRunner {
	name = strings.TrimSpace(name)
	if name == "" {
		name = "unnamed"
	}
	if len(name) > 200 {
		name = name[:200]
	}
	if len(capabilities) > maxCapabilitiesBytes || !json.Valid([]byte(capabilities)) {
		capabilities = ""
	}
	now := time.Now().UTC()

	agentRunnerCacheMu.Lock()
	entry, cached := agentRunnerCache[name]
	agentRunnerCacheMu.Unlock()
	if cached && now.Sub(entry.touchedAt) < runnerTouchInterval &&
		(version == "" || version == entry.runner.Version) &&
		(capabilities == "" || capabilities == entry.capabilities) {
		return entry.runner
	}

	runner, err := db.ExecuteTransaction(func(tx *sql.Tx) (*models.AgentRunner, error) {
		return transactional.AgentRunnerRepository.UpsertSeen(tx, name, version, models.JSONText(capabilities), now)
	})
	if err != nil {
		traceway.CaptureException(traceway.NewStackTraceErrorf("failed to register agent runner %q: %w", name, err))
		if cached {
			return entry.runner
		}
		return &models.AgentRunner{Name: name, Version: version, FirstSeenAt: now, LastSeenAt: now}
	}
	agentRunnerCacheMu.Lock()
	agentRunnerCache[name] = &agentRunnerCacheEntry{runner: runner, touchedAt: now, capabilities: capabilities}
	agentRunnerCacheMu.Unlock()
	return runner
}

// GetAgentRunner returns the authenticated runner set by UseAgentRunnerAuth.
func GetAgentRunner(c *gin.Context) (*models.AgentRunner, bool) {
	value, exists := c.Get(AgentRunnerContextKey)
	if !exists {
		return nil, false
	}
	runner, ok := value.(*models.AgentRunner)
	return runner, ok
}
