package middleware

import (
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

func TestBodyCutOffTellsAHangUpFromABadBody(t *testing.T) {
	gin.SetMode(gin.TestMode)
	cutOffs := make(chan bool, 1)
	r := gin.New()
	r.POST("/ingest", func(c *gin.Context) {
		GuardBodyRead(c, 2*time.Second, time.Minute)
		var v map[string]any
		if err := json.NewDecoder(c.Request.Body).Decode(&v); err == nil {
			t.Errorf("decode succeeded, want a bind error")
		}
		cutOffs <- bodyCutOff(c)
		c.Status(http.StatusBadRequest)
	})
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	srv := &http.Server{Handler: r, ReadHeaderTimeout: 2 * time.Second}
	go srv.Serve(ln)
	t.Cleanup(func() { srv.Close() })

	cases := []struct {
		name    string
		request string
		want    bool
	}{
		{"hang-up before the declared length", "POST /ingest HTTP/1.1\r\nHost: x\r\nContent-Length: 1000\r\n\r\n{\"frames\":\"abc", true},
		{"hang-up inside a chunked body", "POST /ingest HTTP/1.1\r\nHost: x\r\nTransfer-Encoding: chunked\r\n\r\nd\r\n{\"frames\":\"ab\r\n", true},
		{"complete but malformed body", "POST /ingest HTTP/1.1\r\nHost: x\r\nContent-Length: 11\r\n\r\n{\"frames\":}", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			conn, err := net.Dial("tcp", ln.Addr().String())
			if err != nil {
				t.Fatal(err)
			}
			fmt.Fprint(conn, tc.request)
			if tc.want {
				conn.Close()
			} else {
				defer conn.Close()
			}
			select {
			case got := <-cutOffs:
				if got != tc.want {
					t.Fatalf("bodyCutOff = %v, want %v", got, tc.want)
				}
			case <-time.After(5 * time.Second):
				t.Fatal("handler never finished")
			}
		})
	}
}
