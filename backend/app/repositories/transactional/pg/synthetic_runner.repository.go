//go:build transactional_pg

package pg

import (
	"database/sql"
	"time"

	"github.com/tracewayapp/traceway/backend/app/db"
	"github.com/tracewayapp/traceway/backend/app/models"

	"github.com/tracewayapp/lit/v2"
)

type syntheticRunnerRepository struct{}

const syntheticRunnerColumns = "id, name, version, first_seen_at, last_seen_at"

// UpsertSeen self-registers a runner by name on first contact and refreshes
// its liveness/version afterwards, returning the current row. A lost race on
// the unique name (two first polls at once) falls back to the winner's row.
func (r *syntheticRunnerRepository) UpsertSeen(tx *sql.Tx, name string, version string, now time.Time) (*models.SyntheticRunner, error) {
	existing, err := r.findByName(tx, name)
	if err != nil {
		return nil, err
	}
	if existing == nil {
		runner := &models.SyntheticRunner{
			Name:        name,
			Version:     version,
			FirstSeenAt: now.UTC(),
			LastSeenAt:  now.UTC(),
		}
		id, insertErr := lit.Insert[models.SyntheticRunner](tx, runner)
		if insertErr == nil {
			runner.Id = id
			return runner, nil
		}
		existing, err = r.findByName(tx, name)
		if err != nil {
			return nil, err
		}
		if existing == nil {
			return nil, insertErr
		}
	}
	if version == "" {
		version = existing.Version
	}
	query, args, err := lit.ParseNamedQuery(
		db.Driver,
		"UPDATE synthetic_runners SET last_seen_at = :now, version = :version WHERE id = :id",
		lit.P{"now": now.UTC(), "version": version, "id": existing.Id},
	)
	if err != nil {
		return nil, err
	}
	if err := lit.UpdateNative(tx, query, args...); err != nil {
		return nil, err
	}
	existing.Version = version
	existing.LastSeenAt = now.UTC()
	return existing, nil
}

func (r *syntheticRunnerRepository) findByName(tx *sql.Tx, name string) (*models.SyntheticRunner, error) {
	return lit.SelectSingleNamed[models.SyntheticRunner](
		tx,
		"SELECT "+syntheticRunnerColumns+" FROM synthetic_runners WHERE name = :name",
		lit.P{"name": name},
	)
}

func (r *syntheticRunnerRepository) CountOnline(tx *sql.Tx, since time.Time) (int, error) {
	result, err := lit.SelectSingleNamed[models.CountResult](
		tx,
		"SELECT COUNT(*) as count FROM synthetic_runners WHERE last_seen_at >= :since",
		lit.P{"since": since.UTC()},
	)
	if err != nil {
		return 0, err
	}
	if result == nil {
		return 0, nil
	}
	return result.Count, nil
}

var SyntheticRunnerRepository = syntheticRunnerRepository{}
