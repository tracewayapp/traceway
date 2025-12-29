package repositories

import (
	"backend/app/models"
	"database/sql"

	"github.com/tracewayapp/go-lightning/lpg"
)

type transactionRepository struct{}

func (e transactionRepository) FindAll(tx *sql.Tx) ([]*models.Transaction, error) {
	return lpg.SelectGeneric[models.Transaction](tx, "SELECT * FROM transactions")
}

// func (e transactionRepository) Create(tx *sql.Tx, transaction *models.Transaction) error {
// 	return lpg.Create[models.Transaction](tx, "SELECT * FROM transactions")
// }

var TransactionRepository = transactionRepository{}
