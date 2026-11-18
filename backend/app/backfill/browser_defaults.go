package backfill

import (
	"database/sql"
	"time"

	"github.com/tracewayapp/lit/v2"
	"github.com/tracewayapp/traceway/backend/v2/app/db"
	"github.com/tracewayapp/traceway/backend/v2/app/models"
	"github.com/tracewayapp/traceway/backend/v2/app/services/dashboards"
)

// RunBrowserDefaults is a data migration: once applied, deleting a default
// dashboard remains an intentional user choice across subsequent restarts.
func RunBrowserDefaults() error {
	_, err := db.ExecuteTransaction(func(tx *sql.Tx) (struct{}, error) {
		if !db.IsSQLite() {
			if _, err := tx.Exec("SELECT pg_advisory_xact_lock(824737008)"); err != nil {
				return struct{}{}, err
			}
		}
		const name = "browser-project-default-dashboards-v2.1"
		marker, err := lit.SelectSingleNamed[models.CountResult](tx, "SELECT COUNT(*) AS count FROM backfill_runs WHERE name = :name", lit.P{"name": name})
		if err != nil || marker.Count > 0 {
			return struct{}{}, err
		}
		projects, err := lit.SelectNamed[models.Project](tx, "SELECT * FROM projects WHERE organization_id IS NOT NULL ORDER BY organization_id, id", lit.P{})
		if err != nil {
			return struct{}{}, err
		}
		for _, project := range projects {
			if err := dashboards.EnsureBrowserDefault(tx, project); err != nil {
				return struct{}{}, err
			}
		}
		return struct{}{}, lit.DeleteNamed(db.Driver, tx, "INSERT INTO backfill_runs (name, ran_at) VALUES (:name, :at)", lit.P{"name": name, "at": time.Now().UTC()})
	})
	return err
}
