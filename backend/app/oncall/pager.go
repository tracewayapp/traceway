package oncall

import (
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/tracewayapp/traceway/backend/app/db"
	"github.com/tracewayapp/traceway/backend/app/models"
	"github.com/tracewayapp/traceway/backend/app/notifications"
	"github.com/tracewayapp/traceway/backend/app/repositories/transactional"
)

// EscalationChannelPolicyId extracts the policyId from an escalation
// channel's config; 0 means the config is missing or malformed.
func EscalationChannelPolicyId(configJSON json.RawMessage) int {
	var cfg struct {
		PolicyId int `json:"policyId"`
	}
	if err := json.Unmarshal(configJSON, &cfg); err != nil {
		return 0
	}
	return cfg.PolicyId
}

// OpenPageFromDispatch is registered as the notifications package's page
// opener (see notifications.RegisterPageOpener). It runs inside dispatch for
// rules targeting an escalation channel: instead of sending anything, it opens
// (or dedups into) a page; the escalator worker does all notifying.
func OpenPageFromDispatch(configJSON json.RawMessage, rule *models.NotificationRuleWithChannel, msg notifications.Message) error {
	policyId := EscalationChannelPolicyId(configJSON)
	if policyId == 0 {
		return errors.New("escalation channel config has no policyId")
	}

	ruleId := rule.Id
	opened, err := openPage(openPageParams{
		PolicyId:  policyId,
		ProjectId: rule.ProjectId,
		RuleId:    &ruleId,
		RuleName:  rule.Name,
		RuleType:  rule.RuleType,
		Subject:   msg.Subject,
		Body:      msg.Body,
		URL:       msg.URL,
		Severity:  string(msg.Severity),
		DedupKey:  pageDedupKey(fmt.Sprintf("%d", rule.Id), msg.DedupToken),
	})
	if err != nil {
		return err
	}
	if opened {
		Wake()
	}
	return nil
}

// OpenTestPage opens a real page for the channel test endpoint, exercising the
// full escalation loop.
func OpenTestPage(policyId int, projectId uuid.UUID, channelId int, channelName string) error {
	opened, err := openPage(openPageParams{
		PolicyId:  policyId,
		ProjectId: projectId,
		RuleName:  "Channel test",
		RuleType:  "test",
		Subject:   fmt.Sprintf("Test page from channel %q", channelName),
		Body:      "This is a test page sent from the escalation channel test button. Acknowledge or resolve it from the On-Call page.",
		Severity:  string(notifications.SeverityInfo),
		DedupKey:  fmt.Sprintf("test|channel:%d", channelId),
	})
	if err != nil {
		return err
	}
	if opened {
		Wake()
	}
	return nil
}

const maxDedupKeyLength = 300

// pageDedupKey builds "prefix|token". pages.dedup_key is VARCHAR(300) on
// Postgres while the token can be unbounded client input (endpoint names,
// metric names), so an oversized token is replaced with a deterministic
// digest: the same long token still dedups into the same page instead of
// failing the insert.
func pageDedupKey(prefix string, token string) string {
	key := prefix + "|" + token
	if len(key) <= maxDedupKeyLength {
		return key
	}
	sum := sha256.Sum256([]byte(token))
	return prefix + "|" + hex.EncodeToString(sum[:])[:32]
}

type openPageParams struct {
	PolicyId  int
	ProjectId uuid.UUID
	RuleId    *int
	RuleName  string
	RuleType  string
	Subject   string
	Body      string
	URL       string
	Severity  string
	DedupKey  string
}

// openPage creates a page or bumps the unresolved page holding the dedup key.
// Returns true when a new page was created.
func openPage(params openPageParams) (bool, error) {
	opened, err := db.ExecuteTransaction(func(tx *sql.Tx) (bool, error) {
		return openPageInTx(tx, params)
	})
	if err != nil && db.IsUniqueViolation(err) {
		// Lost a race with a concurrent fire holding the same dedup key: the
		// other side created the page, so this fire is just an extra event.
		_, bumpErr := db.ExecuteTransaction(func(tx *sql.Tx) (bool, error) {
			return openPageInTx(tx, params)
		})
		return false, bumpErr
	}
	return opened, err
}

func openPageInTx(tx *sql.Tx, params openPageParams) (bool, error) {
	now := time.Now().UTC()

	existing, err := transactional.PageRepository.FindUnresolvedByDedupKey(tx, params.DedupKey)
	if err != nil {
		return false, err
	}
	if existing != nil {
		// Never resets the escalation clock and never re-notifies.
		return false, transactional.PageRepository.BumpEvent(tx, existing.Id, now)
	}

	policy, err := transactional.EscalationPolicyRepository.FindById(tx, params.PolicyId)
	if err != nil {
		return false, err
	}
	if policy == nil {
		return false, fmt.Errorf("escalation policy %d not found", params.PolicyId)
	}
	project, err := transactional.ProjectRepository.FindById(tx, params.ProjectId)
	if err != nil {
		return false, err
	}
	if project == nil || project.OrganizationId == nil || *project.OrganizationId != policy.OrganizationId {
		return false, fmt.Errorf("escalation policy %d does not belong to the project's organization", params.PolicyId)
	}

	// Urgency is resolved once at page open and remembered: a low-severity
	// duplicate bump must never flip an in-flight high-urgency page.
	policyUrgency := ""
	if parsedDefinition, defErr := ParsePolicyDefinition(policy.Definition); defErr == nil {
		policyUrgency = parsedDefinition.Urgency
	}

	policyId := policy.Id
	nextEscalationAt := now
	page := &models.Page{
		OrganizationId:   policy.OrganizationId,
		ProjectId:        params.ProjectId,
		PolicyId:         &policyId,
		PolicySnapshot:   policy.Definition,
		Urgency:          ResolveUrgency(policyUrgency, params.Severity),
		RuleId:           params.RuleId,
		RuleName:         params.RuleName,
		RuleType:         params.RuleType,
		Subject:          params.Subject,
		Body:             params.Body,
		URL:              params.URL,
		Severity:         params.Severity,
		Status:           models.PageStatusOpen,
		DedupKey:         params.DedupKey,
		EventCount:       1,
		LastEventAt:      now,
		EscalationLevel:  -1,
		RepeatIteration:  0,
		NextEscalationAt: &nextEscalationAt,
		CreatedAt:        now,
		UpdatedAt:        now,
	}
	if _, err := transactional.PageRepository.Create(tx, page); err != nil {
		return false, err
	}
	return true, nil
}
