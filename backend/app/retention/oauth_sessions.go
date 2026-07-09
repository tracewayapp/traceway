package retention

import (
	"context"
	"database/sql"
	"time"

	"github.com/tracewayapp/traceway/backend/app/repositories/transactional"
)

const oauthSessionsPruneInterval = 24 * time.Hour

func startOAuthSessionsPrune(ctx context.Context) {
	startDBPruneWorker(ctx, "oauth_sessions", oauthSessionsPruneInterval, func(tx *sql.Tx) (int64, error) {
		return transactional.OAuthSessionRepository.PruneExpired(tx, time.Now().UTC())
	})
}
