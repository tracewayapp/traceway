package notifications

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/tracewayapp/traceway/backend/app/db"
	"github.com/tracewayapp/traceway/backend/app/models"
	"github.com/tracewayapp/traceway/backend/app/repositories/transactional"
	"github.com/tracewayapp/traceway/backend/app/synthetics"
	traceway "go.tracewayapp.com"
)

// PageResolver auto-resolves the unresolved page a check_down rule opened,
// identified by its rule-scoped dedup token. Implemented by the oncall
// package and registered from cmd/run.go (notifications cannot import oncall).
type PageResolver func(ruleId int, dedupToken string) error

var pageResolver PageResolver

func RegisterPageResolver(resolver PageResolver) {
	pageResolver = resolver
}

// OnCheckStateChange is the synthetics notifier (synthetics.RegisterNotifier
// in cmd/run.go). It runs AFTER the state transaction committed and holds no
// transaction: dispatch opens its own, and the SQLite main DB has a single
// connection, so an ambient transaction here would deadlock.
func OnCheckStateChange(transition synthetics.StateTransition) {
	defer traceway.Recover()

	rules, err := db.ExecuteTransaction(func(tx *sql.Tx) ([]*models.NotificationRuleWithChannel, error) {
		return transactional.NotificationRuleRepository.FindEnabledEventRules(tx, transition.Check.ProjectId)
	})
	if err != nil {
		traceway.CaptureException(fmt.Errorf("failed to load event notification rules for check transition: %w", err))
		return
	}

	wentDown := transition.To == models.CheckStatusDown
	recovered := transition.From == models.CheckStatusDown && transition.To == models.CheckStatusUp
	if !wentDown && !recovered {
		return
	}

	projectName := ""
	for _, rule := range rules {
		if rule.RuleType != "check_down" {
			continue
		}
		if rule.SnoozedUntil != nil && rule.SnoozedUntil.After(time.Now()) {
			continue
		}
		if projectName == "" {
			projectName = getProjectName(transition.Check.ProjectId)
		}

		if wentDown {
			dedupKey := checkDownDedupKey(rule.Id, transition.Check.Id)
			if dedup.isDuplicate(dedupKey, time.Duration(rule.CooldownMinutes)*time.Minute) {
				continue
			}
			msg := buildCheckDownMessage(&transition.Check, transition.ErrorMsg, projectName)
			// Record before dispatch: a persistently failing dispatch retries
			// once per cooldown window, never on every transition.
			dedup.record(dedupKey)
			dispatch(rule, msg)
			continue
		}

		// Recovery. Escalation-channel rules must NOT dispatch: dispatch would
		// open a fresh "recovered" page that stays open forever. Auto-resolve
		// the down page instead.
		if rule.ChannelType == "escalation" {
			if pageResolver == nil {
				continue
			}
			if err := pageResolver(rule.Id, checkPageDedupToken(transition.Check.Id)); err != nil {
				traceway.CaptureException(fmt.Errorf("failed to auto-resolve page for recovered check %d (rule=%d): %w", transition.Check.Id, rule.Id, err))
			}
			continue
		}
		dispatch(rule, buildCheckRecoveredMessage(&transition.Check, projectName))
	}
}

// checkPageDedupToken must match the DedupToken set on check_down messages,
// so recovery can find the page the down transition opened.
func checkPageDedupToken(checkId int) string {
	return fmt.Sprintf("check:%d", checkId)
}
