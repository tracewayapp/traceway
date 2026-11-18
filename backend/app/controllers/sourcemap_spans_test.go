package controllers

import (
	"bytes"
	"context"
	"errors"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/tracewayapp/traceway/backend/v2/app/middleware"
	"github.com/tracewayapp/traceway/backend/v2/app/storage"
	"github.com/tracewayapp/traceway/backend/v2/app/traceway"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
)

type uploadSpanStore struct {
	mu       sync.Mutex
	writes   []string
	failed   bool
	mapsRead chan struct{}
}

func (s *uploadSpanStore) Write(_ context.Context, stringKey string, data []byte) error {
	time.Sleep(15 * time.Millisecond)
	s.mu.Lock()
	defer s.mu.Unlock()
	s.writes = append(s.writes, stringKey)
	if s.failed {
		return errors.New("synthetic storage failure")
	}
	return nil
}
func (s *uploadSpanStore) Read(_ context.Context, key string) ([]byte, error) {
	if strings.HasSuffix(key, ".map") {
		s.mapsRead <- struct{}{}
	}
	return nil, storage.ErrNotFound
}
func (s *uploadSpanStore) Delete(context.Context, string) error { return nil }

func TestSourceMapUploadPhaseSpans(t *testing.T) {
	for _, failed := range []bool{false, true} {
		name := "success"
		if failed {
			name = "storage failure"
		}
		t.Run(name, func(t *testing.T) {
			recorder := tracetest.NewSpanRecorder()
			provider := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(recorder))
			defer provider.Shutdown(context.Background())
			restore := traceway.Install(traceway.Providers{TracerProvider: provider})
			defer restore()
			fake := &uploadSpanStore{failed: failed, mapsRead: make(chan struct{}, 2)}
			previous := storage.Store
			storage.Store = fake
			defer func() { storage.Store = previous }()
			var body bytes.Buffer
			writer := multipart.NewWriter(&body)
			file, err := writer.CreateFormFile("files", "upload.js.map")
			if err != nil {
				t.Fatal(err)
			}
			_, err = io.WriteString(file, `{"version":3,"debugId":"12345678-1234-1234-1234-123456789012"}`)
			if err != nil {
				t.Fatal(err)
			}
			if err = writer.Close(); err != nil {
				t.Fatal(err)
			}
			ctx, parent := provider.Tracer("upload-test").Start(context.Background(), "upload request")
			rec := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(rec)
			c.Request = httptest.NewRequest(http.MethodPost, "/api/sourcemaps/upload", &body).WithContext(ctx)
			c.Request.Header.Set("Content-Type", writer.FormDataContentType())
			c.Set(middleware.ProjectIdContextKey, uuid.New())
			SourceMapController.Upload(c)
			parent.End()
			if failed {
				if rec.Code != 500 || len(c.Errors) != 1 {
					t.Fatalf("failure response: %d %v", rec.Code, c.Errors)
				}
			} else {
				if rec.Code != 200 || rec.Body.String() != `{"uploaded":1}` {
					t.Fatalf("success response: %d %s", rec.Code, rec.Body.String())
				}
				// Both warm-up jobs stop at the missing map; neither can touch Store afterward.
				for range 2 {
					select {
					case <-fake.mapsRead:
					case <-time.After(5 * time.Second):
						t.Fatal("warm-up did not finish storage reads")
					}
				}
			}
			spans := map[string]sdktrace.ReadOnlySpan{}
			for _, span := range recorder.Ended() {
				spans[span.Name()] = span
				if span.SpanContext().TraceID() != parent.SpanContext().TraceID() {
					t.Fatal("upload phase detached from request")
				}
			}
			required := []string{"reading source map upload", "storing source map upload", "sourcemap.read_file", "sourcemap.write_file"}
			if !failed {
				required = append(required, "sourcemap.extract_debug_id", "sourcemap.write_debug_id_alias", "sourcemap.invalidate_cache")
			}
			for _, name := range required {
				if spans[name] == nil {
					t.Fatalf("missing span %s", name)
				}
			}
			for _, name := range []string{"sourcemap.write_file", "sourcemap.write_debug_id_alias"} {
				span := spans[name]
				if span == nil {
					continue
				}
				if span.EndTime().Sub(span.StartTime()) < 15*time.Millisecond {
					t.Fatalf("%s missed slow storage call", name)
				}
				if span.Parent().SpanID() != spans["storing source map upload"].SpanContext().SpanID() {
					t.Fatalf("wrong parent for %s", name)
				}
			}
			want := 2
			if failed {
				want = 1
			}
			if len(fake.writes) != want {
				t.Fatalf("writes=%d want %d", len(fake.writes), want)
			}
		})
	}
}
