// Package reports serves the reports API.
package reports

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"

	"example.com/reports-api/internal/httpx"
	"example.com/reports-api/internal/telemetry"
)

// Exporter produces report exports.
type Exporter interface {
	Export(ctx context.Context, id int64, format string) (string, error)
}

// Handler serves the reports API.
type Handler struct{ exporter Exporter }

// NewHandler returns a Handler.
func NewHandler(e Exporter) *Handler { return &Handler{exporter: e} }

type exportRequest struct {
	Format string `json:"format"`
}

// Export serves POST /api/reports/{id}/export.
func (h *Handler) Export(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		httpx.JSON(w, http.StatusBadRequest, map[string]string{"error": "bad report id"})
		return
	}
	var req exportRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || (req.Format != "csv" && req.Format != "xlsx" && req.Format != "json") {
		httpx.JSON(w, http.StatusBadRequest, map[string]string{"error": "format must be csv, xlsx or json"})
		return
	}
	telemetry.SetAttributes(r.Context(), "report.id", id, "report.format", req.Format)
	url, err := h.exporter.Export(r.Context(), id, req.Format)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]string{"url": url})
}
