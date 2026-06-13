package notifications

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/tracewayapp/traceway/backend/app/models"
)

type taskFailureRateConfig struct {
	TaskName         string  `json:"taskName"`
	ThresholdPercent float64 `json:"thresholdPercent"`
	LookbackMinutes  int     `json:"lookbackMinutes"`
	MinExecutions    int     `json:"minExecutions"`
}

func evaluateTaskFailureRate(ctx context.Context, rule *models.NotificationRule, projectId uuid.UUID) (*EvalResult, error) {
	var cfg taskFailureRateConfig
	if err := json.Unmarshal(rule.Config, &cfg); err != nil {
		return nil, fmt.Errorf("invalid task_failure_rate config: %w", err)
	}
	if cfg.LookbackMinutes <= 0 {
		cfg.LookbackMinutes = 60
	}
	if cfg.MinExecutions <= 0 {
		cfg.MinExecutions = 5
	}

	now := time.Now().UTC()
	from := now.Add(-time.Duration(cfg.LookbackMinutes) * time.Minute)
	named := cfg.TaskName != "" && cfg.TaskName != "*"

	total, err := countTaskExecutions(ctx, projectId, cfg.TaskName, named, from, now)
	if err != nil {
		return nil, err
	}
	if total < int64(cfg.MinExecutions) {
		return &EvalResult{Fired: false}, nil
	}

	failed, err := countFailedTaskExecutions(ctx, projectId, cfg.TaskName, named, from, now)
	if err != nil {
		return nil, err
	}

	rate := float64(failed) / float64(total) * 100
	if rate < cfg.ThresholdPercent {
		return &EvalResult{Fired: false}, nil
	}

	taskName := cfg.TaskName
	if taskName == "" || taskName == "*" {
		taskName = "all tasks"
	}
	projectName := getProjectName(projectId)
	msg := buildTaskFailureRateMessage(taskName, rate, cfg.ThresholdPercent, failed, total, cfg.LookbackMinutes, projectName)
	return &EvalResult{Fired: true, Message: msg}, nil
}
