package repositories

import (
	"backend/app/models"
	"database/sql"
	"time"

	"github.com/tracewayapp/go-lightning/lit"
)

type userRepository struct{}

func (r *userRepository) FindByEmail(tx *sql.Tx, email string) (*models.User, error) {
	return lit.SelectSingle[models.User](
		tx,
		"SELECT id, email, name, password, created_at FROM users WHERE email = $1",
		email,
	)
}

func (r *userRepository) FindById(tx *sql.Tx, id int) (*models.User, error) {
	return lit.SelectSingle[models.User](
		tx,
		"SELECT id, email, name, password, created_at FROM users WHERE id = $1",
		id,
	)
}

func (r *userRepository) Create(tx *sql.Tx, email string, name string, hashedPassword string) (*models.User, error) {
	user := &models.User{
		Email:     email,
		Name:      name,
		Password:  hashedPassword,
		CreatedAt: time.Now().UTC(),
	}

	id, err := lit.Insert(tx, user)
	if err != nil {
		return nil, err
	}
	user.Id = id

	return user, nil
}

func (r *userRepository) EmailExists(tx *sql.Tx, email string) (bool, error) {
	user, err := r.FindByEmail(tx, email)
	if err != nil {
		return false, err
	}
	return user != nil, nil
}

func (r *userRepository) SetPasswordResetToken(tx *sql.Tx, userId int, token string, expiresAt time.Time) error {
	now := time.Now()
	return lit.Update[models.User](
		tx,
		&models.User{
			PasswordResetToken:       &token,
			PasswordResetExpiresAt:   &expiresAt,
			PasswordResetRequestedAt: &now,
		},
		"id = $1",
		userId,
	)
}

func (r *userRepository) ClearPasswordResetToken(tx *sql.Tx, userId int) error {
	_, err := tx.Exec(
		"UPDATE users SET password_reset_token = NULL, password_reset_expires_at = NULL, password_reset_requested_at = NULL WHERE id = $1",
		userId,
	)
	return err
}

func (r *userRepository) FindByPasswordResetToken(tx *sql.Tx, token string) (*models.User, error) {
	return lit.SelectSingle[models.User](
		tx,
		"SELECT id, email, name, password, created_at, password_reset_token, password_reset_expires_at, password_reset_requested_at FROM users WHERE password_reset_token = $1",
		token,
	)
}

func (r *userRepository) UpdatePassword(tx *sql.Tx, userId int, hashedPassword string) error {
	return lit.Update[models.User](
		tx,
		&models.User{Password: hashedPassword},
		"id = $1",
		userId,
	)
}

var UserRepository = userRepository{}
