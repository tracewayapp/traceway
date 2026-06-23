//go:build pgch

package chdb

import (
	"errors"
	"io"
	"net"
	"strings"
	"syscall"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
)

const batchSendMaxAttempts = 3

func SendBatch(query string, fill func(driver.Batch) error) error {
	var lastErr error
	for attempt := 1; attempt <= batchSendMaxAttempts; attempt++ {
		batch, err := Conn.PrepareBatch(BatchCtx(), query)
		if err != nil {
			lastErr = err
			if isRetryableConnError(err) && attempt < batchSendMaxAttempts {
				time.Sleep(time.Duration(attempt) * 50 * time.Millisecond)
				continue
			}
			return err
		}
		if err := fill(batch); err != nil {
			batch.Abort()
			return err
		}
		if err := batch.Send(); err != nil {
			lastErr = err
			if isRetryableConnError(err) && attempt < batchSendMaxAttempts {
				time.Sleep(time.Duration(attempt) * 50 * time.Millisecond)
				continue
			}
			return err
		}
		return nil
	}
	return lastErr
}

func isRetryableConnError(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) ||
		errors.Is(err, syscall.ECONNRESET) || errors.Is(err, syscall.EPIPE) ||
		errors.Is(err, net.ErrClosed) {
		return true
	}
	msg := err.Error()
	for _, s := range []string{
		"read: EOF",
		"unexpected EOF",
		"broken pipe",
		"connection reset",
		"first block packet",
		"use of closed network connection",
	} {
		if strings.Contains(msg, s) {
			return true
		}
	}
	return false
}
