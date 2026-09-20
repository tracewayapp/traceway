package notifications

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/tracewayapp/traceway/backend/app/db"
	"github.com/tracewayapp/traceway/backend/app/hooks"
	"github.com/tracewayapp/traceway/backend/app/models"
	"github.com/tracewayapp/traceway/backend/app/repositories/telemetry"
	"github.com/tracewayapp/traceway/backend/app/repositories/transactional"
	traceway "go.tracewayapp.com"
)

type newErrorConfig struct {
	IgnorePatterns []string `json:"ignorePatterns"`
}

func registerReportHook() {
	hooks.RegisterReportHook(func(event hooks.ReportEvent) {
		if len(event.ExceptionHashes) == 0 && len(event.AiTraces) == 0 {
			return
		}
		go evaluateEventRules(event)
	})
}

func evaluateEventRules(event hooks.ReportEvent) {
	defer traceway.Recover()

	rules, err := db.ExecuteTransaction(func(tx *sql.Tx) ([]*models.NotificationRuleWithChannel, error) {
		return transactional.NotificationRuleRepository.FindEnabledEventRules(tx, event.ProjectId)
	})
	if err != nil {
		traceway.CaptureException(fmt.Errorf("failed to load event notification rules: %w", err))
		return
	}

	ctx := context.Background()

	for _, rule := range rules {
		if rule.SnoozedUntil != nil && rule.SnoozedUntil.After(time.Now()) {
			continue
		}

		switch rule.RuleType {
		case "new_error":
			evaluateNewError(ctx, rule, event)
		case "error_regression":
			evaluateErrorRegression(ctx, rule, event)
		case "ai_trace_cost":
			evaluateAiTraceCostEvent(rule, event)
		case "ai_conversation_cost":
			evaluateAiConversationCostEvent(ctx, rule, event)
		case "ai_flagged_content":
			evaluateAiFlaggedContentEvent(rule, event)
		}
	}
}

type aiTraceCostEventConfig struct {
	TraceName     string  `json:"traceName"`
	ThresholdCost float64 `json:"thresholdCost"`
}

func evaluateAiTraceCostEvent(rule *models.NotificationRuleWithChannel, event hooks.ReportEvent) {
	if len(event.AiTraces) == 0 {
		return
	}

	var cfg aiTraceCostEventConfig
	if err := json.Unmarshal(rule.Config, &cfg); err != nil {
		return
	}
	if cfg.ThresholdCost <= 0 {
		return
	}

	projectName := getProjectName(event.ProjectId)

	for _, at := range event.AiTraces {
		if at.TotalCost < cfg.ThresholdCost {
			continue
		}
		if cfg.TraceName != "" && cfg.TraceName != "*" && at.TraceName != cfg.TraceName {
			continue
		}

		dedupKey := aiCostDedupKey(rule.Id, at.TraceName)
		if dedup.isDuplicate(dedupKey, time.Duration(rule.CooldownMinutes)*time.Minute) {
			continue
		}
		msg := buildAiTraceCostMessage(at.TraceName, at.TotalCost, cfg.ThresholdCost, projectName)
		// Record before dispatch: a persistently failing dispatch retries once
		// per cooldown window, never on every ingest event.
		dedup.record(dedupKey)
		dispatch(rule, msg)
	}
}

type aiConversationCostConfig struct {
	ThresholdCost float64 `json:"thresholdCost"`
}

func evaluateAiConversationCostEvent(ctx context.Context, rule *models.NotificationRuleWithChannel, event hooks.ReportEvent) {
	var cfg aiConversationCostConfig
	if err := json.Unmarshal(rule.Config, &cfg); err != nil {
		return
	}
	if cfg.ThresholdCost <= 0 {
		return
	}

	cooldown := time.Duration(rule.CooldownMinutes) * time.Minute
	var candidateIds []string
	seen := map[string]struct{}{}
	for _, at := range event.AiTraces {
		if at.ConversationId == "" {
			continue
		}
		if _, dup := seen[at.ConversationId]; dup {
			continue
		}
		seen[at.ConversationId] = struct{}{}
		if dedup.isDuplicate(aiConversationCostDedupKey(rule.Id, at.ConversationId), cooldown) {
			continue
		}
		candidateIds = append(candidateIds, at.ConversationId)
	}
	if len(candidateIds) == 0 {
		return
	}

	costs, err := telemetry.AiTraceRepository.GetConversationCosts(ctx, event.ProjectId, candidateIds, time.Now().Add(-24*time.Hour))
	if err != nil {
		traceway.CaptureException(fmt.Errorf("failed to load conversation costs for notification: %w", err))
		return
	}

	projectName := getProjectName(event.ProjectId)
	for _, conversationId := range candidateIds {
		cost := costs[conversationId]
		if cost < cfg.ThresholdCost {
			continue
		}
		msg := buildAiConversationCostMessage(conversationId, cost, cfg.ThresholdCost, projectName)
		// Record before dispatch: a persistently failing dispatch retries once
		// per cooldown window, never on every ingest event.
		dedup.record(aiConversationCostDedupKey(rule.Id, conversationId))
		dispatch(rule, msg)
	}
}

