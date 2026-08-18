package retention

import (
	"context"
	"database/sql"
	"time"

	"github.com/tracewayapp/traceway/backend/app/repositories/transactional"
)

const authTokensPruneInterval = 24 * time.Hour

func startAuthTokensPrune(ctx context.Context) {
	startDBPruneWorker(ctx, "auth_tokens", authTokensPruneInterval, func(tx *sql.Tx) (int64, error) {
		now := time.Now().UTC()
		var total int64

		n, err := transactional.DeviceAuthorizationRepository.PruneExpired(tx, now)
		if err != nil {
			return total, err
		}
		total += n

		n, err = transactional.RefreshTokenRepository.PruneExpired(tx, now)
		if err != nil {
			return total, err
		}
		total += n

		n, err = transactional.PersonalAccessTokenRepository.PruneExpired(tx, now)
		if err != nil {
			return total, err
		}
		total += n

		n, err = transactional.AuthorizationCodeRepository.PruneExpired(tx, now)
		if err != nil {
			return total, err
		}
		total += n

		n, err = transactional.OauthClientRepository.PruneStale(tx, now)
		if err != nil {
			return total, err
		}
		total += n

		n, err = transactional.SetupTokenRepository.PruneExpired(tx, now)
		if err != nil {
			return total, err
		}
		total += n

		n, err = transactional.SetupPlanRepository.PruneOld(tx, now)
		if err != nil {
			return total, err
		}
		total += n

		return total, nil
	})
}
