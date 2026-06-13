package repositories

import (
	"database/sql"
	"time"

	"github.com/tracewayapp/traceway/backend/app/db"
	"github.com/tracewayapp/traceway/backend/app/models"

	"github.com/google/uuid"
	"github.com/tracewayapp/lit/v2"
)

type notificationRuleRepository struct{}

func (r *notificationRuleRepository) FindByProjectWithChannel(tx *sql.Tx, projectId uuid.UUID) ([]*models.NotificationRuleWithChannel, error) {
	return lit.SelectNamed[models.NotificationRuleWithChannel](
		tx,
		`SELECT r.id, r.project_id, r.channel_id, r.name, r.rule_type, r.config, r.enabled, r.cooldown_minutes, r.severity, r.snoozed_until, r.created_by, r.created_at, r.updated_at,
			c.name as channel_name, c.channel_type as channel_type
		FROM notification_rules r
		JOIN notification_channels c ON c.id = r.channel_id
		WHERE r.project_id = :project_id
		ORDER BY r.created_at DESC`,
		lit.P{"project_id": projectId},
	)
}

func (r *notificationRuleRepository) FindById(tx *sql.Tx, id int) (*models.NotificationRule, error) {
	return lit.SelectSingleNamed[models.NotificationRule](
		tx,
		"SELECT id, project_id, channel_id, name, rule_type, config, enabled, cooldown_minutes, severity, snoozed_until, created_by, created_at, updated_at FROM notification_rules WHERE id = :id",
		lit.P{"id": id},
	)
}

func (r *notificationRuleRepository) FindEnabledPolledRules(tx *sql.Tx) ([]*models.NotificationRuleWithChannel, error) {
	return lit.Select[models.NotificationRuleWithChannel](
		tx,
		`SELECT r.id, r.project_id, r.channel_id, r.name, r.rule_type, r.config, r.enabled, r.cooldown_minutes, r.severity, r.snoozed_until, r.created_by, r.created_at, r.updated_at,
			c.name as channel_name, c.channel_type as channel_type
		FROM notification_rules r
		JOIN notification_channels c ON c.id = r.channel_id
		WHERE r.enabled = true AND c.enabled = true
			AND r.rule_type NOT IN ('new_error', 'error_regression', 'ai_trace_cost')`,
	)
}

func (r *notificationRuleRepository) FindEnabledEventRules(tx *sql.Tx, projectId uuid.UUID) ([]*models.NotificationRuleWithChannel, error) {
	return lit.SelectNamed[models.NotificationRuleWithChannel](
		tx,
		`SELECT r.id, r.project_id, r.channel_id, r.name, r.rule_type, r.config, r.enabled, r.cooldown_minutes, r.severity, r.snoozed_until, r.created_by, r.created_at, r.updated_at,
			c.name as channel_name, c.channel_type as channel_type
		FROM notification_rules r
		JOIN notification_channels c ON c.id = r.channel_id
		WHERE r.project_id = :project_id AND r.enabled = true AND c.enabled = true
			AND r.rule_type IN ('new_error', 'error_regression', 'ai_trace_cost')`,
		lit.P{"project_id": projectId},
	)
}

func (r *notificationRuleRepository) Create(tx *sql.Tx, rule *models.NotificationRule) (int, error) {
	return lit.Insert[models.NotificationRule](tx, rule)
}

func (r *notificationRuleRepository) Update(tx *sql.Tx, rule *models.NotificationRule) error {
	return lit.UpdateNamed(tx, rule, "id = :id", lit.P{"id": rule.Id})
}

func (r *notificationRuleRepository) Delete(tx *sql.Tx, id int) error {
	return lit.DeleteNamed(db.Driver, tx, "DELETE FROM notification_rules WHERE id = :id", lit.P{"id": id})
}

func (r *notificationRuleRepository) UpdateEnabled(tx *sql.Tx, id int, enabled bool) error {
	q, a, err := lit.ParseNamedQuery(db.Driver, "UPDATE notification_rules SET enabled = :enabled, updated_at = :updated_at WHERE id = :id", lit.P{"enabled": enabled, "updated_at": time.Now().UTC(), "id": id})
	if err != nil {
		return err
	}
	return lit.UpdateNative(tx, q, a...)
}

func (r *notificationRuleRepository) UpdateSnoozedUntil(tx *sql.Tx, id int, snoozedUntil *time.Time) error {
	q, a, err := lit.ParseNamedQuery(db.Driver, "UPDATE notification_rules SET snoozed_until = :snoozed_until, updated_at = :updated_at WHERE id = :id", lit.P{"snoozed_until": snoozedUntil, "updated_at": time.Now().UTC(), "id": id})
	if err != nil {
		return err
	}
	return lit.UpdateNative(tx, q, a...)
}

var NotificationRuleRepository = notificationRuleRepository{}
