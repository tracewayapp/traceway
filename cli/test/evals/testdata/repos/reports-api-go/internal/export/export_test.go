package export

import (
	"context"
	"strings"
	"testing"
	"time"
)

type fakeRows struct {
	cols []string
	rows []Row
	err  error
}

func (f fakeRows) Rows(context.Context, int64) ([]string, []Row, error) { return f.cols, f.rows, f.err }

type fakeBlobs struct {
	puts   map[string][]byte
	putErr error
}

func (f *fakeBlobs) Put(_ context.Context, key, _ string, data []byte) error {
	if f.putErr != nil {
		return f.putErr
	}
	if f.puts == nil {
		f.puts = map[string][]byte{}
	}
	f.puts[key] = data
	return nil
}

func (f *fakeBlobs) SignedURL(key string, ttl time.Duration) (string, error) {
	return "https://storage.example/reports-exports/" + key + "?expires=" + ttl.String(), nil
}

func TestExportCSV(t *testing.T) {
	blobs := &fakeBlobs{}
	e := New(fakeRows{cols: []string{"region", "revenue"}, rows: []Row{{Values: []string{"EMEA", "1200"}}}}, blobs)
	e.now = func() time.Time { return time.Date(2026, 3, 2, 10, 0, 0, 0, time.UTC) }
	url, err := e.Export(context.Background(), 42, "csv")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(url, "exports/42/20260302T100000Z.csv") {
		t.Fatalf("unexpected url %s", url)
	}
	if got := string(blobs.puts["exports/42/20260302T100000Z.csv"]); got != "region,revenue\nEMEA,1200\n" {
		t.Fatalf("unexpected file %q", got)
	}
}

func TestRenderXLSXIsAZip(t *testing.T) {
	b, _, err := render([]string{"a"}, []Row{{Values: []string{"1"}}}, "xlsx")
	if err != nil || len(b) < 4 || string(b[:2]) != "PK" {
		t.Fatalf("xlsx render: %v", err)
	}
}
