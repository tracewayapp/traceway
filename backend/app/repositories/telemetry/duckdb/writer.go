//go:build telemetry_duckdb

package duckdb

import (
	"context"
	"database/sql/driver"
	"fmt"
	"runtime"
	"sync"
	"sync/atomic"
	"time"

	"github.com/duckdb/duckdb-go/v2"
	"github.com/tracewayapp/traceway/backend/app/db"
	traceway "go.tracewayapp.com"
)

// The per-request appender path (one connection + one WAL commit per ingest
// request) is what capped write throughput in the benchmarks: at the cliff a
// 16k-row request held an admission slot for 1-3s. The writers below decouple
// the request path from the flush: requests convert rows and enqueue, one
// goroutine per hot table owns a persistent connection + Appender and flushes
// on a size-or-age trigger, coalescing many request batches into one WAL
// commit. A 200 therefore means "accepted into the bounded write queue", not
// "committed" — the same semantics the OTel Collector and its sending queue
// give. The queue is bounded and enqueue rejects with ErrIngestQueueFull
// (mapped to 503 + Retry-After) so overload is shed before the ack, never by
// dropping acked rows.
const (
	defaultWriteQueueRows     = 131072
	defaultWriteFlushRows     = 32768
	defaultWriteFlushInterval = 100 * time.Millisecond

	writerReconnectMinBackoff = 100 * time.Millisecond
	writerReconnectMaxBackoff = 5 * time.Second
)

var batchedTables = []string{"spans", "metric_points", "log_records", "endpoints", "tasks"}

type writeReq struct {
	rows [][]driver.Value
	// done non-nil marks a flush barrier: the writer flushes everything
	// enqueued before it (channel FIFO) and closes the channel. rows is nil.
	done chan struct{}
}

// tableWriter owns one table's queue, sharded across a small pool of writer
// goroutines (VictoriaMetrics-style: drain concurrency sized to CPU). Each
// goroutine owns one channel and one connection + Appender exclusively — the
// duckdb Appender is not thread-safe; all cross-goroutine interaction goes
// through the channels. Batches are round-robined across the shards, so
// per-channel FIFO still guarantees a barrier sent to every shard flushes
// everything enqueued before it.
type tableWriter struct {
	table         string
	chans         []chan writeReq
	next          atomic.Uint64
	maxQueueRows  int64
	flushRows     int
	flushInterval time.Duration

	// pendingRows counts rows admitted but not yet flushed (queued +
	// appended-unflushed). It is decremented after the flush, not after the
	// append, so it is simultaneously the backpressure bound, the
	// acked-but-unpersisted loss window, and the queue-depth gauge: a stalled
	// WAL commit keeps it high and enqueue keeps rejecting. Do not "fix" it
	// to decrement at append time.
	pendingRows  atomic.Int64
	enqueuedRows atomic.Uint64
	flushedRows  atomic.Uint64
	flushErrors  atomic.Uint64
}

type writerSet struct {
	byTable map[string]*tableWriter
	cancel  context.CancelFunc
	wg      sync.WaitGroup
}

var activeWriters atomic.Pointer[writerSet]

type WriterOptions struct {
	QueueRows     int
	FlushRows     int
	FlushInterval time.Duration
	Writers       int
}

// defaultWritersPerTable mirrors the admission gate's old concurrency
// (2×CPU), which is also VictoriaMetrics' default insert concurrency — the
// single-writer v1 measurably under-drained bulk ingest vs the old
// per-request appenders.
func defaultWritersPerTable() int {
	n := runtime.NumCPU() * 2
	if n < 2 {
		n = 2
	}
	if n > 8 {
		n = 8
	}
	return n
}

