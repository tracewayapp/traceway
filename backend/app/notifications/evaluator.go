package notifications

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/tracewayapp/traceway/backend/app/config"
	"github.com/tracewayapp/traceway/backend/app/db"
	"github.com/tracewayapp/traceway/backend/app/models"
	"github.com/tracewayapp/traceway/backend/app/repositories/telemetry"
	"github.com/tracewayapp/traceway/backend/app/repositories/transactional"
	"github.com/tracewayapp/traceway/backend/app/traceway"
)

func StartEvaluator(ctx context.Context) {
	config.Logln("Starting notification evaluator")
	seedCooldowns(ctx)
	startDedupPurger(ctx)
	registerReportHook()
	go startPolledLoop(ctx)
}

func pollInterval() time.Duration {
	return config.PollSeconds(config.Config.NotificationPollSeconds, 60)
}

func seedCooldowns(ctx context.Context) {
	entries, err := telemetry.FiredNotificationRepository.FindLastFiredPerRule(ctx)
	if err != nil {
		traceway.CaptureExceptionContext(ctx, fmt.Errorf("failed to seed notification cooldowns: %w", err))
	} else {
		cooldowns.seed(entries)
	}

	// Backstop: fired_notifications rows only exist once an outcome is
	// terminal, so a crash between enqueue and delivery would otherwise let
	// the rule re-fire immediately at boot while its outbox row still exists.
	enqueued, err := db.ExecuteTransactionContext(ctx, func(tx *sql.Tx) (map[int]time.Time, error) {
		return transactional.OutboxRepository.LastEnqueuedPerRule(tx)
	})
	if err != nil {
		traceway.CaptureExceptionContext(ctx, fmt.Errorf("failed to seed notification cooldowns from outbox: %w", err))
		return
	}
	cooldowns.seed(enqueued)
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
			traceway.RunTask(ctx, "notifications.evaluate", evaluatePolledRules)
		}
	}
}

func evaluatePolledRules(ctx context.Context) {
	rules, err := db.ExecuteTransactionContext(ctx, func(tx *sql.Tx) ([]*models.NotificationRuleWithChannel, error) {
		return transactional.NotificationRuleRepository.FindEnabledPolledRules(tx)
	})
	if err != nil {
		traceway.CaptureExceptionContext(ctx, fmt.Errorf("failed to load polled notification rules: %w", err))
		return
	}
	if len(rules) == 0 {
		traceway.SkipTask(ctx)
		return
	}

	for _, rule := range rules {
		if rule.SnoozedUntil != nil && rule.SnoozedUntil.After(time.Now()) {
			continue
		}

		if !statefulRuleTypes[rule.RuleType] && !isPerSeriesRule(rule.RuleType, rule.Config) && !cooldowns.canFire(rule.Id, rule.CooldownMinutes) {
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
			traceway.CaptureExceptionContext(ctx, fmt.Errorf("notification evaluator error (rule=%d, type=%s): %w", rule.Id, rule.RuleType, err))
			continue
		}

		if result != nil && result.Recovered {
			dispatchRecovery(ctx, rule, result.Message)
			continue
		}
		if result != nil && result.Fired {
			if len(result.Messages) > 0 {
				for _, msg := range result.Messages {
					dispatch(ctx, rule, msg)
				}
			} else if !dispatch(ctx, rule, result.Message) && statefulRuleTypes[rule.RuleType] {
				rollBackRuleState(ctx, rule.Id)
			}
		}
	}
}
