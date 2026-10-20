package export

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"testing"
	"time"

	"example.com/reports-api/internal/telemetry"
)

type hiddenRows struct {
	err error
}

func (h hiddenRows) Rows(context.Context, int64) ([]string, []Row, error) {
	if h.err != nil {
		return nil, nil, h.err
	}
	return []string{"region", "revenue"}, []Row{{Values: []string{"EMEA", "1200"}}, {Values: []string{"APAC", "900"}}}, nil
}

type hiddenBlobs struct {
	putErr, signErr error
	puts            int
	stored          map[string][]byte
}

func (h *hiddenBlobs) Put(_ context.Context, key, _ string, data []byte) error {
	h.puts++
	if h.putErr != nil {
		return h.putErr
	}
	if h.stored == nil {
		h.stored = map[string][]byte{}
	}
	h.stored[key] = data
	return nil
}

func (h *hiddenBlobs) SignedURL(key string, _ time.Duration) (string, error) {
	if h.signErr != nil {
		return "", h.signErr
	}
	return "https://storage.example/reports-exports/" + key + "?sig=x", nil
}

// sink collects everything an export emits: slog records (message and
// attributes) and what reaches the telemetry hooks.
type sink struct {
	mu  sync.Mutex
	buf strings.Builder
}

func (s *sink) write(v ...any) {
	s.mu.Lock()
	defer s.mu.Unlock()
	fmt.Fprintln(&s.buf, v...)
}

type sinkHandler struct {
	s     *sink
	attrs []slog.Attr
}

func (h sinkHandler) Enabled(context.Context, slog.Level) bool { return true }
func (h sinkHandler) Handle(_ context.Context, r slog.Record) error {
	parts := []any{r.Level.String(), r.Message}
	for _, a := range h.attrs {
		parts = append(parts, a.String())
	}
	r.Attrs(func(a slog.Attr) bool { parts = append(parts, a.String()); return true })
	h.s.write(parts...)
	return nil
}
func (h sinkHandler) WithAttrs(as []slog.Attr) slog.Handler {
	return sinkHandler{s: h.s, attrs: append(append([]slog.Attr{}, h.attrs...), as...)}
}
func (h sinkHandler) WithGroup(string) slog.Handler { return h }

func observe(t *testing.T, f func() error) (string, error) {
	t.Helper()
	s := &sink{}
	prevLog := slog.Default()
	prevRecord, prevAttrs := telemetry.RecordError, telemetry.SetAttributes
	slog.SetDefault(slog.New(sinkHandler{s: s}))
	telemetry.RecordError = func(_ context.Context, err error) { s.write("recorded", err) }
	telemetry.SetAttributes = func(_ context.Context, kv ...any) { s.write(append([]any{"attributes"}, kv...)...) }
	defer func() {
		slog.SetDefault(prevLog)
		telemetry.RecordError, telemetry.SetAttributes = prevRecord, prevAttrs
	}()
	err := f()
	if err != nil {
		s.write("returned", err)
	}
	return s.buf.String(), err
}

func TestHiddenFailuresAreStillExportFailures(t *testing.T) {
	cases := map[string]struct {
		rows   Rows
		blobs  *hiddenBlobs
		format string
	}{
		"rows":   {hiddenRows{err: errors.New("canceling statement due to statement timeout")}, &hiddenBlobs{}, "csv"},
		"render": {hiddenRows{}, &hiddenBlobs{}, "pdf"},
		"put":    {hiddenRows{}, &hiddenBlobs{putErr: errors.New("storage credentials: credential endpoint: 503 Service Unavailable")}, "xlsx"},
		"sign":   {hiddenRows{}, &hiddenBlobs{signErr: errors.New("no storage credentials loaded")}, "json"},
	}
	for name, c := range cases {
		_, err := New(c.rows, c.blobs).Export(context.Background(), 7, c.format)
		if err == nil || !errors.Is(err, ErrExportFailed) {
			t.Errorf("%s failure: Export must still fail with ErrExportFailed, got %v", name, err)
		}
		if c.blobs.puts > 1 {
			t.Errorf("%s failure: the upload was attempted %d times; the export behaviour must not change", name, c.blobs.puts)
		}
	}
}

func TestHiddenSuccessUnchanged(t *testing.T) {
	blobs := &hiddenBlobs{}
	e := New(hiddenRows{}, blobs)
	e.now = func() time.Time { return time.Date(2026, 5, 4, 8, 30, 0, 0, time.UTC) }
	url, err := e.Export(context.Background(), 9, "json")
	if err != nil {
		t.Fatal(err)
	}
	if url != "https://storage.example/reports-exports/exports/9/20260504T083000Z.json?sig=x" {
		t.Fatalf("unexpected url %s", url)
	}
	if got := string(blobs.stored["exports/9/20260504T083000Z.json"]); got != `[{"region":"EMEA","revenue":"1200"},{"region":"APAC","revenue":"900"}]` {
		t.Fatalf("the rendered file changed: %s", got)
	}
}

// The next failure must say which step failed and why: through the returned
// error (the handler records it on the span), a slog record, or the
// telemetry hooks.
func TestHiddenNextFailureExplainsItself(t *testing.T) {
	for name, c := range map[string]struct {
		rows   Rows
		blobs  *hiddenBlobs
		format string
		want   string
	}{
		"rows":   {hiddenRows{err: errors.New("canceling statement due to statement timeout")}, &hiddenBlobs{}, "csv", "statement timeout"},
		"render": {hiddenRows{}, &hiddenBlobs{}, "pdf", `unknown format "pdf"`},
		"put":    {hiddenRows{}, &hiddenBlobs{putErr: errors.New("storage credentials: credential endpoint: 503 Service Unavailable")}, "xlsx", "503 Service Unavailable"},
		"sign":   {hiddenRows{}, &hiddenBlobs{signErr: errors.New("no storage credentials loaded")}, "json", "no storage credentials loaded"},
	} {
		out, _ := observe(t, func() error {
			_, err := New(c.rows, c.blobs).Export(context.Background(), 7, c.format)
			return err
		})
		if !strings.Contains(out, c.want) {
			t.Errorf("%s failure: nothing the export emits names the cause %q; got:\n%s", name, c.want, out)
		}
	}
}
