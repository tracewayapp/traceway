package middleware

import (
	"io"
	"strings"
	"testing"
	"time"
)

type pausingReader struct {
	r     io.Reader
	pause time.Duration
}

func (p *pausingReader) Read(b []byte) (int, error) {
	time.Sleep(p.pause)
	return p.r.Read(b)
}

// Discarding the rest of an upload bypasses the decompressor. Had it been
// counted as time spent reading the wire for the decompressor, it would be
// subtracted from the gunzip time and report it as zero.
func TestDiscardingTheRestIsNotCountedAgainstGunzipTime(t *testing.T) {
	rest := strings.Repeat("x", 4096)
	meter := &ingestBodyMeter{
		wire:        io.NopCloser(&pausingReader{r: strings.NewReader(rest), pause: 5 * time.Millisecond}),
		limit:       1 << 20,
		gzip:        true,
		wireBytes:   1000,
		wireTime:    10 * time.Millisecond,
		decodedTime: 50 * time.Millisecond,
	}

	if !meter.discardRest() {
		t.Fatal("a rest under the cap was not discarded")
	}
	if meter.wireTime != 10*time.Millisecond || meter.wireBytes != 1000 {
		t.Fatalf("discarding changed the decompressor's wire counters: %d bytes, %s", meter.wireBytes, meter.wireTime)
	}
	if meter.discardedBytes != int64(len(rest)) || meter.discardTime < 5*time.Millisecond {
		t.Fatalf("discard not measured: %d bytes, %s", meter.discardedBytes, meter.discardTime)
	}
	if gunzip := meter.decodedTime - meter.wireTime; gunzip != 40*time.Millisecond {
		t.Fatalf("gunzip time is %s, want the 40ms spent decompressing", gunzip)
	}
}

func TestDiscardingGivesUpPastTheCap(t *testing.T) {
	meter := &ingestBodyMeter{wire: io.NopCloser(strings.NewReader(strings.Repeat("x", 2048))), limit: 1024}
	if meter.discardRest() {
		t.Fatal("a rest over the cap was reported as discarded")
	}
}
