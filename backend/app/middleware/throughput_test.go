package middleware

import (
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

func guardedServer(t *testing.T, idle, total time.Duration) (string, chan error) {
	t.Helper()
	gin.SetMode(gin.TestMode)

	prev := throughputGracePeriod
	throughputGracePeriod = 500 * time.Millisecond
	t.Cleanup(func() { throughputGracePeriod = prev })

	readErrs := make(chan error, 8)
	r := gin.New()
	r.POST("/ingest", func(c *gin.Context) {
		GuardBodyRead(c, idle, total)
		_, err := io.Copy(io.Discard, c.Request.Body)
		readErrs <- err
		c.Status(http.StatusOK)
	})

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	srv := &http.Server{Handler: r, ReadHeaderTimeout: 2 * time.Second, ReadTimeout: 3 * time.Second}
	go srv.Serve(ln)
	t.Cleanup(func() { srv.Close() })
	return ln.Addr().String(), readErrs
}

func TestGuardBodyReadRejectsADripClient(t *testing.T) {
	addr, readErrs := guardedServer(t, 2*time.Second, time.Minute)

	conn, err := net.Dial("tcp", addr)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	fmt.Fprint(conn, "POST /ingest HTTP/1.1\r\nHost: x\r\nContent-Length: 10000000\r\n\r\n")

	start := time.Now()
	go func() {
		for i := 0; i < 300; i++ {
			if _, err := conn.Write([]byte("a")); err != nil {
				return
			}
			time.Sleep(100 * time.Millisecond)
		}
	}()

	select {
	case err := <-readErrs:
		if !errors.Is(err, ErrBodyTooSlow) {
			t.Fatalf("read ended with %v, want ErrBodyTooSlow", err)
		}
		if elapsed := time.Since(start); elapsed > throughputGracePeriod+3*time.Second {
			t.Fatalf("drip client held the slot for %v", elapsed)
		}
	case <-time.After(throughputGracePeriod + 5*time.Second):
		t.Fatal("drip client was never cut off")
	}
}

func TestGuardBodyReadAcceptsASlowButRealClient(t *testing.T) {
	addr, readErrs := guardedServer(t, 3*time.Second, time.Minute)

	const total = 64 << 10
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	fmt.Fprintf(conn, "POST /ingest HTTP/1.1\r\nHost: x\r\nContent-Length: %d\r\n\r\n", total)

	chunk := make([]byte, 8<<10)
	go func() {
		for sent := 0; sent < total; sent += len(chunk) {
			if _, err := conn.Write(chunk); err != nil {
				return
			}
			time.Sleep(100 * time.Millisecond)
		}
	}()

	select {
	case err := <-readErrs:
		if err != nil {
			t.Fatalf("a legitimate 80KB/s client was rejected: %v", err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("timed out")
	}
}

func TestGuardBodyReadAcceptsACompleteBodyUnderTheFloor(t *testing.T) {
	addr, readErrs := guardedServer(t, 3*time.Second, time.Minute)

	const total = 1 << 10
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	fmt.Fprintf(conn, "POST /ingest HTTP/1.1\r\nHost: x\r\nContent-Length: %d\r\n\r\n", total)

	go func() {
		half := make([]byte, total/2)
		if _, err := conn.Write(half); err != nil {
			return
		}
		time.Sleep(1200 * time.Millisecond)
		conn.Write(half)
	}()

	select {
	case err := <-readErrs:
		if err != nil {
			t.Fatalf("a complete slow body was rejected: %v", err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("timed out")
	}
}

func TestGuardBodyReadClearsTheDeadlineOnceTheBodyIsComplete(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ctxErrs := make(chan error, 1)
	r := gin.New()
	r.POST("/slow-handler", func(c *gin.Context) {
		GuardBodyRead(c, 300*time.Millisecond, time.Minute)
		if _, err := io.Copy(io.Discard, c.Request.Body); err != nil {
			ctxErrs <- err
			return
		}
		select {
		case <-c.Request.Context().Done():
			ctxErrs <- c.Request.Context().Err()
		case <-time.After(time.Second):
			ctxErrs <- nil
		}
		c.Status(http.StatusOK)
	})
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	srv := &http.Server{Handler: r}
	go srv.Serve(ln)
	t.Cleanup(func() { srv.Close() })

	conn, err := net.Dial("tcp", ln.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	const total = 2 << 10
	fmt.Fprintf(conn, "POST /slow-handler HTTP/1.1\r\nHost: x\r\nContent-Length: %d\r\n\r\n", total)
	half := make([]byte, total/2)
	conn.Write(half)
	time.Sleep(150 * time.Millisecond)
	conn.Write(half)

	if err := <-ctxErrs; err != nil {
		t.Fatalf("a handler outliving the idle window after the body arrived lost its context: %v", err)
	}
}

func drainInSmallReads(r io.Reader) error {
	buf := make([]byte, 64)
	for {
		if _, err := r.Read(buf); err != nil {
			if errors.Is(err, io.EOF) {
				return nil
			}
			return err
		}
	}
}

func sendWholeBody(t *testing.T, addr, path string, size int) {
	t.Helper()
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { conn.Close() })
	fmt.Fprintf(conn, "POST %s HTTP/1.1\r\nHost: x\r\nContent-Length: %d\r\n\r\n", path, size)
	go conn.Write(make([]byte, size))
}

func TestGuardBodyReadRestartsTheClockWhenRearmed(t *testing.T) {
	gin.SetMode(gin.TestMode)
	prev := throughputGracePeriod
	throughputGracePeriod = 500 * time.Millisecond
	t.Cleanup(func() { throughputGracePeriod = prev })

	readErrs := make(chan error, 1)
	r := gin.New()
	r.Use(GuardBodyReads(DefaultBodyIdle, DefaultBodyTotal))
	r.POST("/upload", func(c *gin.Context) {
		time.Sleep(700 * time.Millisecond)
		GuardBodyRead(c, DefaultBodyIdle, DefaultBodyTotal)
		readErrs <- drainInSmallReads(c.Request.Body)
		c.Status(http.StatusOK)
	})
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	srv := &http.Server{Handler: r}
	go srv.Serve(ln)
	t.Cleanup(func() { srv.Close() })

	sendWholeBody(t, ln.Addr().String(), "/upload", 200<<10)

	select {
	case err := <-readErrs:
		if err != nil {
			t.Fatalf("a body that arrived at once was rejected after the handler waited before reading: %v", err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("timed out")
	}
}

func TestAdmissionGateRestartsTheGuardAfterWaiting(t *testing.T) {
	gin.SetMode(gin.TestMode)
	prev := throughputGracePeriod
	throughputGracePeriod = 500 * time.Millisecond
	t.Cleanup(func() { throughputGracePeriod = prev })

	readErrs := make(chan error, 1)
	r := gin.New()
	r.Use(GuardBodyReads(DefaultBodyIdle, DefaultBodyTotal))
	gate := newAdmissionGate(1, 5*time.Second, "saturated", nil, "2")
	r.POST("/hold", gate, func(c *gin.Context) {
		time.Sleep(700 * time.Millisecond)
		c.Status(http.StatusOK)
	})
	r.POST("/upload", gate, func(c *gin.Context) {
		readErrs <- drainInSmallReads(c.Request.Body)
		c.Status(http.StatusOK)
	})
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	srv := &http.Server{Handler: r}
	go srv.Serve(ln)
	t.Cleanup(func() { srv.Close() })

	sendWholeBody(t, ln.Addr().String(), "/hold", 0)
	time.Sleep(50 * time.Millisecond)
	sendWholeBody(t, ln.Addr().String(), "/upload", 200<<10)

	select {
	case err := <-readErrs:
		if err != nil {
			t.Fatalf("a body that waited in the admission gate was rejected: %v", err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("timed out")
	}
}

func TestGuardBodyReadLiftsTheServerReadTimeoutForBodilessRequests(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ctxErrs := make(chan error, 1)
	r := gin.New()
	r.Use(GuardBodyReads(DefaultBodyIdle, DefaultBodyTotal))
	r.GET("/stream", func(c *gin.Context) {
		select {
		case <-c.Request.Context().Done():
			ctxErrs <- c.Request.Context().Err()
		case <-time.After(1500 * time.Millisecond):
			ctxErrs <- nil
		}
		c.Status(http.StatusOK)
	})
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	srv := &http.Server{Handler: r, ReadTimeout: time.Second}
	go srv.Serve(ln)
	t.Cleanup(func() { srv.Close() })

	conn, err := net.Dial("tcp", ln.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	fmt.Fprint(conn, "GET /stream HTTP/1.1\r\nHost: x\r\n\r\n")

	if err := <-ctxErrs; err != nil {
		t.Fatalf("a bodiless handler outliving ReadTimeout lost its context: %v", err)
	}
}
