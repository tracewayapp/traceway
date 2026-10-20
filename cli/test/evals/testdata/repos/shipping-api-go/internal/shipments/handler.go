package shipments

import (
	"encoding/json"
	"fmt"
	"net/http"

	"example.com/shipping-api/internal/httpx"
)

// Handler serves the shipments API.
type Handler struct{ svc *Service }

// NewHandler returns a Handler.
func NewHandler(svc *Service) *Handler { return &Handler{svc: svc} }

// Create serves POST /api/shipments.
func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	var req CreateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.Error(w, r, fmt.Errorf("%w: %v", httpx.ErrInvalid, err))
		return
	}
	sh, err := h.svc.Create(r.Context(), req)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusCreated, sh)
}

// Get serves GET /api/shipments/{id}.
func (h *Handler) Get(w http.ResponseWriter, r *http.Request) {
	sh, err := h.svc.Get(r.Context(), r.PathValue("id"))
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	if sh == nil {
		httpx.JSON(w, http.StatusNotFound, map[string]string{"error": "shipment not found"})
		return
	}
	httpx.JSON(w, http.StatusOK, sh)
}
