package cmd

import (
	"fmt"
	"io"
	"net"
	"net/http"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

func TestReadTimeoutCutsOffSlowBodies(t *testing.T) {
	gin.SetMode(gin.TestMode)

	r := gin.New()
	r.POST("/small", func(c *gin.Context) {
		io.Copy(io.Discard, c.Request.Body)
		c.Status(http.StatusOK)
	})

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	srv := &http.Server{Handler: r, ReadHeaderTimeout: time.Second, ReadTimeout: 2 * time.Second}
	go srv.Serve(ln)
	defer srv.Close()

	conn, err := net.Dial("tcp", ln.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	fmt.Fprint(conn, "POST /small HTTP/1.1\r\nHost: x\r\nContent-Length: 50\r\n\r\n")
	start := time.Now()
	for i := 0; i < 50; i++ {
		if _, err := conn.Write([]byte("a")); err != nil {
			if elapsed := time.Since(start); elapsed > 4*time.Second {
				t.Fatalf("slow body held the connection for %v despite a 2s ReadTimeout", elapsed)
			}
			return
		}
		time.Sleep(150 * time.Millisecond)
	}
	t.Fatalf("a %v drip completed despite a 2s ReadTimeout", time.Since(start))
}
