package notifications

import (
	"context"
	"database/sql"
	"fmt"
	"strconv"
	"time"

	"github.com/tracewayapp/traceway/backend/app/config"
	"github.com/tracewayapp/traceway/backend/app/db"
	"github.com/tracewayapp/traceway/backend/app/models"
	"github.com/tracewayapp/traceway/backend/app/repositories"
	traceway "go.tracewayapp.com"
)

func StartEvaluator(ctx context.Context) {
	config.Logln("Starting notification evaluator")
	seedCooldowns(ctx)
	startDedupPurger(ctx)
	registerReportHook()
	go startPolledLoop(ctx)
}

func pollInterval() time.Duration {
	seconds := 60
	if v := config.Config.NotificationPollSeconds; v != "" {
		if parsed, err := strconv.Atoi(v); err == nil && parsed >= 5 {
			seconds = parsed
		}
	}
	return time.Duration(seconds) * time.Second
}

func seedCooldowns(ctx context.Context) {
	entries, err := repositories.FiredNotificationRepository.FindLastFiredPerRule(ctx)
	if err != nil {
		traceway.CaptureException(fmt.Errorf("failed to seed notification cooldowns: %w", err))
		return
	}
	cooldowns.seed(entries)
}

func startPolledLoop(ctx context.Context) {
	defer traceway.Recover()

	ticker := time.NewTicker(pollInterval())
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			evaluatePolledRules(ctx)
		}
	}
}

func evaluatePolledRules(ctx context.Context) {
	rules, err := db.ExecuteTransaction(func(tx *sql.Tx) ([]*models.NotificationRuleWithChannel, error) {
		return repositories.NotificationRuleRepository.FindEnabledPolledRules(tx)
	})
	if err != nil {
		traceway.CaptureException(fmt.Errorf("failed to load polled notification rules: %w", err))
		return
	}

	for _, rule := range rules {
		if rule.SnoozedUntil != nil && rule.SnoozedUntil.After(time.Now()) {
			continue
		}

		if !cooldowns.canFire(rule.Id, rule.CooldownMinutes) {
			continue
		}

		evaluator, ok := polledEvaluators[rule.RuleType]
		if !ok {
			continue
		}

		nr := &models.NotificationRule{
			Id:              rule.Id,
			ProjectId:       rule.ProjectId,
			ChannelId:       rule.ChannelId,
			Name:            rule.Name,
			RuleType:        rule.RuleType,
			Config:          rule.Config,
			Enabled:         rule.Enabled,
			CooldownMinutes: rule.CooldownMinutes,
			SnoozedUntil:    rule.SnoozedUntil,
			CreatedBy:       rule.CreatedBy,
			CreatedAt:       rule.CreatedAt,
			UpdatedAt:       rule.UpdatedAt,
		}
		result, err := evaluator(ctx, nr, rule.ProjectId)
		if err != nil {
			traceway.CaptureException(fmt.Errorf("notification evaluator error (rule=%d, type=%s): %w", rule.Id, rule.RuleType, err))
			continue
		}

		if result != nil && result.Fired {
			if len(result.Messages) > 0 {
				for _, msg := range result.Messages {
					dispatch(rule, msg)
				}
			} else {
				dispatch(rule, result.Message)
			}
		}
	}
}
