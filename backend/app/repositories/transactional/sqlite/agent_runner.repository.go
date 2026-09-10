//go:build !transactional_pg

package sqlite

import (
	"database/sql"
	"time"

	"github.com/tracewayapp/traceway/backend/app/db"
	"github.com/tracewayapp/traceway/backend/app/models"

	"github.com/tracewayapp/lit/v2"
)

type agentRunnerRepository struct{}

const agentRunnerColumns = "id, name, version, capabilities, first_seen_at, last_seen_at"

// UpsertSeen self-registers a runner by name on first contact and refreshes
// its liveness, version and capabilities afterwards, returning the current
// row. A lost race on the unique name falls back to the winner's row.
func (r *agentRunnerRepository) UpsertSeen(tx *sql.Tx, name string, version string, capabilities models.JSONText, now time.Time) (*models.AgentRunner, error) {
	existing, err := r.FindByName(tx, name)
	if err != nil {
		return nil, err
	}
	if existing == nil {
		runner := &models.AgentRunner{
			Name:         name,
			Version:      version,
			Capabilities: capabilities,
			FirstSeenAt:  now.UTC(),
			LastSeenAt:   now.UTC(),
		}
		id, insertErr := lit.Insert[models.AgentRunner](tx, runner)
		if insertErr == nil {
			runner.Id = id
			return runner, nil
		}
		existing, err = r.FindByName(tx, name)
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
	if len(capabilities) == 0 {
		capabilities = existing.Capabilities
	}
	query, args, err := lit.ParseNamedQuery(
		db.Driver,
		"UPDATE agent_runners SET last_seen_at = :now, version = :version, capabilities = :capabilities WHERE id = :id",
		lit.P{"now": now.UTC(), "version": version, "capabilities": capabilities, "id": existing.Id},
	)
	if err != nil {
		return nil, err
	}
	if err := lit.UpdateNative(tx, query, args...); err != nil {
		return nil, err
	}
	existing.Version = version
	existing.Capabilities = capabilities
	existing.LastSeenAt = now.UTC()
	return existing, nil
}

func (r *agentRunnerRepository) FindByName(tx *sql.Tx, name string) (*models.AgentRunner, error) {
	return lit.SelectSingleNamed[models.AgentRunner](
		tx,
		"SELECT "+agentRunnerColumns+" FROM agent_runners WHERE name = :name",
		lit.P{"name": name},
	)
}

func (r *agentRunnerRepository) FindAll(tx *sql.Tx) ([]*models.AgentRunner, error) {
	return lit.SelectNamed[models.AgentRunner](
		tx,
		"SELECT "+agentRunnerColumns+" FROM agent_runners ORDER BY name ASC",
		lit.P{},
	)
}

func (r *agentRunnerRepository) CountOnline(tx *sql.Tx, since time.Time) (int, error) {
	result, err := lit.SelectSingleNamed[models.CountResult](
		tx,
		"SELECT COUNT(*) as count FROM agent_runners WHERE last_seen_at >= :since",
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

var AgentRunnerRepository = agentRunnerRepository{}
