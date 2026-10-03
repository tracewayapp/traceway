//go:build !transactional_pg

package middleware

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"slices"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/tracewayapp/traceway/backend/app/db"
	_ "modernc.org/sqlite"
)

var errSimulatedCommit = errors.New("simulated commit failure")

// failCommitConnector hands out one real in-memory sqlite connection whose
// transactions fail to commit when failCommit is set. A failed commit rolls
// back for real so the connection stays clean; returning driver.ErrBadConn
// instead would make database/sql discard the connection and, with it, the
// in-memory database, so a "0 rows" assertion would pass vacuously.
type failCommitConnector struct {
	drv        driver.Driver
	failCommit bool
}

func (c *failCommitConnector) Connect(context.Context) (driver.Conn, error) {
	conn, err := c.drv.Open(":memory:")
	if err != nil {
		return nil, err
	}
	return &failCommitConn{Conn: conn, failCommit: c.failCommit}, nil
}

func (c *failCommitConnector) Driver() driver.Driver { return c.drv }

type failCommitConn struct {
	driver.Conn
	failCommit bool
}

func (c *failCommitConn) BeginTx(ctx context.Context, opts driver.TxOptions) (driver.Tx, error) {
	tx, err := c.Conn.(driver.ConnBeginTx).BeginTx(ctx, opts)
	if err != nil {
		return nil, err
	}
	return &failCommitTx{Tx: tx, failCommit: c.failCommit}, nil
}

type failCommitTx struct {
	driver.Tx
	failCommit bool
}

func (t *failCommitTx) Commit() error {
	if t.failCommit {
		_ = t.Tx.Rollback()
		return errSimulatedCommit
	}
	return t.Tx.Commit()
}

func setupTransactionalDB(t *testing.T, failCommit bool) {
	t.Helper()

	base, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	connector := &failCommitConnector{drv: base.Driver(), failCommit: failCommit}
	base.Close()

	mainDB := sql.OpenDB(connector)
	mainDB.SetMaxOpenConns(1)

	prev := db.DB
	db.DB = mainDB
	t.Cleanup(func() {
		mainDB.Close()
		db.DB = prev
	})

	if _, err := mainDB.Exec(`CREATE TABLE things (id INTEGER PRIMARY KEY AUTOINCREMENT, name TEXT NOT NULL)`); err != nil {
		t.Fatalf("create table: %v", err)
	}
}

func insertThing(t *testing.T, c *gin.Context, name string) {
	t.Helper()
	if _, err := db.GetTx(c).Exec(`INSERT INTO things (name) VALUES (?)`, name); err != nil {
		t.Fatalf("insert %q: %v", name, err)
	}
}

// assertRows queries the table the setup created: a replaced in-memory
// database would have no such table and fail here instead of counting zero.
func assertRows(t *testing.T, name string, present bool) {
	t.Helper()
	var n int
	if err := db.DB.QueryRow(`SELECT count(*) FROM things WHERE name = ?`, name).Scan(&n); err != nil {
		t.Fatalf("count %q: %v", name, err)
	}
	want := 0
	if present {
		want = 1
	}
	if n != want {
		t.Fatalf("%q rows = %d, want %d", name, n, want)
	}
}

const outerHeader = "X-Outer-Middleware"

type transactionalObservations struct {
	recovered any
	errors    []*gin.Error
}

// newTransactionalRouter mirrors the production chain: recovery outermost, a
// middleware that sets a header before the handler and reads c.Errors after
// it, then Transactional and the handler.
func newTransactionalRouter(handler gin.HandlerFunc) (*gin.Engine, *transactionalObservations) {
	gin.SetMode(gin.TestMode)
	obs := &transactionalObservations{}
	r := gin.New()
	r.Use(gin.CustomRecoveryWithWriter(io.Discard, func(c *gin.Context, err any) {
		obs.recovered = err
		c.AbortWithStatus(http.StatusInternalServerError)
	}))
	r.Use(func(c *gin.Context) {
		c.Header(outerHeader, "set")
		c.Next()
		obs.errors = append([]*gin.Error(nil), c.Errors...)
	})
	r.Any("/tx", Transactional, handler)
	return r, obs
}

// headerWriteObserver reports the moment the first status line reaches the
// underlying writer, so ordering against commit hooks is observable.
type headerWriteObserver struct {
	*httptest.ResponseRecorder
	onFirstWriteHeader func()
}

func (w *headerWriteObserver) WriteHeader(code int) {
	if w.onFirstWriteHeader != nil {
		w.onFirstWriteHeader()
		w.onFirstWriteHeader = nil
	}
	w.ResponseRecorder.WriteHeader(code)
}

func serve(r *gin.Engine, method string) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(method, "/tx", nil))
	return rec
}

func TestTransactionalFailedCommitAnswers500WithoutHandlerResponse(t *testing.T) {
	setupTransactionalDB(t, true)
	hookRan := false
	r, obs := newTransactionalRouter(func(c *gin.Context) {
		insertThing(t, c, "row")
		OnCommit(c, func() { hookRan = true })
		c.Header("Location", "/somewhere")
		c.JSON(http.StatusCreated, gin.H{"ok": true})
	})

	rec := serve(r, http.MethodPost)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", rec.Code)
	}
	if rec.Body.Len() != 0 {
		t.Fatalf("body = %q, want empty", rec.Body.String())
	}
	for _, h := range []string{"Content-Type", "Location"} {
		if v := rec.Header().Get(h); v != "" {
			t.Fatalf("%s = %q, want unset", h, v)
		}
	}
	if v := rec.Header().Get(outerHeader); v != "set" {
		t.Fatalf("%s = %q, want the outer middleware's value", outerHeader, v)
	}
	assertRows(t, "row", false)
	if hookRan {
		t.Fatal("commit hook ran after a failed commit")
	}
	if obs.recovered != nil {
		t.Fatalf("panic reached recovery: %v", obs.recovered)
	}
	if len(obs.errors) != 1 || !errors.Is(obs.errors[0].Err, errSimulatedCommit) {
		t.Fatalf("c.Errors = %v, want one entry wrapping the commit error", obs.errors)
	}
}

