package tickets

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Handlers serves the event and order routes.
type Handlers struct {
	db     *pgxpool.Pool
	secret []byte
}

// NewHandlers returns the handlers; secret signs reservation tokens.
func NewHandlers(db *pgxpool.Pool, secret []byte) *Handlers {
	return &Handlers{db: db, secret: secret}
}

// Routes registers the routes.
func (h *Handlers) Routes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/events", h.Events)
	mux.HandleFunc("GET /api/events/{id}", h.Event)
	mux.HandleFunc("POST /api/orders", h.CreateOrder)
	mux.HandleFunc("GET /api/orders/{id}", h.Order)
}

// EventSummary is one row of the event listing.
type EventSummary struct {
	ID            int64     `json:"id"`
	Title         string    `json:"title"`
	VenueID       int64     `json:"venueId"`
	StartsAt      time.Time `json:"startsAt"`
	MinPriceCents int64     `json:"minPriceCents"`
}

// Events serves GET /api/events?city=: the next 30 events in a city.
func (h *Handlers) Events(w http.ResponseWriter, r *http.Request) {
	city, err := strconv.ParseInt(r.URL.Query().Get("city"), 10, 64)
	if err != nil {
		http.Error(w, "city is required", http.StatusBadRequest)
		return
	}
	rows, err := h.db.Query(r.Context(), `SELECT id, title, venue_id, starts_at, min_price_cents FROM events WHERE city_id = $1 AND starts_at > now() ORDER BY starts_at LIMIT 30`, city)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	out, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (EventSummary, error) {
		var e EventSummary
		return e, row.Scan(&e.ID, &e.Title, &e.VenueID, &e.StartsAt, &e.MinPriceCents)
	})
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

// Event serves GET /api/events/{id}: the event with its seat map.
func (h *Handlers) Event(w http.ResponseWriter, r *http.Request) {
	var ev Event
	err := h.db.QueryRow(r.Context(), `SELECT id, title, description, venue_id, starts_at FROM events WHERE id = $1`, r.PathValue("id")).
		Scan(&ev.ID, &ev.Title, &ev.Description, &ev.VenueID, &ev.StartsAt)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	rows, err := h.db.Query(r.Context(), `SELECT section, row_label, seat_count, price_cents, available FROM seat_blocks WHERE event_id = $1`, ev.ID)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	blocks, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (SeatBlock, error) {
		var b SeatBlock
		return b, row.Scan(&b.Section, &b.Row, &b.SeatCount, &b.PriceCents, &b.Available)
	})
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	ev.Sections = SeatMap(blocks)
	writeJSON(w, http.StatusOK, ev)
}

// Order is one reservation.
type Order struct {
	ID         string `json:"id"`
	EventID    int64  `json:"eventId"`
	BuyerID    int64  `json:"buyerId"`
	Seats      int    `json:"seats"`
	TotalCents int64  `json:"totalCents"`
	Status     string `json:"status"`
	Token      string `json:"token,omitempty"`
}

// CreateOrder serves POST /api/orders: holds seats in one block.
func (h *Handlers) CreateOrder(w http.ResponseWriter, r *http.Request) {
	var in struct {
		SeatBlockID int64 `json:"seatBlockId"`
		BuyerID     int64 `json:"buyerId"`
		Seats       int   `json:"seats"`
	}
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil || in.Seats < 1 {
		http.Error(w, "invalid body", http.StatusBadRequest)
		return
	}
	o := Order{ID: newID(), BuyerID: in.BuyerID, Seats: in.Seats, Status: "reserved"}
	var price int64
	err := h.db.QueryRow(r.Context(), `UPDATE seat_blocks SET available = available - $2 WHERE id = $1 AND available >= $2 RETURNING event_id, price_cents`, in.SeatBlockID, in.Seats).
		Scan(&o.EventID, &price)
	if err == pgx.ErrNoRows {
		http.Error(w, "not enough seats left", http.StatusConflict)
		return
	}
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	o.TotalCents = price * int64(in.Seats)
	if _, err := h.db.Exec(r.Context(), `INSERT INTO orders (id, event_id, buyer_id, seats, total_cents, status, created_at) VALUES ($1, $2, $3, $4, $5, 'reserved', now())`,
		o.ID, o.EventID, o.BuyerID, o.Seats, o.TotalCents); err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	o.Token = h.sign(o)
	writeJSON(w, http.StatusCreated, o)
}

// Order serves GET /api/orders/{id}.
func (h *Handlers) Order(w http.ResponseWriter, r *http.Request) {
	var o Order
	err := h.db.QueryRow(r.Context(), `SELECT id, event_id, buyer_id, seats, total_cents, status FROM orders WHERE id = $1`, r.PathValue("id")).
		Scan(&o.ID, &o.EventID, &o.BuyerID, &o.Seats, &o.TotalCents, &o.Status)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	writeJSON(w, http.StatusOK, o)
}

// sign returns the reservation token the payment page presents.
func (h *Handlers) sign(o Order) string {
	m := hmac.New(sha256.New, h.secret)
	_ = json.NewEncoder(m).Encode(o)
	return o.ID + "." + hex.EncodeToString(m.Sum(nil))
}

func newID() string {
	var b [16]byte
	_, _ = rand.Read(b[:])
	return hex.EncodeToString(b[:])
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
