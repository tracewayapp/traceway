package controllers

import (
	"bytes"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func multipartBody(t *testing.T, parts int, size int) (*bytes.Buffer, string) {
	t.Helper()
	var body bytes.Buffer
	w := multipart.NewWriter(&body)
	for i := 0; i < parts; i++ {
		fw, err := w.CreateFormFile("files", string(rune('a'+i))+".map")
		if err != nil {
			t.Fatal(err)
		}
		if _, err := fw.Write(bytes.Repeat([]byte("x"), size)); err != nil {
			t.Fatal(err)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	return &body, w.FormDataContentType()
}

func runParseCapped(t *testing.T, body []byte, contentType string, contentLength, maxBytes int64) (bool, *httptest.ResponseRecorder) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/sourcemaps/upload", bytes.NewReader(body))
	c.Request.Header.Set("Content-Type", contentType)
	c.Request.ContentLength = contentLength
	return parseMultipartCapped(c, maxBytes, 1<<20), rec
}

func TestParseMultipartCappedAlwaysReports413WhenTruncated(t *testing.T) {
	body, ct := multipartBody(t, 8, 100)
	raw := body.Bytes()

	misreported := 0
	for limit := 0; limit < len(raw); limit++ {
		ok, rec := runParseCapped(t, raw, ct, int64(len(raw)), int64(limit))
		if ok {
			t.Fatalf("limit %d < body %d must not parse", limit, len(raw))
		}
		if rec.Code != http.StatusRequestEntityTooLarge {
			misreported++
			if misreported <= 3 {
				t.Errorf("cut at %d/%d: got %d %s, want 413", limit, len(raw), rec.Code, rec.Body.String())
			}
		}
	}
	if misreported > 0 {
		t.Fatalf("%d of %d truncation points misreported as something other than 413", misreported, len(raw))
	}
}

func TestParseMultipartCappedRejectsUnknownLengthEpilogueOverTheCap(t *testing.T) {
	body, ct := multipartBody(t, 1, 100)
	parts := body.Len()
	raw := append(body.Bytes(), bytes.Repeat([]byte("e"), 1<<20)...)

	ok, rec := runParseCapped(t, raw, ct, -1, int64(parts+4096))
	if ok {
		t.Fatal("a 1MB epilogue slipped past a cap a few KB above the parts")
	}
	if rec.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("got %d %s, want 413", rec.Code, rec.Body.String())
	}
}
