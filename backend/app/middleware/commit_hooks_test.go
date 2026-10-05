//go:build !transactional_pg && !telemetry_ch && !telemetry_duckdb

package middleware

import (
	"database/sql"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/tracewayapp/traceway/backend/app/db"
	_ "modernc.org/sqlite"
)

func newCommitHookTestDB(t *testing.T) *sql.DB {
	t.Helper()
	conn, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	conn.SetMaxOpenConns(1)
	t.Cleanup(func() { conn.Close() })

	if _, err := conn.Exec("CREATE TABLE things (id INTEGER PRIMARY KEY)"); err != nil {
		t.Fatalf("create table: %v", err)
	}

	prev := db.DB
	db.DB = conn
	t.Cleanup(func() { db.DB = prev })
	return conn
}

// A queued side effect must fire only when the write it describes is durable,
// and must be dropped otherwise. Callers rely on this to publish state that
// outlives the request — cache.ProjectCache entries, for one, which would
// survive a rollback and then authenticate a project that does not exist.
func TestOnCommitRunsQueuedHooksOnlyAfterCommit(t *testing.T) {
	gin.SetMode(gin.TestMode)

	for _, tc := range []struct {
		name       string
		status     int
		wantHook   bool
		wantCommit bool
	}{
		{name: "committed", status: http.StatusCreated, wantHook: true, wantCommit: true},
		{name: "rolled back", status: http.StatusInternalServerError, wantHook: false, wantCommit: false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			conn := newCommitHookTestDB(t)

			ran := 0
			r := gin.New()
			r.POST("/thing", Transactional, func(c *gin.Context) {
				tx := db.GetTx(c)
				if _, err := tx.Exec("INSERT INTO things (id) VALUES (1)"); err != nil {
					t.Fatalf("insert: %v", err)
				}
				OnCommit(c, func() { ran++ })
				c.JSON(tc.status, gin.H{})
			})

			recorder := httptest.NewRecorder()
			r.ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, "/thing", nil))

			if recorder.Code != tc.status {
				t.Fatalf("status = %d, want %d", recorder.Code, tc.status)
			}

			wantRuns := 0
			if tc.wantHook {
				wantRuns = 1
			}
			if ran != wantRuns {
				t.Errorf("hook ran %d times, want %d", ran, wantRuns)
			}

			var rows int
			if err := conn.QueryRow("SELECT COUNT(*) FROM things").Scan(&rows); err != nil {
				t.Fatalf("count: %v", err)
			}
			wantRows := 0
			if tc.wantCommit {
				wantRows = 1
			}
			if rows != wantRows {
				t.Errorf("rows = %d, want %d; the hook and the write must agree", rows, wantRows)
			}
		})
	}
}

// Hooks run in the order they were queued, so a later hook may depend on what
// an earlier one published.
func TestOnCommitRunsHooksInOrder(t *testing.T) {
	gin.SetMode(gin.TestMode)
	newCommitHookTestDB(t)

	var order []int
	r := gin.New()
	r.POST("/thing", Transactional, func(c *gin.Context) {
		for i := range 3 {
			OnCommit(c, func() { order = append(order, i) })
		}
		c.JSON(http.StatusOK, gin.H{})
	})

	recorder := httptest.NewRecorder()
	r.ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, "/thing", nil))

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d", recorder.Code)
	}
	if len(order) != 3 || order[0] != 0 || order[1] != 1 || order[2] != 2 {
		t.Errorf("hook order = %v, want [0 1 2]", order)
	}
}
