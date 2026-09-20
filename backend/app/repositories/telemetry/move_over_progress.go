package telemetry

import (
	"context"
	"database/sql"

	"github.com/tracewayapp/traceway/backend/app/db"
	"github.com/tracewayapp/traceway/backend/app/repositories/transactional"
)

type moveOverProgress struct{}

func (moveOverProgress) FindProgress(ctx context.Context) ([]transactional.MoveOverDay, error) {
	return db.ExecuteTransaction(func(tx *sql.Tx) ([]transactional.MoveOverDay, error) {
		return transactional.MoveOverRepository.FindProgress(ctx, tx)
	})
}

func (moveOverProgress) SaveProgress(ctx context.Context, day transactional.MoveOverDay) error {
	_, err := db.ExecuteTransaction(func(tx *sql.Tx) (struct{}, error) {
		return struct{}{}, transactional.MoveOverRepository.SaveProgress(ctx, tx, day)
	})
	return err
}
