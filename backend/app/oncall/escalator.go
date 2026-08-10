package oncall

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/tracewayapp/traceway/backend/app/config"
	"github.com/tracewayapp/traceway/backend/app/db"
	"github.com/tracewayapp/traceway/backend/app/models"
	"github.com/tracewayapp/traceway/backend/app/notifications"
	"github.com/tracewayapp/traceway/backend/app/outbox"
	"github.com/tracewayapp/traceway/backend/app/repositories/transactional"
	"github.com/tracewayapp/traceway/backend/app/repositories/transactional/shared"
	traceway "go.tracewayapp.com"
)

const escalatorAdvisoryLockId = 824737002

// errClaimLost aborts a claim transaction when the guarded escalation-state
// update matches no row: a concurrent acknowledge/resolve committed after the
// due check, and its CancelByKey could not see this claim's uncommitted
// deliveries — so they must never commit. Rolling back is the normal loss of
// a benign race, not an error worth reporting.
var errClaimLost = errors.New("page claim lost to a concurrent acknowledge/resolve")

var wakeCh = make(chan struct{}, 1)

// Wake nudges the escalator so a freshly opened page notifies its first level
// immediately instead of waiting for the next tick. Non-blocking.
func Wake() {
	select {
	case wakeCh <- struct{}{}:
	default:
	}
}

func escalatorPollInterval() time.Duration {
	return config.PollSeconds(config.Config.OncallPollSeconds, 30)
}

// StartEscalator runs the ack-based escalation loop. Each tick is purely
// transactional: due pages advance a level and their deliveries are enqueued
// into the notification outbox, which owns sending, retries, and crash
// recovery. Ack/resolve cancels queued deliveries via outbox.CancelByKey.
func StartEscalator(ctx context.Context) {
	go func() {
		defer traceway.Recover()

		ticker := time.NewTicker(escalatorPollInterval())
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
			case <-wakeCh:
			}
			runEscalatorTick(ctx, time.Now().UTC())
		}
	}()
}

func runEscalatorTick(_ context.Context, now time.Time) {
	// Each page is claimed in its own transaction so one failing page (e.g. a
	// target schedule whose stored definition no longer parses) cannot roll
	// back or block escalation for every other page.
	pageIds, err := db.ExecuteTransaction(func(tx *sql.Tx) ([]int, error) {
		duePages, err := transactional.PageRepository.FindDue(tx, now)
		if err != nil {
			return nil, err
		}
		ids := make([]int, 0, len(duePages))
		for _, page := range duePages {
			ids = append(ids, page.Id)
		}
		return ids, nil
	})
	if err != nil {
		traceway.CaptureException(fmt.Errorf("escalator tick failed: %w", err))
		return
	}

	enqueued := 0
	for _, pageId := range pageIds {
		count, err := db.ExecuteTransaction(func(tx *sql.Tx) (int, error) {
			if !db.IsSQLite() {
				if _, err := tx.Exec(fmt.Sprintf("SELECT pg_advisory_xact_lock(%d)", escalatorAdvisoryLockId)); err != nil {
					return 0, fmt.Errorf("failed to take escalator lock: %w", err)
				}
			}
			page, err := transactional.PageRepository.FindDueById(tx, pageId, now)
			if err != nil {
				return 0, err
			}
			if page == nil {
				// Acknowledged, resolved, or claimed by a concurrent escalator
				// since the due list was read.
				return 0, nil
			}
			return claimPageEscalation(tx, page, now)
		})
		if err != nil {
			if errors.Is(err, errClaimLost) {
				continue
			}
			traceway.CaptureException(fmt.Errorf("failed to escalate page %d: %w", pageId, err))
			continue
		}
		enqueued += count
	}
	if enqueued > 0 {
		outbox.Wake()
	}
}

