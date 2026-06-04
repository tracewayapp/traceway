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
	"github.com/tracewayapp/traceway/backend/app/repositories"
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
		return repositories.NotificationRuleRepository.FindEnabledEventRules(tx, event.ProjectId)
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

		dedupKey := fmt.Sprintf("ai_cost:%d:%s", rule.Id, at.TraceName)
		if dedup.isDuplicate(dedupKey, time.Duration(rule.CooldownMinutes)*time.Minute) {
			continue
		}
		dedup.record(dedupKey)

		msg := buildAiTraceCostMessage(at.TraceName, at.TotalCost, cfg.ThresholdCost, 0, projectName)
		dispatch(rule, msg)
	}
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

func resolveTraceName(ctx context.Context, projectId uuid.UUID, traceId *uuid.UUID, traceType string) string {
	if traceId == nil {
		return ""
	}
	switch traceType {
	case "task":
		task, err := repositories.TaskRepository.FindById(ctx, projectId, *traceId)
		if err != nil {
			traceway.CaptureException(fmt.Errorf("failed to resolve task name for notification: %w", err))
			return ""
		}
		if task == nil {
			return ""
		}
		return task.TaskName
	case "ai_trace":
		trace, err := repositories.AiTraceRepository.FindById(ctx, projectId, *traceId)
		if err != nil {
			traceway.CaptureException(fmt.Errorf("failed to resolve ai trace name for notification: %w", err))
			return ""
		}
		if trace == nil {
			return ""
		}
		return trace.TraceName
	default:
		endpoint, err := repositories.EndpointRepository.FindById(ctx, projectId, *traceId)
		if err != nil {
			traceway.CaptureException(fmt.Errorf("failed to resolve endpoint name for notification: %w", err))
			return ""
		}
		if endpoint == nil {
			return ""
		}
		return endpoint.Endpoint
	}
}

func getProjectName(projectId uuid.UUID) string {
	project, err := db.ExecuteTransaction(func(tx *sql.Tx) (*models.Project, error) {
		return repositories.ProjectRepository.FindById(tx, projectId)
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
