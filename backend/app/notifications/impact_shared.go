package notifications

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/tracewayapp/traceway/backend/app/models"
	"github.com/tracewayapp/traceway/backend/app/repositories"
)

const minImpactThreshold = 0.25

type impactScoreConfig struct {
	MinRequests int `json:"minRequests"`
}

type impactMessageBuilder func(endpoint string, score float64, reason string, projectName string) Message

type impactEndpointData struct {
	endpoint     string
	impact       float64
	totalCount   uint64
	p99          float64
	offsetMs     uint32
	satisfied    uint64
	tolerating   uint64
	bad          uint64
	clientErrors uint64
}

var (
	impactStateMu sync.RWMutex
	impactState   = make(map[string]map[string]bool)
)

type cachedImpact struct {
	endpoints  []impactEndpointData
	computedAt time.Time
}

var (
	impactCacheMu sync.Mutex
	impactCache   = make(map[string]cachedImpact)
)

func getImpactEndpoints(ctx context.Context, projectId uuid.UUID, minRequests int) ([]impactEndpointData, error) {
	key := fmt.Sprintf("%s:%d", projectId.String(), minRequests)

	impactCacheMu.Lock()
	cached, ok := impactCache[key]
	impactCacheMu.Unlock()
	if ok && time.Since(cached.computedAt) < 30*time.Second {
		return cached.endpoints, nil
	}

	endpoints, err := computeImpactEndpoints(ctx, projectId, minRequests)
	if err != nil {
		return nil, err
	}

	impactCacheMu.Lock()
	for k, v := range impactCache {
		if time.Since(v.computedAt) > 5*time.Minute {
			delete(impactCache, k)
		}
	}
	impactCache[key] = cachedImpact{endpoints: endpoints, computedAt: time.Now()}
	impactCacheMu.Unlock()

	return endpoints, nil
}

func evaluateImpactScore(ctx context.Context, rule *models.NotificationRule, projectId uuid.UUID, threshold float64, buildMsg impactMessageBuilder) (*EvalResult, error) {
	var cfg impactScoreConfig
	if err := json.Unmarshal(rule.Config, &cfg); err != nil {
		return nil, fmt.Errorf("invalid impact_score config: %w", err)
	}
	if cfg.MinRequests <= 0 {
		cfg.MinRequests = 50
	}

	endpoints, err := getImpactEndpoints(ctx, projectId, cfg.MinRequests)
	if err != nil {
		return nil, err
	}

	currentSet := make(map[string]impactEndpointData)
	for _, e := range endpoints {
		if e.impact >= threshold {
			currentSet[e.endpoint] = e
		}
	}

	stateKey := fmt.Sprintf("%d:%s", rule.Id, projectId.String())

	impactStateMu.RLock()
	prevSet := impactState[stateKey]
	impactStateMu.RUnlock()

	newSet := make(map[string]bool)
	for ep := range currentSet {
		newSet[ep] = true
	}

	impactStateMu.Lock()
	impactState[stateKey] = newSet
	impactStateMu.Unlock()

	if prevSet == nil {
		return &EvalResult{Fired: false}, nil
	}

	projectName := getProjectName(projectId)

	var messages []Message
	for ep, data := range currentSet {
		if prevSet[ep] {
			continue
		}
		reason := repositories.ComputeImpactReason(
			ep, data.totalCount, data.satisfied, data.tolerating,
			data.bad, data.clientErrors, data.p99, data.offsetMs,
		)
		messages = append(messages, buildMsg(ep, data.impact, reason, projectName))
	}

	if len(messages) == 0 {
		return &EvalResult{Fired: false}, nil
	}
	return &EvalResult{Fired: true, Messages: messages}, nil
}

func evaluateImpactScoreCritical(ctx context.Context, rule *models.NotificationRule, projectId uuid.UUID) (*EvalResult, error) {
	return evaluateImpactScore(ctx, rule, projectId, 0.75, buildImpactScoreCriticalMessage)
}

func evaluateImpactScoreHigh(ctx context.Context, rule *models.NotificationRule, projectId uuid.UUID) (*EvalResult, error) {
	return evaluateImpactScore(ctx, rule, projectId, 0.50, buildImpactScoreHighMessage)
}

func evaluateImpactScoreMedium(ctx context.Context, rule *models.NotificationRule, projectId uuid.UUID) (*EvalResult, error) {
	return evaluateImpactScore(ctx, rule, projectId, minImpactThreshold, buildImpactScoreMediumMessage)
}
