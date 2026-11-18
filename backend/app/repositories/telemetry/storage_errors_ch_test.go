//go:build telemetry_ch

package telemetry

import (
	"fmt"
	"testing"

	"github.com/ClickHouse/clickhouse-go/v2"
)

func TestClickHouseOverloadIsRetryableButPoisonIsNot(t *testing.T) {
	for _, exception := range []*clickhouse.Exception{
		{Code: 241, Message: "Memory limit (total) exceeded"}, {Code: 202, Message: "Too many simultaneous queries"},
		{Code: 252, Message: "Too many parts (300). Merges are processing significantly slower than inserts"},
	} {
		if !IsTransientStorageError(fmt.Errorf("insert: %w", exception)) {
			t.Errorf("code %d should be retryable", exception.Code)
		}
	}
	for _, exception := range []*clickhouse.Exception{
		{Code: 252, Message: "Too many partitions for single INSERT block (more than 100)"}, {Code: 62, Message: "Syntax error"},
		{Code: 53, Message: "Type mismatch in IN or VALUES section"},
	} {
		if IsTransientStorageError(fmt.Errorf("insert: %w", exception)) {
			t.Errorf("code %d (%s) is permanent and must not be retried", exception.Code, exception.Message)
		}
	}
}