// claimPageEscalation advances one due page to its next escalation level and
// enqueues its deliveries. Returns the number of outbox rows created. Runs
// inside the claim transaction.
func claimPageEscalation(tx *sql.Tx, page *models.Page, now time.Time) (int, error) {
	definition, err := ParsePolicyDefinition(page.PolicySnapshot)
	if err != nil || len(definition.Steps) == 0 {
		traceway.CaptureException(fmt.Errorf("page %d has an unusable policy snapshot, stopping its escalation", page.Id))
		return 0, updateEscalationState(tx, page.Id, page.EscalationLevel, page.RepeatIteration, nil, now)
	}

	level := page.EscalationLevel + 1
	if level >= len(definition.Steps) {
		return 0, updateEscalationState(tx, page.Id, page.EscalationLevel, page.RepeatIteration, nil, now)
	}
	step := definition.Steps[level]

	enqueued := 0
	notifiedUsers := map[int]bool{}
	for _, target := range step.Targets {
		switch target.Type {
		case TargetSchedule, TargetTeam, TargetUser:
			userIds, err := resolveUserTarget(tx, page.OrganizationId, target, now)
			if err != nil {
				return 0, err
			}
			for _, userId := range userIds {
				if notifiedUsers[userId] {
					continue
				}
				notifiedUsers[userId] = true
				count, err := claimUserDeliveries(tx, page, level, userId, now)
				if err != nil {
					return 0, err
				}
				enqueued += count
			}
		case TargetChannel:
			count, err := claimChannelDelivery(tx, page, level, target.Id, now)
			if err != nil {
				return 0, err
			}
			enqueued += count
		default:
			traceway.CaptureException(fmt.Errorf("page %d step %d has unknown target type %q", page.Id, level, target.Type))
		}
	}

	lastStep := level == len(definition.Steps)-1
	switch {
	case !lastStep:
		next := now.Add(time.Duration(step.DelayMinutes) * time.Minute)
		err = updateEscalationState(tx, page.Id, level, page.RepeatIteration, &next, now)
	case page.RepeatIteration < definition.RepeatCount:
		next := now.Add(time.Duration(step.DelayMinutes) * time.Minute)
		err = updateEscalationState(tx, page.Id, -1, page.RepeatIteration+1, &next, now)
	default:
		// Exhausted: the page stays open until someone acknowledges or
		// resolves it, but nothing further is sent.
		err = updateEscalationState(tx, page.Id, level, page.RepeatIteration, nil, now)
	}
	return enqueued, err
}

// updateEscalationState is the claim path's terminal write: every branch of a
// claim must go through the status-guarded update so a page acknowledged or
// resolved mid-claim rolls the whole claim back via errClaimLost.
func updateEscalationState(tx *sql.Tx, pageId int, level int, iteration int, nextEscalationAt *time.Time, now time.Time) error {
	updated, err := transactional.PageRepository.UpdateEscalationState(tx, pageId, level, iteration, nextEscalationAt, now)
	if err != nil {
		return err
	}
	if !updated {
		return errClaimLost
	}
	return nil
}

// resolveUserTarget maps a schedule/team/user target to concrete user ids.
// Dangling targets resolve to nothing (reported, never fatal).
func resolveUserTarget(tx *sql.Tx, organizationId int, target StepTarget, now time.Time) ([]int, error) {
	switch target.Type {
	case TargetSchedule:
		userIds, err := CurrentOnCallForSchedule(tx, target.Id, now)
		if err != nil {
			return nil, err
		}
		if len(userIds) == 0 {
			traceway.CaptureException(fmt.Errorf("escalation target schedule %d resolved to nobody on call", target.Id))
		}
		return userIds, nil
	case TargetTeam:
		team, err := transactional.TeamRepository.FindById(tx, target.Id)
		if err != nil {
			return nil, err
		}
		if team == nil || team.OrganizationId != organizationId {
			traceway.CaptureException(fmt.Errorf("escalation target team %d no longer exists", target.Id))
			return nil, nil
		}
		return transactional.TeamRepository.FindMemberUserIds(tx, team.Id)
	case TargetUser:
		role, err := transactional.OrganizationRepository.GetUserRole(tx, organizationId, target.Id)
		if err != nil {
			return nil, err
		}
		if role == "" {
			traceway.CaptureException(fmt.Errorf("escalation target user %d is no longer an organization member", target.Id))
			return nil, nil
		}
		return []int{target.Id}, nil
	}
	return nil, nil
}