func TestTransactionalReleasesResponseAfterCommit(t *testing.T) {
	cases := []struct {
		name        string
		method      string
		render      func(c *gin.Context)
		wantStatus  int
		wantBody    string
		wantHeaders map[string]string
	}{
		{
			name:       "json",
			method:     http.MethodPost,
			render:     func(c *gin.Context) { c.JSON(http.StatusCreated, gin.H{"ok": true}) },
			wantStatus: http.StatusCreated,
			wantBody:   `{"ok":true}`,
			wantHeaders: map[string]string{
				"Content-Type": "application/json; charset=utf-8",
			},
		},
		{
			name:        "redirect",
			method:      http.MethodGet,
			render:      func(c *gin.Context) { c.Redirect(http.StatusSeeOther, "/next") },
			wantStatus:  http.StatusSeeOther,
			wantBody:    "<a href=\"/next\">See Other</a>.\n\n",
			wantHeaders: map[string]string{"Location": "/next"},
		},
		{
			name:       "bare status",
			method:     http.MethodDelete,
			render:     func(c *gin.Context) { c.Status(http.StatusNoContent) },
			wantStatus: http.StatusNoContent,
		},
		{
			name:       "json no content",
			method:     http.MethodDelete,
			render:     func(c *gin.Context) { c.JSON(http.StatusNoContent, nil) },
			wantStatus: http.StatusNoContent,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			setupTransactionalDB(t, false)
			var events []string
			r, obs := newTransactionalRouter(func(c *gin.Context) {
				insertThing(t, c, "row")
				OnCommit(c, func() { events = append(events, "hook") })
				tc.render(c)
			})

			rec := &headerWriteObserver{ResponseRecorder: httptest.NewRecorder()}
			rec.onFirstWriteHeader = func() { events = append(events, "header") }
			r.ServeHTTP(rec, httptest.NewRequest(tc.method, "/tx", nil))

			if rec.Code != tc.wantStatus {
				t.Fatalf("status = %d, want %d", rec.Code, tc.wantStatus)
			}
			if got := rec.Body.String(); got != tc.wantBody {
				t.Fatalf("body = %q, want %q", got, tc.wantBody)
			}
			for h, want := range tc.wantHeaders {
				if got := rec.Header().Get(h); got != want {
					t.Fatalf("%s = %q, want %q", h, got, want)
				}
			}
			if v := rec.Header().Get(outerHeader); v != "set" {
				t.Fatalf("%s = %q, want the outer middleware's value", outerHeader, v)
			}
			assertRows(t, "row", true)
			if !slices.Equal(events, []string{"hook", "header"}) {
				t.Fatalf("events = %v, want the hook to run before the first header write", events)
			}
			if obs.recovered != nil || len(obs.errors) != 0 {
				t.Fatalf("recovered = %v, c.Errors = %v, want neither", obs.recovered, obs.errors)
			}
		})
	}
}

func TestTransactionalPanicAfterWriteAnswers500(t *testing.T) {
	setupTransactionalDB(t, false)
	r, obs := newTransactionalRouter(func(c *gin.Context) {
		insertThing(t, c, "row")
		c.JSON(http.StatusCreated, gin.H{"ok": true})
		panic("boom")
	})

	rec := serve(r, http.MethodPost)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", rec.Code)
	}
	if rec.Body.Len() != 0 {
		t.Fatalf("body = %q, want empty", rec.Body.String())
	}
	if v := rec.Header().Get("Content-Type"); v != "" {
		t.Fatalf("Content-Type = %q, want unset", v)
	}
	assertRows(t, "row", false)
	if obs.recovered != "boom" {
		t.Fatalf("recovered = %v, want the handler's panic", obs.recovered)
	}
}

func TestTransactionalPanickingCommitHookDoesNotStopOthers(t *testing.T) {
	setupTransactionalDB(t, false)
	secondRan := false
	r, obs := newTransactionalRouter(func(c *gin.Context) {
		insertThing(t, c, "row")
		OnCommit(c, func() { panic("hook boom") })
		OnCommit(c, func() { secondRan = true })
		c.JSON(http.StatusCreated, gin.H{"ok": true})
	})

	rec := serve(r, http.MethodPost)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201", rec.Code)
	}
	if got := rec.Body.String(); got != `{"ok":true}` {
		t.Fatalf("body = %q, want the handler's JSON", got)
	}
	assertRows(t, "row", true)
	if !secondRan {
		t.Fatal("second commit hook did not run after the first panicked")
	}
	if obs.recovered != nil {
		t.Fatalf("hook panic reached recovery: %v", obs.recovered)
	}
	if len(obs.errors) != 1 {
		t.Fatalf("c.Errors = %v, want exactly one entry for the panicking hook", obs.errors)
	}
}

type deadlineWriter struct{ http.ResponseWriter }

func (deadlineWriter) SetReadDeadline(time.Time) error { return nil }

// httptest.ResponseRecorder has no SetReadDeadline, so the controller only
// succeeds if it unwraps through the buffer down to deadlineWriter.
func TestResponseBufferUnwrapKeepsResponseControllerWorking(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(deadlineWriter{httptest.NewRecorder()})
	bufferResponse(c)

	if err := http.NewResponseController(c.Writer).SetReadDeadline(time.Time{}); err != nil {
		t.Fatalf("SetReadDeadline through the buffer: %v", err)
	}
}
