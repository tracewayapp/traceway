package transactional

import (
	"context"
	"database/sql"
	"time"

	"github.com/tracewayapp/lit/v2"
	"github.com/tracewayapp/traceway/backend/app/db"
)

const (
	MoveOverStarted = "started"
	MoveOverDone    = "done"
)

type MoveOverDay struct {
	Table string
	Day   string
	State string
	Rows  int64
}

type moveOverRepository struct{}

func (moveOverRepository) FindProgress(ctx context.Context, tx *sql.Tx) ([]MoveOverDay, error) {
	rows, err := tx.QueryContext(ctx, "SELECT source_table, day, state, moved_rows FROM v2_move_over_days ORDER BY day DESC, source_table")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var days []MoveOverDay
	for rows.Next() {
		var day MoveOverDay
		if err := rows.Scan(&day.Table, &day.Day, &day.State, &day.Rows); err != nil {
			return nil, err
		}
		days = append(days, day)
	}
	return days, rows.Err()
}

func (moveOverRepository) SaveProgress(ctx context.Context, tx *sql.Tx, day MoveOverDay) error {
	query, args, err := lit.ParseNamedQuery(db.Driver, `
		INSERT INTO v2_move_over_days (source_table, day, state, moved_rows, updated_at)
		VALUES (:table, :day, :state, :rows, :updated_at)
		ON CONFLICT (source_table, day) DO UPDATE
		SET state = excluded.state, moved_rows = excluded.moved_rows, updated_at = excluded.updated_at`, lit.P{
		"table": day.Table, "day": day.Day, "state": day.State, "rows": day.Rows,
		"updated_at": time.Now().UTC().Format(time.RFC3339Nano),
	})
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, query, args...)
	return err
}

var MoveOverRepository = moveOverRepository{}
