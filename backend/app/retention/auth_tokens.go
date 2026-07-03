package retention

import (
	"context"
	"database/sql"
	"time"

	"github.com/tracewayapp/traceway/backend/app/repositories"
)

const authTokensPruneInterval = 24 * time.Hour

func startAuthTokensPrune(ctx context.Context) {
	startDBPruneWorker(ctx, "auth_tokens", authTokensPruneInterval, func(tx *sql.Tx) (int64, error) {
		now := time.Now().UTC()
		var total int64

		n, err := repositories.DeviceAuthorizationRepository.PruneExpired(tx, now)
		if err != nil {
			return total, err
		}
		total += n

		n, err = repositories.RefreshTokenRepository.PruneExpired(tx, now)
		if err != nil {
			return total, err
		}
		total += n

		n, err = repositories.PersonalAccessTokenRepository.PruneExpired(tx, now)
		if err != nil {
			return total, err
		}
		total += n

		n, err = repositories.AuthorizationCodeRepository.PruneExpired(tx, now)
		if err != nil {
			return total, err
		}
		total += n

		return total, nil
	})
}
