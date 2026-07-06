//go:build telemetry_ch

package chdb

import (
	"errors"
	"io"
	"net"
	"strings"
	"syscall"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
)

const batchSendMaxAttempts = 3

func SendBatch(query string, fill func(driver.Batch) error) error {
	var batch driver.Batch
	if err := RetryOnConn(batchSendMaxAttempts, IsConnError, func() error {
		b, err := Conn.PrepareBatch(BatchCtx(), query)
		batch = b
		return err
	}); err != nil {
		return err
	}
	if err := fill(batch); err != nil {
		batch.Abort()
		return err
	}
	return batch.Send()
}

func RetryOnConn(maxAttempts int, retryable func(error) bool, fn func() error) error {
	for attempt := 1; ; attempt++ {
		err := fn()
		if err == nil || attempt >= maxAttempts || !retryable(err) {
			return err
		}
		time.Sleep(time.Duration(attempt) * 50 * time.Millisecond)
	}
}

func IsConnError(err error) bool {
	if err == nil {
		return false
	}
	var chErr *clickhouse.Exception
	if errors.As(err, &chErr) {
		return false
	}
	if errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) ||
		errors.Is(err, syscall.ECONNRESET) || errors.Is(err, syscall.EPIPE) ||
		errors.Is(err, net.ErrClosed) {
		return true
	}
	msg := strings.ToLower(err.Error())
	for _, s := range []string{
		"eof",
		"broken pipe",
		"connection reset",
		"reset by peer",
		"connection refused",
		"no route to host",
		"use of closed",
		"first block packet",
	} {
		if strings.Contains(msg, s) {
			return true
		}
	}
	return false
}
