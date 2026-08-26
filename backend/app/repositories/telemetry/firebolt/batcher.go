//go:build telemetry_firebolt

package firebolt

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/tracewayapp/traceway/backend/app/config"
	"github.com/tracewayapp/traceway/backend/app/db"
	traceway "go.tracewayapp.com"
)

// Cross-request ingest batching: SDK fleets send small per-request batches
// that fall below the file-load threshold and pay per-statement cost one
// INSERT at a time. With FIREBOLT_BATCH_MS set (and the copy dir configured),
// rows accumulate across requests per table and flush on a ticker as one
// file load. The HTTP ack then precedes durability by at most one flush
// window; a failed flush counts its rows as dropped so /api/health/deep and
// the bench's dropped-rows gate surface the loss instead of hiding it.
const batcherMaxBufferedRows = 200000

// A buffer past this size flushes immediately instead of waiting for the
// ticker — under sustained load bigger windows would only add latency and
// risk the overflow fallback. Flushes run concurrently per table (bounded)
// because the engine absorbs parallel file loads.
const batcherFlushRowsTrigger = 50000
const batcherMaxConcurrentFlushes = 4

type tableBuffer struct {
	columns []string
	rows    [][]any
}

type ingestBatcher struct {
	mu      sync.Mutex
	buffers map[string]*tableBuffer
	total   int
	kick    chan struct{}
	sem     chan struct{}
}

var (
	batcherOnce sync.Once
	batcher     *ingestBatcher
)

func activeBatcher() *ingestBatcher {
	batcherOnce.Do(func() {
		if config.Config == nil {
			return
		}
		ms, err := strconv.Atoi(strings.TrimSpace(config.Config.FireboltBatchMS))
		if err != nil || ms <= 0 {
			return
		}
		if _, _, ok := copyDirs(); !ok {
			return
		}
		b := &ingestBatcher{
			buffers: map[string]*tableBuffer{},
			kick:    make(chan struct{}, 1),
			sem:     make(chan struct{}, batcherMaxConcurrentFlushes),
		}
		go b.run(time.Duration(ms) * time.Millisecond)
		batcher = b
	})
	return batcher
}

// enqueue buffers rows for the next flush. When the buffer is full the rows
// are refused and the caller inserts directly — backpressure without loss.
func (b *ingestBatcher) enqueue(table string, columns []string, rows [][]any) bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.total+len(rows) > batcherMaxBufferedRows {
		return false
	}
	buf := b.buffers[table]
	if buf == nil {
		buf = &tableBuffer{columns: columns}
		b.buffers[table] = buf
	}
	buf.rows = append(buf.rows, rows...)
	b.total += len(rows)
	if b.total >= batcherFlushRowsTrigger {
		select {
		case b.kick <- struct{}{}:
		default:
		}
	}
	return true
}

func (b *ingestBatcher) run(interval time.Duration) {
	defer traceway.Recover()
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
		case <-b.kick:
		}
		b.flush()
	}
}

func (b *ingestBatcher) flush() {
	b.mu.Lock()
	pending := b.buffers
	b.buffers = map[string]*tableBuffer{}
	b.total = 0
	b.mu.Unlock()

	var wg sync.WaitGroup
	for table, buf := range pending {
		if len(buf.rows) == 0 {
			continue
		}
		wg.Add(1)
		b.sem <- struct{}{}
		go func(table string, buf *tableBuffer) {
			defer wg.Done()
			defer func() { <-b.sem }()
			defer traceway.Recover()
			ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
			defer cancel()
			err := copyIngest(ctx, table, buf.columns, buf.rows)
			if err != nil {
				err = insertRowsDirect(ctx, table, buf.columns, buf.rows)
			}
			if err != nil {
				for range buf.rows {
					db.RecordTelemetryRowDropped(table)
				}
				traceway.CaptureException(fmt.Errorf("firebolt batcher: dropped %d %s rows: %w", len(buf.rows), table, err))
			}
		}(table, buf)
	}
	wg.Wait()
}
