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

// Set records whether the rule is firing. since is the start of the current
// firing episode and is cleared on recovery.
func (r *notificationRuleStateRepository) Set(tx *sql.Tx, ruleId int, firing bool, now time.Time) error {
	var since any
	if firing {
		since = now.UTC()
	}
	query, args, err := lit.ParseNamedQuery(
		db.Driver,
		"INSERT INTO notification_rule_states (rule_id, firing, since, updated_at) VALUES (:rule_id, :firing, :since, :updated_at) ON CONFLICT(rule_id) DO UPDATE SET firing = excluded.firing, since = excluded.since, updated_at = excluded.updated_at",
		lit.P{"rule_id": ruleId, "firing": firing, "since": since, "updated_at": now.UTC()},
	)
	if err != nil {
		return err
	}
	return lit.UpdateNative(tx, query, args...)
}

func (r *notificationRuleStateRepository) Delete(tx *sql.Tx, ruleId int) error {
	return lit.DeleteNamed(db.Driver, tx, "DELETE FROM notification_rule_states WHERE rule_id = :rule_id", lit.P{"rule_id": ruleId})
}

var NotificationRuleStateRepository = &notificationRuleStateRepository{}
