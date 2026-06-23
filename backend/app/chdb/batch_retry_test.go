//go:build pgch

package chdb

import (
	"context"
	"errors"
	"io"
	"testing"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/ClickHouse/clickhouse-go/v2/lib/column"
	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
)

type fakeBatch struct {
	sendErr error
	sent    bool
	aborted bool
}

func (b *fakeBatch) Abort() error                  { b.aborted = true; return nil }
func (b *fakeBatch) Append(v ...any) error         { return nil }
func (b *fakeBatch) AppendStruct(v any) error      { return nil }
func (b *fakeBatch) Column(int) driver.BatchColumn { return nil }
func (b *fakeBatch) Flush() error                  { return nil }
func (b *fakeBatch) Send() error                   { b.sent = true; return b.sendErr }
func (b *fakeBatch) IsSent() bool                  { return b.sent }
func (b *fakeBatch) Rows() int                     { return 0 }
func (b *fakeBatch) Columns() []column.Interface   { return nil }
func (b *fakeBatch) Close() error                  { return nil }

type fakeConn struct {
	prepareErrs []error
	sendErr     error
	calls       int
	lastBatch   *fakeBatch
}

func (c *fakeConn) Query(ctx context.Context, query string, args ...any) (driver.Rows, error) {
	return nil, nil
}
func (c *fakeConn) QueryRow(ctx context.Context, query string, args ...any) driver.Row { return nil }
func (c *fakeConn) Exec(ctx context.Context, query string, args ...any) error          { return nil }
func (c *fakeConn) Stats() driver.Stats                                                { return driver.Stats{} }
func (c *fakeConn) PrepareBatch(ctx context.Context, query string, opts ...driver.PrepareBatchOption) (driver.Batch, error) {
	i := c.calls
	c.calls++
	var e error
	if i < len(c.prepareErrs) {
		e = c.prepareErrs[i]
	}
	if e != nil {
		return nil, e
	}
	b := &fakeBatch{sendErr: c.sendErr}
	c.lastBatch = b
	return b, nil
}

func TestSendBatchRecoversFromStalePrepareEOF(t *testing.T) {
	orig := Conn
	defer func() { Conn = orig }()

	eof := errors.New("query processing: failed to read first block packet from 1.2.3.4:9440 (conn_id=1): read: EOF")
	fc := &fakeConn{prepareErrs: []error{eof, eof, nil}}
	Conn = fc

	fills := 0
	err := SendBatch("INSERT INTO x", func(b driver.Batch) error {
		fills++
		return b.Append(1)
	})
	if err != nil {
		t.Fatalf("expected success after prepare retries, got %v", err)
	}
	if fc.calls != 3 {
		t.Fatalf("expected 3 prepare attempts, got %d", fc.calls)
	}
	if fills != 1 {
		t.Fatalf("expected fill invoked once (after the successful prepare), got %d", fills)
	}
}

func TestSendBatchDoesNotRetrySendFailure(t *testing.T) {
	orig := Conn
	defer func() { Conn = orig }()

	sendEOF := errors.New("query processing: failed to read packet from 1.2.3.4:9440 (conn_id=1): read: EOF")
	fc := &fakeConn{prepareErrs: []error{nil}, sendErr: sendEOF}
	Conn = fc

	err := SendBatch("INSERT INTO x", func(b driver.Batch) error { return nil })
	if !errors.Is(err, sendEOF) {
		t.Fatalf("expected the send error returned without retry, got %v", err)
	}
	if fc.calls != 1 {
		t.Fatalf("send failures must not be retried (would risk duplicate inserts); got %d prepare calls", fc.calls)
	}
}

func TestSendBatchDoesNotRetryServerException(t *testing.T) {
	orig := Conn
	defer func() { Conn = orig }()

	boom := &clickhouse.Exception{Code: 27, Message: "Cannot parse input: unexpected EOF"}
	fc := &fakeConn{prepareErrs: []error{boom, nil}}
	Conn = fc

	err := SendBatch("INSERT INTO x", func(b driver.Batch) error { return nil })
	if !errors.Is(err, boom) {
		t.Fatalf("a server exception must not be retried even if its message contains EOF, got %v", err)
	}
	if fc.calls != 1 {
		t.Fatalf("expected no retry on server exception, got %d attempts", fc.calls)
	}
}

func TestSendBatchExhaustsPrepareAttempts(t *testing.T) {
	orig := Conn
	defer func() { Conn = orig }()

	eof := errors.New("read: EOF")
	fc := &fakeConn{prepareErrs: []error{eof, eof, eof, nil}}
	Conn = fc

	err := SendBatch("INSERT INTO x", func(b driver.Batch) error { return nil })
	if err == nil {
		t.Fatal("expected error after exhausting attempts")
	}
	if fc.calls != batchSendMaxAttempts {
		t.Fatalf("expected %d attempts, got %d", batchSendMaxAttempts, fc.calls)
	}
}

func TestSendBatchAbortsOnFillError(t *testing.T) {
	orig := Conn
	defer func() { Conn = orig }()

	fc := &fakeConn{prepareErrs: []error{nil}}
	Conn = fc

	fillErr := errors.New("bad row")
	err := SendBatch("INSERT INTO x", func(b driver.Batch) error { return fillErr })
	if !errors.Is(err, fillErr) {
		t.Fatalf("expected fill error back, got %v", err)
	}
	if fc.calls != 1 {
		t.Fatalf("fill error must not retry, got %d attempts", fc.calls)
	}
	if fc.lastBatch == nil || !fc.lastBatch.aborted {
		t.Fatal("expected the batch to be aborted on fill error")
	}
}

func TestRetryOnConn(t *testing.T) {
	boom := errors.New("boom")

	calls := 0
	err := RetryOnConn(3, func(error) bool { return true }, func() error { calls++; return boom })
	if !errors.Is(err, boom) || calls != 3 {
		t.Fatalf("expected last error after 3 attempts, got err=%v calls=%d", err, calls)
	}

	calls = 0
	err = RetryOnConn(3, func(error) bool { return false }, func() error { calls++; return boom })
	if !errors.Is(err, boom) || calls != 1 {
		t.Fatalf("non-retryable error must not retry, got err=%v calls=%d", err, calls)
	}

	calls = 0
	err = RetryOnConn(3, func(error) bool { return true }, func() error {
		calls++
		if calls < 2 {
			return boom
		}
		return nil
	})
	if err != nil || calls != 2 {
		t.Fatalf("expected success on second attempt, got err=%v calls=%d", err, calls)
	}
}

func TestIsConnError(t *testing.T) {
	cases := []struct {
		err  error
		want bool
	}{
		{nil, false},
		{io.EOF, true},
		{io.ErrUnexpectedEOF, true},
		{errors.New("failed to read first block packet from x: read: EOF"), true},
		{errors.New("write: broken pipe"), true},
		{errors.New("read: connection reset by peer"), true},
		{errors.New("dial tcp 1.2.3.4:9440: connect: connection refused"), true},
		{errors.New("dial tcp: no route to host"), true},
		{errors.New("use of closed network connection"), true},
		{&clickhouse.Exception{Code: 62, Message: "syntax error near EOF"}, false},
		{errors.New("code: 241, memory limit exceeded"), false},
	}
	for _, tc := range cases {
		if got := IsConnError(tc.err); got != tc.want {
			t.Errorf("IsConnError(%v) = %v, want %v", tc.err, got, tc.want)
		}
	}
}
