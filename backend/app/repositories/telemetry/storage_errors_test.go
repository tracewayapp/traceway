package telemetry

import (
	"context"
	"errors"
	"fmt"
	"io"
	"syscall"
	"testing"
)

func TestTransientStorageErrorsAreRetryable(t *testing.T) {
	for _, err := range []error{
		context.DeadlineExceeded, fmt.Errorf("insert: %w", context.DeadlineExceeded), fmt.Errorf("read: %w", io.ErrUnexpectedEOF),
		fmt.Errorf("dial: %w", syscall.ECONNREFUSED), fmt.Errorf("write: %w", syscall.EPIPE),
	} {
		if !IsTransientStorageError(err) {
			t.Errorf("%v should be retryable", err)
		}
	}
	for _, err := range []error{nil, errors.New("invalid stored OTel span ID"), errors.New("syntax error near SELECT")} {
		if IsTransientStorageError(err) {
			t.Errorf("%v must not be retried forever", err)
		}
	}
}
