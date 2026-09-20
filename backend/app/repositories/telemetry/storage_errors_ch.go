//go:build telemetry_ch

package telemetry

import (
	"errors"
	"strings"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/tracewayapp/traceway/backend/app/chdb"
)

var transientClickHouseCodes = map[int32]bool{
	159: true, // TIMEOUT_EXCEEDED
	160: true, // TOO_SLOW
	164: true, // READONLY
	202: true, // TOO_MANY_SIMULTANEOUS_QUERIES
	203: true, // NO_FREE_CONNECTION
	209: true, // SOCKET_TIMEOUT
	210: true, // NETWORK_ERROR
	236: true, // ABORTED
	241: true, // MEMORY_LIMIT_EXCEEDED
	242: true, // TABLE_IS_READ_ONLY
	243: true, // NOT_ENOUGH_SPACE
	252: true, // TOO_MANY_PARTS
	319: true, // UNKNOWN_STATUS_OF_INSERT
	394: true, // QUERY_WAS_CANCELLED
	425: true, // SYSTEM_ERROR
	999: true, // KEEPER_EXCEPTION
}

func isTransientBackendError(err error) bool {
	var exception *clickhouse.Exception
	if errors.As(err, &exception) {
		// Code 252 is also raised for one insert spanning too many partitions, which no retry can fix.
		if exception.Code == 252 && strings.Contains(exception.Message, "partitions for single INSERT block") {
			return false
		}
		return transientClickHouseCodes[exception.Code]
	}
	return chdb.IsConnError(err)
}
