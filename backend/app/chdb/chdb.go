//go:build telemetry_ch

package chdb

import (
	"github.com/tracewayapp/traceway/backend/app/config"
	"context"
	"crypto/tls"
	"fmt"
	"strings"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
)

type ChConn interface {
	Query(ctx context.Context, query string, args ...any) (driver.Rows, error)
	QueryRow(ctx context.Context, query string, args ...any) driver.Row
	PrepareBatch(ctx context.Context, query string, opts ...driver.PrepareBatchOption) (driver.Batch, error)
	Exec(ctx context.Context, query string, args ...any) error
	Stats() driver.Stats
}

var Conn ChConn

func Init() error {
	return initExternal()
}

func initExternal() error {
	cfg := config.Config
	tlsConfig := &tls.Config{}

	clickhouseServer := cfg.ClickhouseServer
	clickhouseDatabase := cfg.ClickhouseDatabase
	clickhouseUsername := cfg.ClickhouseUsername
	clickhousePassword := cfg.ClickhousePassword
	clickhouseTls := cfg.ClickhouseTLS

	if clickhouseTls == "false" {
		tlsConfig = nil
	}

	conn, err := clickhouse.Open(&clickhouse.Options{
		Addr: []string{clickhouseServer},
		Auth: clickhouse.Auth{
			Database: clickhouseDatabase,
			Username: clickhouseUsername,
			Password: clickhousePassword,
		},
		TLS:   tlsConfig,
		Debug: false,
		Debugf: func(format string, v ...interface{}) {
			msg := fmt.Sprintf(format, v...)

			if strings.Contains(msg, "[prepare batch]") || strings.Contains(msg, "[send query]") {
				fmt.Println("CLICKHOUSE: ", msg[strings.LastIndex(msg, "["):])
			}
		},
		Settings: clickhouse.Settings{
			"max_execution_time": 60,
		},
		Compression: &clickhouse.Compression{
			Method: clickhouse.CompressionLZ4,
		},
		DialTimeout:      time.Duration(10) * time.Second,
		MaxOpenConns:     15,
		MaxIdleConns:     15,
		ConnMaxLifetime:  time.Duration(10) * time.Minute,
		ConnOpenStrategy: clickhouse.ConnOpenInOrder,
		BlockBufferSize:  10,
	})

	if err != nil {
		return err
	}

	Conn = conn

	return nil
}
