//go:build !transactional_pg

package sqlite

import (
	"database/sql"

	"github.com/tracewayapp/traceway/backend/app/db"
	"github.com/tracewayapp/traceway/backend/app/models"

	"github.com/tracewayapp/lit/v2"
)

type userNotificationRuleRepository struct{}

const userNotificationRuleColumns = "id, user_id, urgency, position, delay_minutes, contact_method_id, created_at"

func (r *userNotificationRuleRepository) FindByUser(tx *sql.Tx, userId int) ([]*models.UserNotificationRule, error) {
	return lit.SelectNamed[models.UserNotificationRule](
		tx,
		"SELECT "+userNotificationRuleColumns+" FROM user_notification_rules WHERE user_id = :user_id ORDER BY urgency ASC, position ASC, id ASC",
		lit.P{"user_id": userId},
	)
}

func (r *userNotificationRuleRepository) FindByUserAndUrgency(tx *sql.Tx, userId int, urgency string) ([]*models.UserNotificationRule, error) {
	return lit.SelectNamed[models.UserNotificationRule](
		tx,
		"SELECT "+userNotificationRuleColumns+" FROM user_notification_rules WHERE user_id = :user_id AND urgency = :urgency ORDER BY position ASC, id ASC",
		lit.P{"user_id": userId, "urgency": urgency},
	)
}

// ReplaceForUser swaps the user's entire rule set in one transaction, so
// positions are correct by construction.
func (r *userNotificationRuleRepository) ReplaceForUser(tx *sql.Tx, userId int, rules []*models.UserNotificationRule) error {
	if err := lit.DeleteNamed(db.Driver, tx, "DELETE FROM user_notification_rules WHERE user_id = :user_id", lit.P{"user_id": userId}); err != nil {
		return err
	}
	for _, rule := range rules {
		if _, err := lit.Insert[models.UserNotificationRule](tx, rule); err != nil {
			return err
		}
	}
	return nil
}

var UserNotificationRuleRepository = userNotificationRuleRepository{}
