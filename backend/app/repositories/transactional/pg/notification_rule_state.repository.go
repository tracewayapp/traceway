//go:build transactional_pg

package pg

import (
	"database/sql"
	"time"

	"github.com/tracewayapp/traceway/backend/app/db"
	"github.com/tracewayapp/traceway/backend/app/models"

	"github.com/tracewayapp/lit/v2"
)

type notificationRuleStateRepository struct{}

func (r *notificationRuleStateRepository) Find(tx *sql.Tx, ruleId int) (*models.NotificationRuleState, error) {
	return lit.SelectSingleNamed[models.NotificationRuleState](
		tx,
		"SELECT id, rule_id, firing, since, updated_at FROM notification_rule_states WHERE rule_id = :rule_id",
		lit.P{"rule_id": ruleId},
	)
}

func (r *notificationRuleStateRepository) Transition(tx *sql.Tx, ruleId int, firing bool, now time.Time) (bool, error) {
	var since any
	if firing {
		since = now.UTC()
	}
	params := lit.P{"rule_id": ruleId, "firing": firing, "since": since, "updated_at": now.UTC()}
	changed, err := execAffected(tx, "UPDATE notification_rule_states SET firing = :firing, since = :since, updated_at = :updated_at WHERE rule_id = :rule_id AND firing <> :firing", params)
	if err != nil || changed {
		return changed, err
	}
	return execAffected(tx, "INSERT INTO notification_rule_states (rule_id, firing, since, updated_at) VALUES (:rule_id, :firing, :since, :updated_at) ON CONFLICT(rule_id) DO NOTHING", params)
}

func execAffected(tx *sql.Tx, namedQuery string, params lit.P) (bool, error) {
	query, args, err := lit.ParseNamedQuery(db.Driver, namedQuery, params)
	if err != nil {
		return false, err
	}
	result, err := tx.Exec(query, args...)
	if err != nil {
		return false, err
	}
	affected, err := result.RowsAffected()
	return affected > 0, err
}

func (r *notificationRuleStateRepository) Delete(tx *sql.Tx, ruleId int) error {
	return lit.DeleteNamed(db.Driver, tx, "DELETE FROM notification_rule_states WHERE rule_id = :rule_id", lit.P{"rule_id": ruleId})
}

var NotificationRuleStateRepository = &notificationRuleStateRepository{}
