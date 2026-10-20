package invoices

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
)

// Summary is one row of a customer's invoice list.
type Summary struct {
	ID       int64  `json:"id"`
	Number   string `json:"number"`
	IssuedAt string `json:"issuedAt"`
	Total    string `json:"total"`
}

// History lists a customer's invoices, newest first.
type History interface {
	ForCustomer(ctx context.Context, customerID int64) ([]Summary, error)
}

// List serves GET /api/customers/{id}/invoices.
func List(h History) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
		if err != nil {
			http.Error(w, "bad id", http.StatusBadRequest)
			return
		}
		list, err := h.ForCustomer(r.Context(), id)
		if err != nil {
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(list)
	}
}
