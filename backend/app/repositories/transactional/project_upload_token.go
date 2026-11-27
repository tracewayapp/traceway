package transactional

import (
	"database/sql"
	"errors"
	"strings"

	"github.com/google/uuid"
	"github.com/tracewayapp/traceway/backend/v2/app/db"
)

// EnsureSourceMapToken backfills legacy projects without rotating an existing token.
// A concurrent setup can win the conditional update while this transaction waits.
func EnsureSourceMapToken(tx *sql.Tx, projectID uuid.UUID) (token string, added bool, err error) {
	candidate := strings.ReplaceAll(uuid.NewString(), "-", "")
	err = tx.QueryRow("UPDATE projects SET source_map_token = $1 WHERE id = $2 AND source_map_token IS NULL RETURNING source_map_token", candidate, projectID).Scan(&token)
	if errors.Is(err, sql.ErrNoRows) {
		err = tx.QueryRow("SELECT source_map_token FROM projects WHERE id = $1", projectID).Scan(&token)
		return token, false, err
	}
	if err != nil {
		return "", false, err
	}
	return token, true, db.NotifyProjectCacheChanged(tx, projectID)
}
