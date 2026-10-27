package notifications

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/tracewayapp/traceway/backend/app/db"
	"github.com/tracewayapp/traceway/backend/app/models"
	"github.com/tracewayapp/traceway/backend/app/repositories/transactional"
	"github.com/tracewayapp/traceway/backend/app/slo"
	traceway "go.tracewayapp.com"
)

var statefulRuleTypes = map[string]bool{
	"slo_burn_rate":       true,
	"slo_budget_consumed": true,
}

const (
	defaultBurnLongWindowMinutes  = 60
	defaultBurnShortWindowMinutes = 5
	defaultBurnRateThreshold      = 14.4
	defaultBudgetConsumedPercent  = 80
)

func SLODedupToken(sloId int) string {
	return fmt.Sprintf("slo:%d", sloId)
}

type sloBurnRateConfig struct {
	SloId              int     `json:"sloId"`
	LongWindowMinutes  int     `json:"longWindowMinutes"`
	ShortWindowMinutes int     `json:"shortWindowMinutes"`
	BurnRateThreshold  float64 `json:"burnRateThreshold"`
}

type sloBudgetConsumedConfig struct {
	SloId           int     `json:"sloId"`
	ConsumedPercent float64 `json:"consumedPercent"`
}

func loadRuleSLO(sloId int, projectId uuid.UUID) (*models.SLO, error) {
	entity, err := db.ExecuteTransaction(func(tx *sql.Tx) (*models.SLO, error) {
		return transactional.SLORepository.FindByIdForProject(tx, sloId, projectId)
	})
	if err != nil {
		return nil, err
	}
	if entity == nil {
		return nil, fmt.Errorf("SLO %d not found in project", sloId)
	}
	return entity, nil
}

func evaluateSLOBurnRate(ctx context.Context, rule *models.NotificationRule, projectId uuid.UUID) (*EvalResult, error) {
	var cfg sloBurnRateConfig
	if err := json.Unmarshal(rule.Config, &cfg); err != nil {
		return nil, fmt.Errorf("invalid slo_burn_rate config: %w", err)
	}
	if cfg.LongWindowMinutes <= 0 {
		cfg.LongWindowMinutes = defaultBurnLongWindowMinutes
	}
	if cfg.ShortWindowMinutes <= 0 {
		cfg.ShortWindowMinutes = defaultBurnShortWindowMinutes
	}
	if cfg.BurnRateThreshold <= 0 {
		cfg.BurnRateThreshold = defaultBurnRateThreshold
	}
	entity, err := loadRuleSLO(cfg.SloId, projectId)
	if err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	if !entity.Enabled {
		return recoverDisabledSLO(rule, entity, projectId, now)
	}
	if entity.LastBucketAt == nil {
		return &EvalResult{}, nil
	}
	cfg.LongWindowMinutes = sliceAligned(cfg.LongWindowMinutes, entity.BucketMinutes)
	cfg.ShortWindowMinutes = sliceAligned(cfg.ShortWindowMinutes, entity.BucketMinutes)

	long, err := slo.WindowSums(ctx, entity, time.Duration(cfg.LongWindowMinutes)*time.Minute, now)
	if err != nil {
		return nil, err
	}
	short, err := slo.WindowSums(ctx, entity, time.Duration(cfg.ShortWindowMinutes)*time.Minute, now)
	if err != nil {
		return nil, err
	}
	target := entity.Target()
	longBurn := slo.BurnRate(long, target)
	shortBurn := slo.BurnRate(short, target)
	met := long.Total > 0 && longBurn >= cfg.BurnRateThreshold && shortBurn >= cfg.BurnRateThreshold
	clear := longBurn < cfg.BurnRateThreshold || shortBurn < cfg.BurnRateThreshold

	fire, recover, err := transitionRuleState(rule, met, clear, now)
	if err != nil {
		return nil, err
	}
	switch {
	case fire:
		return &EvalResult{Fired: true, Message: buildSLOBurnRateMessage(entity, longBurn, shortBurn, cfg.LongWindowMinutes, cfg.ShortWindowMinutes, cfg.BurnRateThreshold, getProjectName(projectId))}, nil
	case recover:
		detail := fmt.Sprintf("The burn rate is back to %.1fx over the last %s and %.1fx over the last %s (threshold %.1fx).", longBurn, windowLabel(cfg.LongWindowMinutes), shortBurn, windowLabel(cfg.ShortWindowMinutes), cfg.BurnRateThreshold)
		return &EvalResult{Recovered: true, Message: buildSLORecoveredMessage(entity, detail, getProjectName(projectId))}, nil
	}
	return &EvalResult{}, nil
}

