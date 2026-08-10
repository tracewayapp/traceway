//go:build !transactional_pg

package sqlite

import (
	"database/sql"
	"time"

	"github.com/tracewayapp/traceway/backend/app/db"
	"github.com/tracewayapp/traceway/backend/app/models"

	"github.com/tracewayapp/lit/v2"
)

type userContactMethodRepository struct{}

const userContactMethodColumns = "id, user_id, method_type, config, enabled, verified, verification_code_hash, verification_expires_at, verification_attempts, created_at"

func (r *userContactMethodRepository) FindById(tx *sql.Tx, id int) (*models.UserContactMethod, error) {
	return lit.SelectSingleNamed[models.UserContactMethod](
		tx,
		"SELECT "+userContactMethodColumns+" FROM user_contact_methods WHERE id = :id",
		lit.P{"id": id},
	)
}

func (r *userContactMethodRepository) FindByUser(tx *sql.Tx, userId int) ([]*models.UserContactMethod, error) {
	return lit.SelectNamed[models.UserContactMethod](
		tx,
		"SELECT "+userContactMethodColumns+" FROM user_contact_methods WHERE user_id = :user_id ORDER BY created_at ASC, id ASC",
		lit.P{"user_id": userId},
	)
}

// FindEnabledByUser returns enabled AND verified methods: unverified numbers
// are never paged.
func (r *userContactMethodRepository) FindEnabledByUser(tx *sql.Tx, userId int) ([]*models.UserContactMethod, error) {
	return lit.SelectNamed[models.UserContactMethod](
		tx,
		"SELECT "+userContactMethodColumns+" FROM user_contact_methods WHERE user_id = :user_id AND enabled = :enabled AND verified = :verified ORDER BY created_at ASC, id ASC",
		lit.P{"user_id": userId, "enabled": true, "verified": true},
	)
}

// SetVerification stores a fresh hashed code and flips the method back to
// unverified with zero attempts.
func (r *userContactMethodRepository) SetVerification(tx *sql.Tx, id int, codeHash string, expiresAt time.Time) error {
	query, args, err := lit.ParseNamedQuery(
		db.Driver,
		"UPDATE user_contact_methods SET verified = :verified, verification_code_hash = :code_hash, verification_expires_at = :expires_at, verification_attempts = 0 WHERE id = :id",
		lit.P{"verified": false, "code_hash": codeHash, "expires_at": expiresAt.UTC(), "id": id},
	)
	if err != nil {
		return err
	}
	return lit.UpdateNative(tx, query, args...)
}

func (r *userContactMethodRepository) MarkVerified(tx *sql.Tx, id int) error {
	query, args, err := lit.ParseNamedQuery(
		db.Driver,
		"UPDATE user_contact_methods SET verified = :verified, verification_code_hash = '', verification_expires_at = NULL, verification_attempts = 0 WHERE id = :id",
		lit.P{"verified": true, "id": id},
	)
	if err != nil {
		return err
	}
	return lit.UpdateNative(tx, query, args...)
}

// IncrementVerificationAttempts consumes one verification attempt, guarded in
// SQL so concurrent requests cannot exceed the cap (a check-then-increment in
// Go would race). Returns false when the attempt budget is already spent.
func (r *userContactMethodRepository) IncrementVerificationAttempts(tx *sql.Tx, id int, maxAttempts int) (bool, error) {
	query, args, err := lit.ParseNamedQuery(
		db.Driver,
		"UPDATE user_contact_methods SET verification_attempts = verification_attempts + 1 WHERE id = :id AND verification_attempts < :max_attempts",
		lit.P{"id": id, "max_attempts": maxAttempts},
	)
	if err != nil {
		return false, err
	}
	result, err := tx.Exec(query, args...)
	if err != nil {
		return false, err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return false, err
	}
	return affected > 0, nil
}

func (r *userContactMethodRepository) Create(tx *sql.Tx, method *models.UserContactMethod) (int, error) {
	return lit.Insert[models.UserContactMethod](tx, method)
}

func (r *userContactMethodRepository) Update(tx *sql.Tx, method *models.UserContactMethod) error {
	return lit.UpdateNamed(tx, method, "id = :id", lit.P{"id": method.Id})
}

func (r *userContactMethodRepository) Delete(tx *sql.Tx, id int) error {
	return lit.DeleteNamed(db.Driver, tx, "DELETE FROM user_contact_methods WHERE id = :id", lit.P{"id": id})
}

var UserContactMethodRepository = userContactMethodRepository{}
