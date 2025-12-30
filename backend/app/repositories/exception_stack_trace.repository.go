package repositories

import (
	"backend/app/chdb"
	"backend/app/models"
	"context"
)

type exceptionStackTraceRepository struct{}

func (e *exceptionStackTraceRepository) InsertAsync(ctx context.Context, lines []models.ExceptionStackTrace) error {
	batch, err := (*chdb.Conn).PrepareBatch(ctx, "INSERT INTO exception_stack_traces (transaction_id, exception_hash, stack_trace, recorded_at)")
	if err != nil {
		return err
	}
	for _, e := range lines {
		if err := batch.Append(e.TransactionId, e.ExceptionHash, e.StackTrace, e.RecordedAt); err != nil {
			return err
		}
	}
	return batch.Send()
}

var ExceptionStackTraceRepository = exceptionStackTraceRepository{}