// StartWriters starts one background writer per batched table. Idempotent.
func StartWriters(ctx context.Context, opts WriterOptions) {
	if activeWriters.Load() != nil {
		return
	}
	if opts.QueueRows <= 0 {
		opts.QueueRows = defaultWriteQueueRows
	}
	if opts.FlushRows <= 0 {
		opts.FlushRows = defaultWriteFlushRows
	}
	if opts.FlushInterval <= 0 {
		opts.FlushInterval = defaultWriteFlushInterval
	}
	if opts.Writers <= 0 {
		opts.Writers = defaultWritersPerTable()
	}

	// Derive a cancelable context so StopWriters works even though the
	// server passes context.Background().
	ctx, cancel := context.WithCancel(ctx)
	ws := &writerSet{byTable: make(map[string]*tableWriter, len(batchedTables)), cancel: cancel}
	for _, table := range batchedTables {
		w := &tableWriter{
			table:         table,
			chans:         make([]chan writeReq, opts.Writers),
			maxQueueRows:  int64(opts.QueueRows),
			flushRows:     opts.FlushRows,
			flushInterval: opts.FlushInterval,
		}
		for i := range w.chans {
			// Every data batch carries >=1 row, so admitted batches <=
			// admitted rows <= maxQueueRows regardless of how round-robin
			// distributes them; the slack absorbs concurrent zero-row
			// barriers. An admitted send therefore never blocks.
			w.chans[i] = make(chan writeReq, opts.QueueRows+16)
			ws.wg.Add(1)
			go w.runLoop(ctx, w.chans[i], &ws.wg)
		}
		ws.byTable[table] = w
	}
	if !activeWriters.CompareAndSwap(nil, ws) {
		cancel()
		ws.wg.Wait()
		return
	}
	db.SetTelemetryWriteQueueDepthFn(func() map[string]int {
		depths := make(map[string]int, len(batchedTables))
		if cur := activeWriters.Load(); cur != nil {
			for table, w := range cur.byTable {
				depths[table] = int(w.pendingRows.Load())
			}
		}
		return depths
	})
}

// StopWriters flushes and stops all writers. Needed by tests, which swap the
// global DuckDB connector between cases.
func StopWriters() {
	ws := activeWriters.Swap(nil)
	if ws == nil {
		return
	}
	ws.cancel()
	ws.wg.Wait()
	db.SetTelemetryWriteQueueDepthFn(nil)
}

