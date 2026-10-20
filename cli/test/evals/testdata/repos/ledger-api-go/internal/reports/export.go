// Package reports serves the finance exports.
package reports

import (
	"context"
	"database/sql"
	"encoding/csv"
	"net/http"
	"strconv"
	"time"
)

// Entry is one ledger line as exported.
type Entry struct {
	ID           int64
	PostedAt     time.Time
	AccountCode  string
	Counterparty string
	Description  string
	AmountCents  int64
	Currency     string
}

// Source yields the entries of an organization in a range, in posting order.
type Source interface {
	Entries(ctx context.Context, orgID int64, from, to time.Time, each func(Entry) error) error
}

// Store reads entries from PostgreSQL.
type Store struct{ db *sql.DB }

// NewStore returns a Source backed by db.
func NewStore(db *sql.DB) *Store { return &Store{db: db} }

// Entries streams the range with one query; rows are written as they are
// scanned, so memory stays flat whatever the range holds.
func (s *Store) Entries(ctx context.Context, orgID int64, from, to time.Time, each func(Entry) error) error {
	rows, err := s.db.QueryContext(ctx, `SELECT id, posted_at, account_code, counterparty, description, amount_cents, currency FROM ledger_entries WHERE org_id = $1 AND posted_at >= $2 AND posted_at < $3 ORDER BY posted_at`, orgID, from, to)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var e Entry
		if err := rows.Scan(&e.ID, &e.PostedAt, &e.AccountCode, &e.Counterparty, &e.Description, &e.AmountCents, &e.Currency); err != nil {
			return err
		}
		if err := each(e); err != nil {
			return err
		}
	}
	return rows.Err()
}

// Exporter serves GET /api/reports/export.
type Exporter struct{ src Source }

// NewExporter returns the export handler.
func NewExporter(src Source) *Exporter { return &Exporter{src: src} }

// Export writes the organization's entries between from and to as CSV.
func (x *Exporter) Export(w http.ResponseWriter, r *http.Request) {
	orgID, err := strconv.ParseInt(r.Header.Get("X-Org-Id"), 10, 64)
	if err != nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	from, err1 := time.Parse(time.DateOnly, r.URL.Query().Get("from"))
	to, err2 := time.Parse(time.DateOnly, r.URL.Query().Get("to"))
	if err1 != nil || err2 != nil || !to.After(from) {
		http.Error(w, "from and to must be dates, from before to", http.StatusBadRequest)
		return
	}
	w.Header().Set("Content-Type", "text/csv")
	w.Header().Set("Content-Disposition", `attachment; filename="ledger.csv"`)
	cw := csv.NewWriter(w)
	_ = cw.Write([]string{"id", "posted_at", "account", "counterparty", "description", "amount", "currency"})
	err = x.src.Entries(r.Context(), orgID, from, to, func(e Entry) error {
		return cw.Write([]string{
			strconv.FormatInt(e.ID, 10), e.PostedAt.UTC().Format(time.RFC3339), e.AccountCode, e.Counterparty,
			e.Description, formatCents(e.AmountCents), e.Currency,
		})
	})
	cw.Flush()
	if err != nil {
		// Headers are already sent; the truncated file fails the client's checksum.
		return
	}
}

func formatCents(c int64) string {
	sign := ""
	if c < 0 {
		sign, c = "-", -c
	}
	frac := strconv.FormatInt(c%100, 10)
	if len(frac) == 1 {
		frac = "0" + frac
	}
	return sign + strconv.FormatInt(c/100, 10) + "." + frac
}
