package export

import (
	"context"
	"database/sql"
	"encoding/json"
)

// SQLRows loads report rows from PostgreSQL.
type SQLRows struct{ db *sql.DB }

// NewSQLRows returns a SQLRows.
func NewSQLRows(db *sql.DB) *SQLRows { return &SQLRows{db: db} }

// Rows runs the report's saved query and returns its result.
func (s *SQLRows) Rows(ctx context.Context, reportID int64) ([]string, []Row, error) {
	var colsJSON string
	if err := s.db.QueryRowContext(ctx, `SELECT columns FROM reports WHERE id = $1`, reportID).Scan(&colsJSON); err != nil {
		return nil, nil, err
	}
	var cols []string
	if err := json.Unmarshal([]byte(colsJSON), &cols); err != nil {
		return nil, nil, err
	}
	rs, err := s.db.QueryContext(ctx, `SELECT values FROM report_rows WHERE report_id = $1 ORDER BY position`, reportID)
	if err != nil {
		return nil, nil, err
	}
	defer rs.Close()
	var rows []Row
	for rs.Next() {
		var raw string
		if err := rs.Scan(&raw); err != nil {
			return nil, nil, err
		}
		var r Row
		if err := json.Unmarshal([]byte(raw), &r.Values); err != nil {
			return nil, nil, err
		}
		rows = append(rows, r)
	}
	return cols, rows, rs.Err()
}
