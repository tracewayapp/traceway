//go:build pgch

package chdb

import (
	"context"
	"errors"
	"io"
	"testing"

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

type attempt struct {
	prepErr error
	sendErr error
}

type fakeConn struct {
	attempts []attempt
	calls    int
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
	var a attempt
	if i < len(c.attempts) {
		a = c.attempts[i]
	}
	if a.prepErr != nil {
		return nil, a.prepErr
	}
	return &fakeBatch{sendErr: a.sendErr}, nil
}

func TestSendBatchRecoversFromStaleConnEOF(t *testing.T) {
	orig := Conn
	defer func() { Conn = orig }()

	eof := errors.New("query processing: failed to read first block packet from 1.2.3.4:9440 (conn_id=1): read: EOF")
	fc := &fakeConn{attempts: []attempt{{sendErr: eof}, {sendErr: eof}, {}}}
	Conn = fc

	fills := 0
	err := SendBatch("INSERT INTO x", func(b driver.Batch) error {
		fills++
		return b.Append(1)
	})
	if err != nil {
		t.Fatalf("expected success after retries, got %v", err)
	}
	if fc.calls != 3 {
		t.Fatalf("expected 3 prepare attempts, got %d", fc.calls)
	}
	if fills != 3 {
		t.Fatalf("expected fill invoked per attempt (3), got %d", fills)
	}
}

func TestSendBatchRetriesPrepareFailure(t *testing.T) {
	orig := Conn
	defer func() { Conn = orig }()

	fc := &fakeConn{attempts: []attempt{{prepErr: io.EOF}, {}}}
	Conn = fc

	err := SendBatch("INSERT INTO x", func(b driver.Batch) error { return nil })
	if err != nil {
		t.Fatalf("expected success after prepare retry, got %v", err)
	}
	if fc.calls != 2 {
		t.Fatalf("expected 2 prepare attempts, got %d", fc.calls)
	}
}

func TestSendBatchDoesNotRetryRealError(t *testing.T) {
	orig := Conn
	defer func() { Conn = orig }()

	boom := errors.New("code: 62, DB::Exception: Syntax error")
	fc := &fakeConn{attempts: []attempt{{sendErr: boom}, {}}}
	Conn = fc

	err := SendBatch("INSERT INTO x", func(b driver.Batch) error { return nil })
	if !errors.Is(err, boom) {
		t.Fatalf("expected the real error back, got %v", err)
	}
	if fc.calls != 1 {
		t.Fatalf("expected no retry, got %d attempts", fc.calls)
	}
}

func TestSendBatchExhaustsAttempts(t *testing.T) {
	orig := Conn
	defer func() { Conn = orig }()

	eof := errors.New("read: EOF")
	fc := &fakeConn{attempts: []attempt{{sendErr: eof}, {sendErr: eof}, {sendErr: eof}, {}}}
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

	fc := &fakeConn{attempts: []attempt{{}}}
	Conn = fc

	fillErr := errors.New("bad row")
	err := SendBatch("INSERT INTO x", func(b driver.Batch) error { return fillErr })
	if !errors.Is(err, fillErr) {
		t.Fatalf("expected fill error back, got %v", err)
	}
	if fc.calls != 1 {
		t.Fatalf("fill error must not retry, got %d attempts", fc.calls)
	}
}

func TestIsRetryableConnError(t *testing.T) {
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
		{errors.New("use of closed network connection"), true},
		{errors.New("code: 241, memory limit exceeded"), false},
		{errors.New("code: 62, syntax error"), false},
	}
	for _, tc := range cases {
		if got := isRetryableConnError(tc.err); got != tc.want {
			t.Errorf("isRetryableConnError(%v) = %v, want %v", tc.err, got, tc.want)
		}
	}
}
