package checkout

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"example.com/checkout-api/internal/payments"
)

type fakeStore struct {
	order *Order
	paid  string
}

func (f *fakeStore) Order(context.Context, int64, int64) (*Order, error) { return f.order, nil }
func (f *fakeStore) OrderByID(context.Context, int64) (*Order, error)    { return f.order, nil }
func (f *fakeStore) PaidOrder(context.Context, int64) (*Order, error)    { return f.order, nil }
func (f *fakeStore) Lines(context.Context, int64) ([]Line, error)        { return nil, nil }
func (f *fakeStore) RecordRefund(context.Context, int64) error           { return nil }
func (f *fakeStore) MarkPaid(_ context.Context, _ int64, chargeID string) error {
	f.paid = chargeID
	return nil
}

type fakePayments struct{ err error }

func (f fakePayments) Charge(context.Context, payments.ChargeRequest) (*payments.Charge, error) {
	if f.err != nil {
		return nil, f.err
	}
	return &payments.Charge{ID: "ch_1", Status: "succeeded"}, nil
}

func (f fakePayments) Refund(context.Context, string, string) error { return f.err }

func TestCheckoutMarksOrderPaid(t *testing.T) {
	st := &fakeStore{order: &Order{ID: 7, Status: "pending", TotalCents: 4200, Currency: "EUR"}}
	h := &Handler{Store: st, Payments: fakePayments{}}
	rec := httptest.NewRecorder()
	h.Checkout(rec, httptest.NewRequest(http.MethodPost, "/api/checkout", strings.NewReader(`{"orderId":7,"customerId":1}`)))
	if rec.Code != http.StatusOK || st.paid != "ch_1" {
		t.Fatalf("status %d, paid %q", rec.Code, st.paid)
	}
}

func TestCheckoutReportsProviderFailure(t *testing.T) {
	st := &fakeStore{order: &Order{ID: 7, Status: "pending", TotalCents: 4200, Currency: "EUR"}}
	h := &Handler{Store: st, Payments: fakePayments{err: errors.New("boom")}}
	rec := httptest.NewRecorder()
	h.Checkout(rec, httptest.NewRequest(http.MethodPost, "/api/checkout", strings.NewReader(`{"orderId":7,"customerId":1}`)))
	if rec.Code != http.StatusBadGateway || st.paid != "" {
		t.Fatalf("status %d, paid %q", rec.Code, st.paid)
	}
}
