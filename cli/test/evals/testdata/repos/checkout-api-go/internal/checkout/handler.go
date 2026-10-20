// Package checkout serves checkout and refunds.
package checkout

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"example.com/checkout-api/internal/payments"
	"example.com/checkout-api/internal/telemetry"
)

// Store is the orders data access the handlers need.
type Store interface {
	// Order loads a customer's order for payment.
	Order(ctx context.Context, id, customerID int64) (*Order, error)
	Lines(ctx context.Context, orderID int64) ([]Line, error)
	// OrderByID loads an order for display.
	OrderByID(ctx context.Context, id int64) (*Order, error)
	// PaidOrder loads an order with its charge, for a refund.
	PaidOrder(ctx context.Context, id int64) (*Order, error)
	MarkPaid(ctx context.Context, id int64, chargeID string) error
	RecordRefund(ctx context.Context, id int64) error
}

// Order is one order.
type Order struct {
	ID            int64
	Status        string
	TotalCents    int64
	Currency      string
	PaymentMethod string
	ChargeID      string
	PaidAt        *time.Time
}

// Line is one order line.
type Line struct {
	SKU            string `json:"sku"`
	Qty            int64  `json:"qty"`
	UnitPriceCents int64  `json:"unitPriceCents"`
}

// Payments is the payment provider.
type Payments interface {
	Charge(ctx context.Context, req payments.ChargeRequest) (*payments.Charge, error)
	Refund(ctx context.Context, chargeID, idempotencyKey string) error
}

// Handler serves the checkout routes.
type Handler struct {
	Store    Store
	Payments Payments
}

type checkoutRequest struct {
	OrderID    int64 `json:"orderId"`
	CustomerID int64 `json:"customerId"`
}

// Checkout serves POST /api/checkout.
func (h *Handler) Checkout(w http.ResponseWriter, r *http.Request) {
	var in checkoutRequest
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	o, err := h.Store.Order(r.Context(), in.OrderID, in.CustomerID)
	if err != nil || o == nil || o.Status != "pending" {
		http.Error(w, "order not payable", http.StatusConflict)
		return
	}
	lines, err := h.Store.Lines(r.Context(), o.ID)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	charge, err := h.Payments.Charge(r.Context(), payments.ChargeRequest{AmountMinor: o.TotalCents, Currency: o.Currency, PaymentMethod: o.PaymentMethod, IdempotencyKey: fmt.Sprintf("order-%d", o.ID)})
	if err != nil {
		telemetry.RecordError(r.Context(), err)
		slog.ErrorContext(r.Context(), "charge failed", "error", err)
		http.Error(w, "payment failed, please try again", http.StatusBadGateway)
		return
	}
	if err := h.Store.MarkPaid(r.Context(), o.ID, charge.ID); err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"orderId": o.ID, "status": "paid", "chargeId": charge.ID, "lines": lines})
}

type refundRequest struct {
	OrderID int64 `json:"orderId"`
}

// Refund serves POST /api/refunds.
func (h *Handler) Refund(w http.ResponseWriter, r *http.Request) {
	var in refundRequest
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	o, err := h.Store.PaidOrder(r.Context(), in.OrderID)
	if err != nil || o == nil || o.Status != "paid" {
		http.Error(w, "order not refundable", http.StatusConflict)
		return
	}
	if err := h.Payments.Refund(r.Context(), o.ChargeID, fmt.Sprintf("refund-%d", o.ID)); err != nil {
		http.Error(w, "refund failed, please try again", http.StatusBadGateway)
		return
	}
	if err := h.Store.RecordRefund(r.Context(), o.ID); err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"orderId": o.ID, "status": "refunded"})
}

// Get serves GET /api/orders/{id}.
func (h *Handler) Get(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.Error(w, "bad order id", http.StatusBadRequest)
		return
	}
	o, err := h.Store.OrderByID(r.Context(), id)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	if o == nil {
		http.NotFound(w, r)
		return
	}
	lines, err := h.Store.Lines(r.Context(), o.ID)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"orderId": o.ID, "status": o.Status, "totalCents": o.TotalCents, "currency": o.Currency, "paidAt": o.PaidAt, "lines": lines})
}
