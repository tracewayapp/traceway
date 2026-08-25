package otelcontrollers

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

type countingBody struct {
	r io.Reader
	n int64
}

func (c *countingBody) Read(p []byte) (int, error) {
	n, err := c.r.Read(p)
	c.n += int64(n)
	return n, err
}

func (c *countingBody) Close() error { return nil }

func gzipMembers(t *testing.T, minBytes int) []byte {
	t.Helper()
	var member bytes.Buffer
	zw := gzip.NewWriter(&member)
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	for buf.Len() < minBytes {
		buf.Write(member.Bytes())
	}
	return buf.Bytes()
}

func readBodyWith(t *testing.T, encoding string, body []byte) ([]byte, error, int64) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/api/otel/v1/traces", nil)
	counter := &countingBody{r: bytes.NewReader(body)}
	c.Request.Body = counter
	if encoding != "" {
		c.Request.Header.Set("Content-Encoding", encoding)
	}
	out, err := readBody(c)
	return out, err, counter.n
}

func TestReadBodyCapsRawGzipStream(t *testing.T) {
	body := gzipMembers(t, maxBodySize*4)

	out, err, consumed := readBodyWith(t, "gzip", body)

	if !errors.Is(err, errBodyTooLarge) {
		t.Fatalf("expected errBodyTooLarge, got err=%v out=%d bytes", err, len(out))
	}
	if consumed > maxBodySize+4096 {
		t.Fatalf("read %d raw bytes despite a %d-byte cap", consumed, maxBodySize)
	}
}

func TestReadBodyStillCapsDecompressedSize(t *testing.T) {
	var gz bytes.Buffer
	zw := gzip.NewWriter(&gz)
	if _, err := zw.Write(bytes.Repeat([]byte("A"), maxBodySize*3)); err != nil {
		t.Fatal(err)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}

	if _, err, _ := readBodyWith(t, "gzip", gz.Bytes()); !errors.Is(err, errBodyTooLarge) {
		t.Fatalf("expected errBodyTooLarge, got %v", err)
	}
}
