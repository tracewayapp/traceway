package invoices

import (
	"context"
	"net/http"
	"strconv"
	"time"

	"example.com/billing-api/internal/mail"
	"example.com/billing-api/internal/telemetry"
)

// Mailer sends e-mails.
type Mailer interface {
	Send(ctx context.Context, m mail.Message) error
}

// Sender e-mails invoices to their customers.
type Sender struct {
	store Store
	mail  Mailer
}

// NewSender returns a Sender.
func NewSender(s Store, m Mailer) *Sender { return &Sender{store: s, mail: m} }

// Send serves POST /api/invoices/{id}/send.
func (s *Sender) Send(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.Error(w, "bad id", http.StatusBadRequest)
		return
	}
	inv, err := s.store.Invoice(r.Context(), id)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	if inv == nil {
		http.NotFound(w, r)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()
	msg := mail.Message{To: inv.Customer.Email, Template: "invoice", Data: map[string]string{"number": inv.Number, "total": money(inv.TotalCents, inv.Currency)}}
	if err := s.mail.Send(ctx, msg); err != nil {
		telemetry.RecordError(ctx, err)
		http.Error(w, "could not send the invoice", http.StatusBadGateway)
		return
	}
	w.WriteHeader(http.StatusAccepted)
}
