package search

import (
	"context"
	"database/sql"
	"errors"
	"time"
)

// SQLStore implements Store on PostgreSQL.
type SQLStore struct{ db *sql.DB }

// NewStore returns the PostgreSQL store.
func NewStore(db *sql.DB) *SQLStore { return &SQLStore{db: db} }

// Candidates runs the full-text query, top 200 by ts_rank_cd.
func (s *SQLStore) Candidates(ctx context.Context, tenantID int64, q string) ([]Hit, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT d.id, d.title, d.snippet, d.updated_at, ts_rank_cd(d.tsv, q) AS rank FROM documents d, websearch_to_tsquery('simple', $1) q WHERE d.tenant_id = $2 AND d.tsv @@ q ORDER BY rank DESC LIMIT 200`, q, tenantID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	now := time.Now()
	var out []Hit
	for rows.Next() {
		var h Hit
		var updated time.Time
		if err := rows.Scan(&h.ID, &h.Title, &h.Snippet, &updated, &h.DBRank); err != nil {
			return nil, err
		}
		h.AgeDays = now.Sub(updated).Hours() / 24
		out = append(out, h)
	}
	return out, rows.Err()
}

// Suggest returns the 10 most used terms with the prefix.
func (s *SQLStore) Suggest(ctx context.Context, tenantID int64, prefix string) ([]Suggestion, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT term, hits FROM suggest_terms WHERE tenant_id = $1 AND term LIKE $2 ORDER BY hits DESC LIMIT 10`, tenantID, prefix+"%")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Suggestion{}
	for rows.Next() {
		var sg Suggestion
		if err := rows.Scan(&sg.Term, &sg.Hits); err != nil {
			return nil, err
		}
		out = append(out, sg)
	}
	return out, rows.Err()
}

// Document loads one document of the tenant.
func (s *SQLStore) Document(ctx context.Context, tenantID, id int64) (*Document, error) {
	var d Document
	err := s.db.QueryRowContext(ctx, `SELECT id, title, body, updated_at FROM documents WHERE id = $1 AND tenant_id = $2`, id, tenantID).
		Scan(&d.ID, &d.Title, &d.Body, &d.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &d, nil
}

// SaveFeedback stores a verdict on a result.
func (s *SQLStore) SaveFeedback(ctx context.Context, tenantID int64, f Feedback) error {
	_, err := s.db.ExecContext(ctx, `INSERT INTO search_feedback (tenant_id, query, document_id, verdict, created_at) VALUES ($1, $2, $3, $4, now())`,
		tenantID, f.Query, f.DocumentID, f.Verdict)
	return err
}