// claimUserDeliveries runs the user's notification-rule chain for the page's
// urgency: every step is enqueued at once as a scheduled outbox delivery
// (NotBefore staggered by the step delay) under the page's cancel key, so
// acknowledging cancels the tail with no separate scheduler. Users without a
// usable chain fall back to every enabled+verified method immediately, or the
// account email when none exist. Each delivery row mints its own ack token.
func claimUserDeliveries(tx *sql.Tx, page *models.Page, level int, userId int, now time.Time) (int, error) {
	user, err := transactional.UserRepository.FindById(tx, userId)
	if err != nil {
		return 0, err
	}
	if user == nil {
		return 0, nil
	}
	methods, err := transactional.UserContactMethodRepository.FindEnabledByUser(tx, userId)
	if err != nil {
		return 0, err
	}
	// Dropping unsendable methods here (rather than at delivery time) keeps the
	// "no methods left" account-email fallback below reachable: a user whose
	// only method is an SMS row from when Twilio was still configured must
	// still be paged.
	methods = sendableContactMethods(methods)
	methodById := make(map[int]*models.UserContactMethod, len(methods))
	for _, method := range methods {
		methodById[method.Id] = method
	}

	enqueued := 0
	appendDelivery := func(methodType string, configJSON json.RawMessage, desc string, delay time.Duration) error {
		token := newAckToken()
		sendAt := now.Add(delay)
		notification := &models.PageNotification{
			PageId:       page.Id,
			Level:        level,
			Iteration:    page.RepeatIteration,
			UserId:       &userId,
			TargetDesc:   desc,
			MethodType:   methodType,
			Status:       models.PageNotificationPending,
			ScheduledFor: &sendAt,
			AckTokenHash: shared.HashAuthToken(token),
			CreatedAt:    now,
		}
		notificationId, err := transactional.PageNotificationRepository.Create(tx, notification)
		if err != nil {
			return err
		}
		var notBefore *time.Time
		if delay > 0 {
			notBefore = &sendAt
		}
		if _, err := outbox.Enqueue(tx, outbox.Delivery{
			Kind:               models.OutboxKindPage,
			AdapterType:        methodType,
			AdapterConfig:      configJSON,
			Message:            buildPageMessage(page, level, ackURLFor(token)),
			NotBefore:          notBefore,
			CancelKey:          outbox.PageCancelKey(page.Id),
			PageNotificationId: &notificationId,
		}); err != nil {
			return err
		}
		enqueued++
		return nil
	}

	deliveryFor := func(method *models.UserContactMethod) (json.RawMessage, string, bool) {
		switch method.MethodType {
		case "email":
			configJSON, desc := EmailDeliveryFor(user.Email, ParseEmailOverride(method.Config))
			return configJSON, desc, true
		case "sms":
			return json.RawMessage(method.Config), smsDescFor(method.Config), true
		case "slack", "pushover", "telegram":
			return json.RawMessage(method.Config), user.Name + " (" + method.MethodType + ")", true
		default:
			traceway.CaptureException(fmt.Errorf("user %d has a contact method of unsupported type %q", userId, method.MethodType))
			return nil, "", false
		}
	}

	urgency := page.Urgency
	if urgency == "" {
		urgency = ResolveUrgency("", page.Severity)
	}
	rules, err := transactional.UserNotificationRuleRepository.FindByUserAndUrgency(tx, userId, urgency)
	if err != nil {
		return 0, err
	}
	usableSteps := 0
	for _, rule := range rules {
		method, ok := methodById[rule.ContactMethodId]
		if !ok {
			// Disabled or unverified since the rule was saved; skip silently
			// (the rules editor shows a warning for stale steps).
			continue
		}
		configJSON, desc, ok := deliveryFor(method)
		if !ok {
			continue
		}
		if rule.DelayMinutes > 0 {
			desc = fmt.Sprintf("%s, +%dm", desc, rule.DelayMinutes)
		}
		if err := appendDelivery(method.MethodType, configJSON, desc, time.Duration(rule.DelayMinutes)*time.Minute); err != nil {
			return 0, err
		}
		usableSteps++
	}
	if usableSteps > 0 {
		return enqueued, nil
	}
	if len(rules) > 0 {
		traceway.CaptureException(fmt.Errorf("user %d has %s-urgency notification rules but no usable steps, falling back", userId, urgency))
	}

	if len(methods) == 0 {
		// Email is never silently missing: no configured methods means the
		// account email is paged.
		configJSON, desc := EmailDeliveryFor(user.Email, "")
		if err := appendDelivery("email", configJSON, desc, 0); err != nil {
			return 0, err
		}
		return enqueued, nil
	}
	for _, method := range methods {
		configJSON, desc, ok := deliveryFor(method)
		if !ok {
			continue
		}
		if err := appendDelivery(method.MethodType, configJSON, desc, 0); err != nil {
			return 0, err
		}
	}
	return enqueued, nil
}

// sendableContactMethods drops methods this instance has no transport for.
// Only SMS can lose its transport: Twilio credentials can be removed after the
// method was created, and paging into a dead channel is worse than falling
// back to the account email.
func sendableContactMethods(methods []*models.UserContactMethod) []*models.UserContactMethod {
	if config.Config.TwilioEnabled() {
		return methods
	}
	sendable := make([]*models.UserContactMethod, 0, len(methods))
	for _, method := range methods {
		if method.MethodType == "sms" {
			continue
		}
		sendable = append(sendable, method)
	}
	return sendable
}

// SMSPhoneNumber extracts the phoneNumber from an sms contact-method config,
// or "" when missing or malformed.
func SMSPhoneNumber(config []byte) string {
	var parsed struct {
		PhoneNumber string `json:"phoneNumber"`
	}
	_ = json.Unmarshal(config, &parsed)
	return parsed.PhoneNumber
}

