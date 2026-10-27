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

// statefulRuleTypes fire on the transition into the condition and send a
// recovery notice on the way out. Their firing state lives in
// notification_rule_states so it survives restarts, and the polled loop
// evaluates them during their cooldown so a recovery is never delayed by it.
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

// SLODedupToken is the DedupToken of every message about one SLO, so a
// recovery can find the page the alert opened.
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
	// Either window below the threshold ends the episode: the short one resets
	// the alert soon after the burn stops, as in the SRE workbook.
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

	// The worker's snapshot already holds the budget of the whole window;
	// summing the window again on every poll would double the read cost.
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

// sliceAligned rounds a window up to whole slices of the SLO: the buckets are
// keyed by their start, so a window shorter than one slice would hold none
// and could never fire.
func sliceAligned(minutes, bucketMinutes int) int {
	if bucketMinutes <= 1 {
		return minutes
	}
	return (minutes + bucketMinutes - 1) / bucketMinutes * bucketMinutes
}

// recoverDisabledSLO ends a firing episode when its SLO is disabled; without
// it the page the alert opened would stay open with nothing left to resolve it.
func recoverDisabledSLO(rule *models.NotificationRule, entity *models.SLO, projectId uuid.UUID, now time.Time) (*EvalResult, error) {
	_, recover, err := transitionRuleState(rule, false, true, now)
	if err != nil || !recover {
		return &EvalResult{}, err
	}
	return &EvalResult{Recovered: true, Message: buildSLORecoveredMessage(entity, "The SLO was disabled.", getProjectName(projectId))}, nil
}

// transitionRuleState moves the durable firing state of a rule. It fires only
// when the rule is not firing yet and its cooldown allows it, so a condition
// met during the cooldown fires on the first tick after it, never silently.
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

// dispatchRecovery sends the notice that closes an alert. Escalation-channel
// rules must not dispatch: that would open a fresh page that stays open
// forever. The page the alert opened is auto-resolved instead.
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
