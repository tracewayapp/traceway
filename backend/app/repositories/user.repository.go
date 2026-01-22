package repositories

import (
	"backend/app/models"
	"database/sql"

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
		Email:    email,
		Name:     name,
		Password: hashedPassword,
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

var UserRepository = userRepository{}