// FlushWriters blocks until every row enqueued before the call is flushed to
// DuckDB. Used by the notification evaluator (which reads back rows it just
// ingested) and by tests. No-op when writers are not running.
func FlushWriters(ctx context.Context) error {
	ws := activeWriters.Load()
	if ws == nil {
		return nil
	}
	var barriers []chan struct{}
	for _, w := range ws.byTable {
		for _, ch := range w.chans {
			done := make(chan struct{})
			select {
			case ch <- writeReq{done: done}:
				barriers = append(barriers, done)
			case <-ctx.Done():
				return ctx.Err()
			}
		}
	}
	for _, done := range barriers {
		select {
		case <-done:
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	return nil
}

// insertRows is the shared entry for the batched tables: enqueue when the
// writers are running, otherwise fall back to the synchronous per-request
// appender path (tests, and any deployment that never starts the writers).
func insertRows(ctx context.Context, table string, rows [][]driver.Value) error {
	if len(rows) == 0 {
		return nil
	}
	if ws := activeWriters.Load(); ws != nil {
		if w, ok := ws.byTable[table]; ok {
			return w.enqueue(rows)
		}
	}
	return appendRowsSync(ctx, table, rows)
}

func appendRowsSync(ctx context.Context, table string, rows [][]driver.Value) error {
	return withAppender(ctx, table, func(appender *duckdb.Appender) {
		for _, row := range rows {
			if err := appender.AppendRow(row...); err != nil {
				captureDroppedRow(table, err)
			}
		}
	})
}

func (w *tableWriter) enqueue(rows [][]driver.Value) error {
	n := int64(len(rows))
	if w.pendingRows.Add(n) > w.maxQueueRows {
		w.pendingRows.Add(-n)
		return db.ErrIngestQueueFull
	}
	w.enqueuedRows.Add(uint64(n))
	w.chans[w.next.Add(1)%uint64(len(w.chans))] <- writeReq{rows: rows}
	return nil
}

// runLoop restarts runOnce after a panic instead of dying: a dead writer
// would strand its shard of the queue into permanent 503s for the table.
func (w *tableWriter) runLoop(ctx context.Context, ch chan writeReq, wg *sync.WaitGroup) {
	defer wg.Done()
	for {
		if done := w.runOnce(ctx, ch); done {
			return
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(writerReconnectMinBackoff):
		}
	}
}

func (w *tableWriter) runOnce(ctx context.Context, ch chan writeReq) (done bool) {
	var (
		conn       driver.Conn
		appender   *duckdb.Appender
		sinceFlush int
	)
	defer func() {
		if r := recover(); r != nil {
			traceway.CaptureException(fmt.Errorf("duckdb %s writer panic: %v", w.table, r))
			if sinceFlush > 0 {
				w.dropUnflushed(sinceFlush, fmt.Errorf("writer panic: %v", r))
			}
			if appender != nil {
				// The unflushed rows were just counted as dropped; Clear so
				// the deferred Close below can't half-commit them.
				appender.Clear()
			}
			done = false
		}
		if appender != nil {
			appender.Close()
		}
		if conn != nil {
			conn.Close()
		}
	}()

	ensure := func() bool {
		backoff := writerReconnectMinBackoff
		for appender == nil {
			var err error
			conn, err = db.DuckDBConnector.Connect(ctx)
			if err == nil {
				appender, err = duckdb.NewAppenderFromConn(conn, "", w.table)
				if err == nil {
					return true
				}
				conn.Close()
				conn = nil
			}
			db.RecordTelemetryInsertFailure()
			captureFlushFailure(w.table, 0, err)
			select {
			case <-ctx.Done():
				return false
			case <-time.After(backoff):
			}
			backoff *= 2
			if backoff > writerReconnectMaxBackoff {
				backoff = writerReconnectMaxBackoff
			}
		}
		return true
	}

	flush := func() {
		if appender == nil || sinceFlush == 0 {
			return
		}
		if err := appender.Flush(); err != nil {
			// The rows in this appender chunk were already acked. Clear and
			// drop them (counted), close the connection, and let ensure()
			// reconnect with backoff; rows still in the channel survive.
			w.flushErrors.Add(1)
			db.RecordTelemetryInsertFailure()
			w.dropUnflushed(sinceFlush, err)
			appender.Clear()
			appender.Close()
			conn.Close()
			appender = nil
			conn = nil
		} else {
			w.flushedRows.Add(uint64(sinceFlush))
			w.pendingRows.Add(-int64(sinceFlush))
		}
		sinceFlush = 0
	}

	timer := time.NewTimer(w.flushInterval)
	if !timer.Stop() {
		<-timer.C
	}
	timerActive := false
	defer timer.Stop()

	for {
		select {
		case <-ctx.Done():
			// Best-effort final flush so StopWriters (tests) doesn't lose
			// rows. In production nothing cancels this context.
			flush()
			return true
		case req := <-ch:
			if req.done != nil {
				flush()
				if timerActive {
					if !timer.Stop() {
						<-timer.C
					}
					timerActive = false
				}
				close(req.done)
				continue
			}
			if !ensure() {
				// Shutdown while reconnecting: the admitted rows are dropped
				// with the process; nothing to flush.
				w.dropUnflushed(len(req.rows), ctx.Err())
				return true
			}
			for _, row := range req.rows {
				if err := appender.AppendRow(row...); err != nil {
					captureDroppedRow(w.table, err)
				}
			}
			// Dropped rows still count toward sinceFlush/pendingRows until
			// the flush settles the batch; they were admitted at enqueue.
			sinceFlush += len(req.rows)
			if sinceFlush >= w.flushRows {
				flush()
				if timerActive {
					if !timer.Stop() {
						<-timer.C
					}
					timerActive = false
				}
			} else if !timerActive {
				timer.Reset(w.flushInterval)
				timerActive = true
			}
		case <-timer.C:
			timerActive = false
			flush()
		}
	}
}

// dropUnflushed settles n admitted-but-lost rows: releases their queue budget
// and records them as dropped (they may have been acked already).
func (w *tableWriter) dropUnflushed(n int, err error) {
	w.pendingRows.Add(-int64(n))
	db.RecordTelemetryRowsDropped(w.table, uint64(n))
	captureFlushFailure(w.table, n, err)
}
