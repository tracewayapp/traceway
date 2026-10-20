//go:build otel

package telemetry

import (
	"database/sql"
	"strings"

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
		cfg.Tracer = otelpgx.NewTracer(otelpgx.WithTrimSQLInSpanName(), otelpgx.WithSpanNameFunc(querySpanName), otelpgx.WithDisableQuerySpanNamePrefix())
		return stdlib.OpenDB(*cfg), nil
	}
}

// querySpanName is "{operation} {table}", e.g. "SELECT rate_tables".
func querySpanName(query string) string {
	f := strings.Fields(query)
	if len(f) == 0 {
		return "QUERY"
	}
	op := strings.ToUpper(f[0])
	for i := 0; i+1 < len(f); i++ {
		if u := strings.ToUpper(f[i]); u == "FROM" || u == "INTO" || (u == "UPDATE" && i == 0) {
			return op + " " + strings.ToLower(strings.Trim(f[i+1], "(),;"))
		}
	}
	return op
}
