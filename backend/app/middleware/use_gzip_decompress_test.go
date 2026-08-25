package middleware

import (
	"bytes"
	"compress/gzip"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

const testLimit = 64 * 1024

func gzipped(t *testing.T, data []byte) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := gzip.NewWriter(&buf)
	if _, err := zw.Write(data); err != nil {
		t.Fatal(err)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func emptyGzipStream(t *testing.T, minBytes int) []byte {
	t.Helper()
	member := gzipped(t, nil)
	var buf bytes.Buffer
	for buf.Len() <= minBytes {
		buf.Write(member)
	}
	return buf.Bytes()
}

func runUseGzip(t *testing.T, encoding string, body []byte) *gin.Context {
	t.Helper()
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/api/report", bytes.NewReader(body))
	if encoding != "" {
		c.Request.Header.Set("Content-Encoding", encoding)
	}
	useGzipLimited(c, testLimit)
	return c
}

func TestUseGzipCapsBodySize(t *testing.T) {
	cases := []struct {
		name         string
		encoding     string
		body         []byte
		wantTooLarge bool
	}{
		{"gzip bomb capped at decompressed limit", "gzip", gzipped(t, bytes.Repeat([]byte("A"), testLimit*100)), true},
		{"plain oversized body capped", "", bytes.Repeat([]byte("A"), testLimit+1), true},
		{"oversized raw gzip body capped", "gzip", emptyGzipStream(t, testLimit), true},
		{"small gzip body passes", "gzip", gzipped(t, []byte(`{"ok":true}`)), false},
		{"small plain body passes", "", []byte(`{"ok":true}`), false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c := runUseGzip(t, tc.encoding, tc.body)
			if c.IsAborted() {
				t.Fatal("middleware aborted on a well-formed body")
			}

			n, err := io.Copy(io.Discard, c.Request.Body)

			var maxBytesErr *http.MaxBytesError
			if tc.wantTooLarge {
				if !errors.As(err, &maxBytesErr) {
					t.Fatalf("expected MaxBytesError after %d bytes, got %v", n, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func TestUseGzipCapsRawBodyNotJustDecompressed(t *testing.T) {
	body := emptyGzipStream(t, testLimit)
	if len(body) <= testLimit {
		t.Fatalf("fixture must exceed the raw cap, got %d bytes for limit %d", len(body), testLimit)
	}

	c := runUseGzip(t, "gzip", body)
	n, err := io.Copy(io.Discard, c.Request.Body)

	var maxBytesErr *http.MaxBytesError
	if !errors.As(err, &maxBytesErr) {
		t.Fatalf("raw body over the cap must be rejected, got err=%v", err)
	}
	if n != 0 {
		t.Fatalf("fixture should decompress to nothing, got %d bytes", n)
	}
}
