package shared

import (
	"context"
	"database/sql"

	"github.com/tracewayapp/lit/v2"
	"github.com/tracewayapp/traceway/backend/app/db"
)

func Traced(ctx context.Context, conn *sql.DB) lit.Executor {
	return db.BindContext(ctx, conn)
}
