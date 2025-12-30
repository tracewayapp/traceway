package repositories

import (
	"backend/app/chdb"
	"backend/app/models"
	"context"
)

type transactionRepository struct{}

func (e *transactionRepository) InsertAsync(ctx context.Context, lines []models.Transaction) error {
	batch, err := (*chdb.Conn).PrepareBatch(ctx, "INSERT INTO transactions (id, endpoint, duration, recorded_at, status_code, body_size, client_ip)")
	if err != nil {
		return err
	}
	for _, e := range lines {
		if err := batch.Append(e.Id, e.Endpoint, e.Duration, e.RecordedAt, e.StatusCode, e.BodySize, e.ClientIP); err != nil {
			return err
		}
	}
	return batch.Send()
}

var TransactionRepository = transactionRepository{}
