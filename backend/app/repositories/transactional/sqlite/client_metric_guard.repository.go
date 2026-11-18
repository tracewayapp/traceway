//go:build !transactional_pg

package sqlite

import (
	"time"

	"github.com/google/uuid"
	"github.com/tracewayapp/lit/v2"
	"github.com/tracewayapp/traceway/backend/v2/app/clientmetrics"
	"github.com/tracewayapp/traceway/backend/v2/app/db"
	"github.com/tracewayapp/traceway/backend/v2/app/repositories/transactional/shared"
)

type clientMetricGuardRepository struct{}

// Insert records what the client metrics guard admitted. An entry another instance
// already wrote is left as it is.
func (r *clientMetricGuardRepository) Insert(ex lit.Executor, entries []clientmetrics.GuardEntry, now time.Time) error {
	for _, entry := range entries {
		query, args, err := lit.ParseNamedQuery(
			db.Driver,
			`INSERT INTO client_metric_guard_entries (project_id, day, scope, value, created_at)
				VALUES (:project_id, :day, :scope, :value, :created_at)
				ON CONFLICT (project_id, day, scope, value) DO NOTHING`,
			lit.P{
				"project_id": entry.ProjectId.String(),
				"day":        entry.Day,
				"scope":      entry.Scope,
				"value":      entry.Value,
				"created_at": shared.FormatAuthTime(now),
			},
		)
		if err != nil {
			return err
		}
		if _, err := ex.Exec(query, args...); err != nil {
			return err
		}
	}
	return nil
}

// ListSince returns the entries of a day and the registrations (day 0) created after
// since.
func (r *clientMetricGuardRepository) ListSince(ex lit.Executor, day int64, since time.Time) ([]clientmetrics.GuardEntry, error) {
	query, args, err := lit.ParseNamedQuery(
		db.Driver,
		`SELECT project_id, day, scope, value FROM client_metric_guard_entries
			WHERE day IN (0, :day) AND created_at > :since`,
		lit.P{"day": day, "since": shared.FormatAuthTime(since)},
	)
	if err != nil {
		return nil, err
	}
	rows, err := ex.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var entries []clientmetrics.GuardEntry
	for rows.Next() {
		var (
			entry     clientmetrics.GuardEntry
			projectId string
		)
		if err := rows.Scan(&projectId, &entry.Day, &entry.Scope, &entry.Value); err != nil {
			return nil, err
		}
		if entry.ProjectId, err = uuid.Parse(projectId); err != nil {
			continue
		}
		entries = append(entries, entry)
	}
	return entries, rows.Err()
}

// PruneBefore deletes the daily entries of days before beforeDay. Registrations stay.
func (r *clientMetricGuardRepository) PruneBefore(ex lit.Executor, beforeDay int64) error {
	query, args, err := lit.ParseNamedQuery(
		db.Driver,
		"DELETE FROM client_metric_guard_entries WHERE day > 0 AND day < :day",
		lit.P{"day": beforeDay},
	)
	if err != nil {
		return err
	}
	_, err = ex.Exec(query, args...)
	return err
}

var ClientMetricGuardRepository = clientMetricGuardRepository{}
