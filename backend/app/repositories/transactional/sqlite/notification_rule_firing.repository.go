//go:build !transactional_pg

package sqlite

import (
	"database/sql"
	"time"

	"github.com/tracewayapp/traceway/backend/app/db"
	"github.com/tracewayapp/traceway/backend/app/models"

	"github.com/tracewayapp/lit/v2"
)

type notificationRuleFiringRepository struct{}

func (r *notificationRuleFiringRepository) FindByRule(tx *sql.Tx, ruleId int) ([]*models.NotificationRuleFiring, error) {
	return lit.SelectNamed[models.NotificationRuleFiring](
		tx,
		"SELECT id, rule_id, token, since, silent_since FROM notification_rule_firing WHERE rule_id = :rule_id ORDER BY token",
		lit.P{"rule_id": ruleId},
	)
}

// Insert reports whether this call opened the alert, so when two instances
// open the same one at once only one of them sends it.
func (r *notificationRuleFiringRepository) Insert(tx *sql.Tx, ruleId int, token string, now time.Time) (bool, error) {
	return execAffected(tx, "INSERT INTO notification_rule_firing (rule_id, token, since) VALUES (:rule_id, :token, :since) ON CONFLICT(rule_id, token) DO NOTHING",
		lit.P{"rule_id": ruleId, "token": token, "since": now.UTC()})
}

// Delete reports whether this call closed the alert, so only one instance
// sends the notice that closes it.
func (r *notificationRuleFiringRepository) Delete(tx *sql.Tx, ruleId int, token string) (bool, error) {
	return execAffected(tx, "DELETE FROM notification_rule_firing WHERE rule_id = :rule_id AND token = :token",
		lit.P{"rule_id": ruleId, "token": token})
}

func (r *notificationRuleFiringRepository) SetSilentSince(tx *sql.Tx, ruleId int, token string, silentSince *time.Time) error {
	query, args, err := lit.ParseNamedQuery(db.Driver, "UPDATE notification_rule_firing SET silent_since = :silent_since WHERE rule_id = :rule_id AND token = :token",
		lit.P{"rule_id": ruleId, "token": token, "silent_since": silentSince})
	if err != nil {
		return err
	}
	return lit.UpdateNative(tx, query, args...)
}

func (r *notificationRuleFiringRepository) DeleteByRule(tx *sql.Tx, ruleId int) error {
	return lit.DeleteNamed(db.Driver, tx, "DELETE FROM notification_rule_firing WHERE rule_id = :rule_id", lit.P{"rule_id": ruleId})
}

var NotificationRuleFiringRepository = &notificationRuleFiringRepository{}
