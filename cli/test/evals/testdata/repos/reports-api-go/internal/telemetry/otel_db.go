//go:build otel

package telemetry

import (
	"database/sql"

	"github.com/exaring/otelpgx"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/stdlib"
)

func init() {
	OpenDB = func(dsn string) (*sql.DB, error) {
		cfg, err := pgx.ParseConfig(dsn)
		if err != nil {
			return nil, err
		}
		cfg.Tracer = otelpgx.NewTracer()
		return stdlib.OpenDB(*cfg), nil
	}
}
