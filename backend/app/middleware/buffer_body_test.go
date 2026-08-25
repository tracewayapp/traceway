//go:build !transactional_pg && !telemetry_ch && !telemetry_duckdb

package middleware

import (
	"database/sql"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/tracewayapp/traceway/backend/app/db"
	_ "modernc.org/sqlite"
)

func TestTransactionalBuffersBodyBeforeOpeningTransaction(t *testing.T) {
	gin.SetMode(gin.TestMode)

	conn, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	conn.SetMaxOpenConns(1)
	defer conn.Close()

	prev := db.DB
	db.DB = conn
	t.Cleanup(func() { db.DB = prev })

	r := gin.New()
	r.POST("/personal-access-tokens", Transactional, func(c *gin.Context) {
		var body map[string]any
		_ = c.ShouldBindJSON(&body)
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})
	r.GET("/other", Transactional, func(c *gin.Context) { c.JSON(http.StatusOK, gin.H{"ok": true}) })

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	srv := &http.Server{Handler: r, ReadTimeout: 3 * time.Second}
	go srv.Serve(ln)
	defer srv.Close()
	addr := ln.Addr().String()

	slow, err := net.Dial("tcp", addr)
	if err != nil {
		t.Fatal(err)
	}
	defer slow.Close()
	fmt.Fprint(slow, "POST /personal-access-tokens HTTP/1.1\r\nHost: x\r\nContent-Type: application/json\r\nContent-Length: 2000\r\n\r\n")
	fmt.Fprint(slow, `{"name":"`)
	go func() {
		for i := 0; i < 1000; i++ {
			if _, err := slow.Write([]byte("a")); err != nil {
				return
			}
			time.Sleep(50 * time.Millisecond)
		}
	}()
	time.Sleep(300 * time.Millisecond)

	start := time.Now()
	resp, err := (&http.Client{Timeout: 5 * time.Second}).Get("http://" + addr + "/other")
	elapsed := time.Since(start)
	if err != nil {
		t.Fatalf("unrelated request failed after %v: %v", elapsed, err)
	}
	resp.Body.Close()

	if elapsed > time.Second {
		t.Fatalf("slow body held the single main-DB connection for %v", elapsed)
	}
}

func TestTransactionalRejectsOversizedBodyWithoutATransaction(t *testing.T) {
	gin.SetMode(gin.TestMode)

	prev := db.DB
	db.DB = nil
	t.Cleanup(func() { db.DB = prev })

	r := gin.New()
	r.POST("/x", Transactional, func(c *gin.Context) { c.Status(http.StatusOK) })

	body := strings.NewReader(strings.Repeat("a", maxTransactionalBodyBytes+1))
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/x", body))
	if rec.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("got %d, want 413", rec.Code)
	}
}
