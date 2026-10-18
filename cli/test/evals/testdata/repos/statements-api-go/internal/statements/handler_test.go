package statements

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

type memStore struct {
	mu         sync.Mutex
	statements map[string]*Statement
}

func newMemStore() *memStore { return &memStore{statements: map[string]*Statement{}} }

func (m *memStore) Account(_ context.Context, id int64) (*Account, error) {
	if id != 7 {
		return nil, nil
	}
	return &Account{ID: 7, LegalName: "Acme GmbH", Currency: "EUR", BillingEmail: "ap@acme.example"}, nil
}

func (m *memStore) InvoicesForPeriod(context.Context, int64, time.Time, time.Time) ([]Invoice, error) {
	return []Invoice{{ID: 1, Number: "INV-1001", TotalCents: 12900, Status: "paid"}}, nil
}

func (m *memStore) CreateStatement(_ context.Context, st *Statement) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	cp := *st
	m.statements[st.ID] = &cp
	return nil
}

func (m *memStore) MarkReady(_ context.Context, id, key string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.statements[id].Status, m.statements[id].ObjectKey = "ready", key
	return nil
}

func (m *memStore) MarkFailed(_ context.Context, id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.statements[id].Status = "failed"
	return nil
}

func (m *memStore) Statement(_ context.Context, id string) (*Statement, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	st, ok := m.statements[id]
	if !ok {
		return nil, nil
	}
	cp := *st
	return &cp, nil
}

type fakeRenderer struct{}

func (fakeRenderer) Render(context.Context, *Account, []Invoice) ([]byte, error) {
	return []byte("%PDF-1.7"), nil
}

type fakeBucket struct {
	mu   sync.Mutex
	puts []string
}

func (b *fakeBucket) Put(_ context.Context, key string, _ []byte) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.puts = append(b.puts, key)
	return nil
}

func (b *fakeBucket) PresignGet(_ context.Context, key string, _ time.Duration) (string, error) {
	return "https://bucket.example/" + key, nil
}

func TestCreateRejectsInvalidPeriod(t *testing.T) {
	mux := http.NewServeMux()
	New(newMemStore(), fakeRenderer{}, &fakeBucket{}).Routes(mux)
	req := httptest.NewRequest(http.MethodPost, "/api/statements", strings.NewReader(`{"accountId":7,"periodStart":"2026-10-01T00:00:00Z","periodEnd":"2026-09-01T00:00:00Z"}`))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status %d, want 422", rec.Code)
	}
}

func TestStatementBecomesDownloadable(t *testing.T) {
	store, bucket := newMemStore(), &fakeBucket{}
	mux := http.NewServeMux()
	New(store, fakeRenderer{}, bucket).Routes(mux)
	req := httptest.NewRequest(http.MethodPost, "/api/statements", strings.NewReader(`{"accountId":7,"periodStart":"2026-09-01T00:00:00Z","periodEnd":"2026-10-01T00:00:00Z"}`))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code/100 != 2 {
		t.Fatalf("status %d: %s", rec.Code, rec.Body.String())
	}
	var st Statement
	if err := json.NewDecoder(rec.Body).Decode(&st); err != nil || st.ID == "" {
		t.Fatalf("no statement id in %q (%v)", rec.Body.String(), err)
	}
	deadline := time.Now().Add(2 * time.Second)
	for {
		rec = httptest.NewRecorder()
		mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/statements/"+st.ID+"/download", nil))
		if rec.Code == http.StatusFound {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("statement never became downloadable (last status %d)", rec.Code)
		}
		time.Sleep(10 * time.Millisecond)
	}
}
