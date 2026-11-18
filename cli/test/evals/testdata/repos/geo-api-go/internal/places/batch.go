package places

import (
	"encoding/json"
	"net/http"
	"strings"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
)

const maxBatch = 5000

type batchRequest struct {
	Addresses []string `json:"addresses"`
}

type batchResult struct {
	Index int     `json:"index"`
	ID    int64   `json:"id,omitempty"`
	Lat   float64 `json:"lat,omitempty"`
	Lon   float64 `json:"lon,omitempty"`
	Found bool    `json:"found"`
}

// batch geocodes up to maxBatch addresses in one query, for CSV imports.
func (h *Handler) batch(w http.ResponseWriter, r *http.Request) {
	_, span := otel.Tracer("geo-api").Start(r.Context(), "decode batch")
	var req batchRequest
	err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4<<20)).Decode(&req)
	span.SetAttributes(attribute.Int("batch.size", len(req.Addresses)))
	span.End()
	if err != nil {
		http.Error(w, "invalid body", http.StatusBadRequest)
		return
	}
	if len(req.Addresses) == 0 || len(req.Addresses) > maxBatch {
		http.Error(w, "between 1 and 5000 addresses", http.StatusUnprocessableEntity)
		return
	}
	normalized := make([]string, len(req.Addresses))
	for i, a := range req.Addresses {
		normalized[i] = strings.Join(strings.Fields(strings.ToLower(a)), " ")
	}
	rows, err := h.db.Query(r.Context(), `SELECT q.idx, p.id, p.lat, p.lon FROM places p JOIN unnest($1::text[]) WITH ORDINALITY AS q(addr, idx) ON p.normalized_address = q.addr`, normalized)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	defer rows.Close()
	out := make([]batchResult, len(normalized))
	for i := range out {
		out[i].Index = i
	}
	for rows.Next() {
		var idx int
		var res batchResult
		if err := rows.Scan(&idx, &res.ID, &res.Lat, &res.Lon); err != nil {
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		res.Index, res.Found = idx-1, true
		out[idx-1] = res
	}
	writeJSON(w, out)
}