func evaluateSLOBudgetConsumed(_ context.Context, rule *models.NotificationRule, projectId uuid.UUID) (*EvalResult, error) {
	var cfg sloBudgetConsumedConfig
	if err := json.Unmarshal(rule.Config, &cfg); err != nil {
		return nil, fmt.Errorf("invalid slo_budget_consumed config: %w", err)
	}
	if cfg.ConsumedPercent <= 0 {
		cfg.ConsumedPercent = defaultBudgetConsumedPercent
	}
	entity, err := loadRuleSLO(cfg.SloId, projectId)
	if err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	if !entity.Enabled {
		return recoverDisabledSLO(rule, entity, projectId, now)
	}
	if entity.LastBucketAt == nil {
		return &EvalResult{}, nil
	}

	ok := entity.BudgetRemaining != nil
	consumed := 0.0
	if ok {
		consumed = 100 - *entity.BudgetRemaining
	}
	met := ok && consumed >= cfg.ConsumedPercent
	clear := !ok || consumed < cfg.ConsumedPercent

	fire, recover, err := transitionRuleState(rule, met, clear, now)
	if err != nil {
		return nil, err
	}
	switch {
	case fire:
		return &EvalResult{Fired: true, Message: buildSLOBudgetConsumedMessage(entity, consumed, cfg.ConsumedPercent, getProjectName(projectId))}, nil
	case recover:
		detail := fmt.Sprintf("%.1f%% of the %d-day error budget is consumed, below the %.0f%% line again.", consumed, entity.WindowDays, cfg.ConsumedPercent)
		if !ok {
			detail = fmt.Sprintf("The SLO has no data in its %d-day window.", entity.WindowDays)
		}
		return &EvalResult{Recovered: true, Message: buildSLORecoveredMessage(entity, detail, getProjectName(projectId))}, nil
	}
	return &EvalResult{}, nil
}

func sliceAligned(minutes, bucketMinutes int) int {
	if bucketMinutes <= 1 {
		return minutes
	}
	return (minutes + bucketMinutes - 1) / bucketMinutes * bucketMinutes
}

func recoverDisabledSLO(rule *models.NotificationRule, entity *models.SLO, projectId uuid.UUID, now time.Time) (*EvalResult, error) {
	_, recover, err := transitionRuleState(rule, false, true, now)
	if err != nil || !recover {
		return &EvalResult{}, err
	}
	return &EvalResult{Recovered: true, Message: buildSLORecoveredMessage(entity, "The SLO was disabled.", getProjectName(projectId))}, nil
}

func transitionRuleState(rule *models.NotificationRule, met bool, clear bool, now time.Time) (fire bool, recover bool, err error) {
	state, err := db.ExecuteTransaction(func(tx *sql.Tx) (*models.NotificationRuleState, error) {
		return transactional.NotificationRuleStateRepository.Find(tx, rule.Id)
	})
	if err != nil {
		return false, false, err
	}
	firing := state != nil && state.Firing
	switch {
	case !firing && met:
		if !cooldowns.canFire(rule.Id, rule.CooldownMinutes) {
			return false, false, nil
		}
		fire = true
	case firing && clear:
		recover = true
	default:
		return false, false, nil
	}
	changed, err := db.ExecuteTransaction(func(tx *sql.Tx) (bool, error) {
		return transactional.NotificationRuleStateRepository.Transition(tx, rule.Id, fire, now)
	})
	if err != nil || !changed {
		return false, false, err
	}
	return fire, recover, nil
}

func rollBackRuleState(ruleId int) {
	_, err := db.ExecuteTransaction(func(tx *sql.Tx) (bool, error) {
		return transactional.NotificationRuleStateRepository.Transition(tx, ruleId, false, time.Now().UTC())
	})
	if err != nil {
		traceway.CaptureException(fmt.Errorf("failed to roll back the state of rule %d after a failed send: %w", ruleId, err))
	}
}

func dispatchRecovery(rule *models.NotificationRuleWithChannel, msg Message) {
	if rule.ChannelType == "escalation" {
		if pageResolver == nil {
			return
		}
		if err := pageResolver(rule.Id, msg.DedupToken); err != nil {
			traceway.CaptureException(fmt.Errorf("failed to auto-resolve page for recovered rule %d: %w", rule.Id, err))
		}
		return
	}
	dispatch(rule, msg)
}

func windowLabel(minutes int) string {
	switch {
	case minutes%1440 == 0:
		return fmt.Sprintf("%dd", minutes/1440)
	case minutes%60 == 0:
		return fmt.Sprintf("%dh", minutes/60)
	}
	return fmt.Sprintf("%dm", minutes)
}
