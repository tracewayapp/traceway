package controllers

import (
	"bytes"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/tracewayapp/traceway/backend/app/middleware"
)

func multipartBody(t *testing.T, parts int, size int) (*bytes.Buffer, string) {
	t.Helper()
	var body bytes.Buffer
	w := multipart.NewWriter(&body)
	for i := 0; i < parts; i++ {
		fw, err := w.CreateFormFile("files", fmt.Sprintf("f%d.map", i))
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

func TestParseMultipartCappedRefusesADeclaredLengthOverTheCapBeforeReading(t *testing.T) {
	body, ct := multipartBody(t, 1, 100)
	raw := body.Bytes()
	ok, rec := runParseCapped(t, raw, ct, int64(len(raw)), int64(len(raw)-1))
	if ok || rec.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("got ok=%v %d %s, want 413 from the Content-Length precheck", ok, rec.Code, rec.Body.String())
	}
}

func TestParseMultipartCappedAlwaysReports413WhenTruncated(t *testing.T) {
	body, ct := multipartBody(t, 8, 100)
	raw := body.Bytes()

	misreported := 0
	for limit := 0; limit < len(raw); limit++ {
		ok, rec := runParseCapped(t, raw, ct, -1, int64(limit))
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

type failAfterReader struct {
	r   io.Reader
	err error
}

func (f *failAfterReader) Read(p []byte) (int, error) {
	n, err := f.r.Read(p)
	if err == io.EOF {
		return n, f.err
	}
	return n, err
}

func TestParseMultipartCappedReportsASlowBodyAs408(t *testing.T) {
	body, ct := multipartBody(t, 2, 100)
	raw := body.Bytes()

	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/sourcemaps/upload", io.NopCloser(&failAfterReader{r: bytes.NewReader(raw[:len(raw)/2]), err: middleware.ErrBodyTooSlow}))
	c.Request.Header.Set("Content-Type", ct)
	c.Request.ContentLength = -1

	if parseMultipartCapped(c, int64(len(raw))*2, 1<<20) {
		t.Fatal("a body cut by the throughput floor must not parse")
	}
	if rec.Code != http.StatusRequestTimeout {
		t.Fatalf("got %d %s, want 408", rec.Code, rec.Body.String())
	}
}

func TestParseMultipartCappedReportsTooManyPartsAs422(t *testing.T) {
	body, ct := multipartBody(t, 1001, 1)
	raw := body.Bytes()

	ok, rec := runParseCapped(t, raw, ct, int64(len(raw)), int64(len(raw))*2)
	if ok {
		t.Fatal("1001 parts must not parse")
	}
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("got %d %s, want 422 telling the client to batch", rec.Code, rec.Body.String())
	}
}
