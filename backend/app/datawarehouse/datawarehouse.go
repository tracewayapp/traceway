package datawarehouse

import (
	"crypto/tls"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
)

var Conn *driver.Conn

// Init initializes the Clickhouse connection pool
func Init() error {
	tlsConfig := &tls.Config{}

	clickhouseServer := os.Getenv("CLICKHOUSE_SERVER")
	clickhouseDatabase := os.Getenv("CLICKHOUSE_DATABASE")
	clickhouseUsername := os.Getenv("CLICKHOUSE_USERNAME")
	clickhousePassword := os.Getenv("CLICKHOUSE_PASSWORD")

	if strings.HasSuffix(clickhouseServer, "9000") {
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

	Conn = &conn

	return nil
}