// smsDescFor labels the delivery row. GET /api/pages/:id returns these to
// every reader of the project, so the number is masked to its last 4 digits:
// the responder still recognises their own phone, nobody else learns it.
func smsDescFor(config []byte) string {
	number := SMSPhoneNumber(config)
	if number == "" {
		return "sms"
	}
	return notifications.MaskPhoneNumber(number) + " (sms)"
}

// ParseEmailOverride extracts the optional email override from an email
// contact-method config; "" means the account email is used.
func ParseEmailOverride(config []byte) string {
	var parsed struct {
		Email string `json:"email"`
	}
	_ = json.Unmarshal(config, &parsed)
	return parsed.Email
}

// EmailDeliveryFor builds the email adapter config (and display description)
// for a recipient, applying the override when present.
func EmailDeliveryFor(accountEmail string, override string) (json.RawMessage, string) {
	email := accountEmail
	if override != "" {
		email = override
	}
	configJSON, _ := json.Marshal(map[string]any{"recipients": []string{email}})
	return configJSON, email + " (email)"
}

// claimChannelDelivery records the delivery-log row for a plain-channel target
// (method_type "channel" for display) and enqueues the outbox delivery with
// the channel's real adapter type and a config snapshot. Channel messages
// carry the dashboard link, never an ack token: an anyone-can-click token in a
// shared room defeats attribution.
func claimChannelDelivery(tx *sql.Tx, page *models.Page, level int, channelId int, now time.Time) (int, error) {
	channel, err := transactional.NotificationChannelRepository.FindById(tx, channelId)
	if err != nil {
		return 0, err
	}
	if channel == nil || channel.ChannelType == "escalation" {
		traceway.CaptureException(fmt.Errorf("escalation target channel %d no longer exists or is not deliverable", channelId))
		return 0, nil
	}
	message := buildPageMessage(page, level, AckLink(page))
	scheduledFor := now
	notification := &models.PageNotification{
		PageId:       page.Id,
		Level:        level,
		Iteration:    page.RepeatIteration,
		TargetDesc:   "Channel: " + channel.Name,
		MethodType:   "channel",
		Status:       models.PageNotificationPending,
		ScheduledFor: &scheduledFor,
		CreatedAt:    now,
	}
	notificationId, err := transactional.PageNotificationRepository.Create(tx, notification)
	if err != nil {
		return 0, err
	}
	if _, err := outbox.Enqueue(tx, outbox.Delivery{
		Kind:               models.OutboxKindPage,
		AdapterType:        channel.ChannelType,
		AdapterConfig:      json.RawMessage(channel.Config),
		Message:            message,
		CancelKey:          outbox.PageCancelKey(page.Id),
		PageNotificationId: &notificationId,
	}); err != nil {
		return 0, err
	}
	return 1, nil
}

// buildPageMessage builds the delivery message. ackURL is per-delivery: user
// deliveries carry their own tokenized link, channel deliveries the dashboard
// link.
func buildPageMessage(page *models.Page, level int, ackURL string) notifications.Message {
	prefix := "[Page] "
	if level > 0 {
		prefix = fmt.Sprintf("[Page — escalation L%d] ", level+1)
	}
	body := page.Body
	if body != "" {
		body += "\n\n"
	}
	body += "Acknowledge this page: " + ackURL

	severity := notifications.Severity(page.Severity)
	if severity == "" {
		severity = notifications.SeverityCritical
	}
	return notifications.Message{
		Subject:  prefix + page.Subject,
		Body:     body,
		Severity: severity,
		RuleType: page.RuleType,
		RuleName: page.RuleName,
		URL:      ackURL,
	}
}

// newAckToken mints an opaque per-delivery acknowledge token. Only its SHA-256
// hash is stored; the plaintext exists solely inside the outgoing message.
func newAckToken() string {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	return "twk_" + base64.RawURLEncoding.EncodeToString(b)
}

// appBaseURL is the dashboard origin for outgoing links. On-call deployments
// should set APP_BASE_URL; the localhost fallback mirrors the email service.
func appBaseURL() string {
	base := config.Config.AppBaseURL
	if base == "" {
		base = "http://localhost:5173"
	}
	return strings.TrimRight(base, "/")
}

// ackURLFor builds the public no-login acknowledge link for a delivery token.
func ackURLFor(token string) string {
	return appBaseURL() + "/ack/" + token
}

// AckLink builds the absolute dashboard link that acknowledges a page. The
// project id is included so the dashboard can switch to the page's project —
// pages are resolved against the viewer's selected project.
func AckLink(page *models.Page) string {
	return appBaseURL() + "/on-call?page=" + strconv.Itoa(page.Id) + "&projectId=" + page.ProjectId.String()
}