type aiFlaggedContentConfig struct {
	Terms []string `json:"terms"`
}

func evaluateAiFlaggedContentEvent(rule *models.NotificationRuleWithChannel, event hooks.ReportEvent) {
	var cfg aiFlaggedContentConfig
	if err := json.Unmarshal(rule.Config, &cfg); err != nil {
		return
	}

	filter := buildTermFilter(cfg.Terms)
	cooldown := time.Duration(rule.CooldownMinutes) * time.Minute
	projectName := ""

	for _, at := range event.AiTraces {
		if !at.Flagged {
			continue
		}
		matched := matchFlaggedTerms(filter, at.FlaggedTerms)
		if len(matched) == 0 {
			continue
		}

		subject := at.ConversationId
		if subject == "" {
			subject = at.TraceName
		}
		dedupKey := aiFlaggedContentDedupKey(rule.Id, subject)
		if dedup.isDuplicate(dedupKey, cooldown) {
			continue
		}
		if projectName == "" {
			projectName = getProjectName(event.ProjectId)
		}
		msg := buildAiFlaggedContentMessage(at.ConversationId, at.UserId, matched, projectName)
		msg.DedupToken = subject
		dedup.record(dedupKey)
		dispatch(rule, msg)
	}
}

// buildTermFilter normalizes the rule's configured terms. An empty config
// means the rule fires on any flagged call.
func buildTermFilter(terms []string) map[string]struct{} {
	filter := map[string]struct{}{}
	for _, term := range terms {
		term = strings.ToLower(strings.TrimSpace(term))
		if term != "" {
			filter[term] = struct{}{}
		}
	}
	return filter
}

// matchFlaggedTerms returns the flagged terms that pass the filter, or all of
// them when the filter is empty.
func matchFlaggedTerms(filter map[string]struct{}, flaggedTerms []string) []string {
	if len(filter) == 0 {
		return flaggedTerms
	}
	var matched []string
	for _, term := range flaggedTerms {
		if _, ok := filter[term]; ok {
			matched = append(matched, term)
		}
	}
	return matched
}

func countOccurrences(hashes []string) map[string]int {
	occurrences := make(map[string]int, len(hashes))
	for _, h := range hashes {
		occurrences[h]++
	}
	return occurrences
}

func extractErrorType(stackTrace string) string {
	if stackTrace == "" {
		return "Unknown Error"
	}
	lines := strings.SplitN(stackTrace, "\n", 2)
	if len(lines) > 0 {
		line := strings.TrimSpace(lines[0])
		if idx := strings.Index(line, ":"); idx > 0 {
			return line[:idx]
		}
		return line
	}
	return "Unknown Error"
}

// resolveExceptionOwner names the endpoint, task or AI trace an exception happened in, for the notification's text.
func resolveExceptionOwner(ctx context.Context, exception models.ExceptionStackTrace) (traceType, name string) {
	owner, err := telemetry.FindExceptionOwner(ctx, exception)
	if err != nil {
		traceway.CaptureException(fmt.Errorf("failed to resolve the entity of exception %s for a notification: %w", exception.Id, err))
		return "", ""
	}
	if owner == nil {
		return "", ""
	}
	return owner.TraceType, owner.Name
}

func getProjectName(projectId uuid.UUID) string {
	project, err := db.ExecuteTransaction(func(tx *sql.Tx) (*models.Project, error) {
		return transactional.ProjectRepository.FindById(tx, projectId)
	})
	if err != nil || project == nil {
		return ""
	}
	return project.Name
}

func shouldIgnore(errorType string, patterns []string) bool {
	lower := strings.ToLower(errorType)
	for _, pattern := range patterns {
		pattern = strings.ToLower(strings.TrimSpace(pattern))
		if pattern == "" {
			continue
		}
		pattern = strings.ReplaceAll(pattern, "*", "")
		if strings.Contains(lower, pattern) {
			return true
		}
	}
	return false
}
