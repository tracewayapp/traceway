//go:build !transactional_pg

package sqlite

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/tracewayapp/lit/v2"
	"github.com/tracewayapp/traceway/backend/v2/app/db"
	"github.com/tracewayapp/traceway/backend/v2/app/models"
)

type sdkConfigRepository struct{}

func (r *sdkConfigRepository) LockProject(tx *sql.Tx, projectId uuid.UUID) error {
	return db.LockSdkConfigProject(tx, projectId)
}

const (
	sdkConfigRuleColumns = "id, project_id, name, position, enabled, conditions, settings, created_by, created_at, updated_at"
	sdkCaptureColumns    = "id, project_id, label, conditions, actions, expires_at, cancelled_at, created_by, created_at, delivered_at, applied_at, received_at"
)

func (r *sdkConfigRepository) ListRules(tx *sql.Tx, projectId uuid.UUID) ([]*models.SdkConfigRule, error) {
	return lit.SelectNamed[models.SdkConfigRule](
		tx,
		"SELECT "+sdkConfigRuleColumns+" FROM sdk_config_rules WHERE project_id = :project_id ORDER BY position ASC, id ASC",
		lit.P{"project_id": projectId},
	)
}

func (r *sdkConfigRepository) FindRule(tx *sql.Tx, projectId uuid.UUID, id int) (*models.SdkConfigRule, error) {
	return lit.SelectSingleNamed[models.SdkConfigRule](
		tx,
		"SELECT "+sdkConfigRuleColumns+" FROM sdk_config_rules WHERE id = :id AND project_id = :project_id",
		lit.P{"id": id, "project_id": projectId},
	)
}

func (r *sdkConfigRepository) CreateRule(tx *sql.Tx, rule *models.SdkConfigRule) (int, error) {
	id, err := lit.Insert[models.SdkConfigRule](tx, rule)
	if err != nil {
		return 0, err
	}
	return id, db.NotifySdkConfigChanged(tx, rule.ProjectId)
}

func (r *sdkConfigRepository) UpdateRule(tx *sql.Tx, rule *models.SdkConfigRule) error {
	if err := lit.UpdateNamed(tx, rule, "id = :id AND project_id = :project_id", lit.P{"id": rule.Id, "project_id": rule.ProjectId}); err != nil {
		return err
	}
	return db.NotifySdkConfigChanged(tx, rule.ProjectId)
}

func (r *sdkConfigRepository) DeleteRule(tx *sql.Tx, projectId uuid.UUID, id int) error {
	if err := lit.DeleteNamed(db.Driver, tx, "DELETE FROM sdk_config_rules WHERE id = :id AND project_id = :project_id", lit.P{"id": id, "project_id": projectId}); err != nil {
		return err
	}
	return db.NotifySdkConfigChanged(tx, projectId)
}

// SetPositions stores the given order: ids[i] gets position i.
func (r *sdkConfigRepository) SetPositions(tx *sql.Tx, projectId uuid.UUID, ids []int) error {
	for position, id := range ids {
		if err := lit.DeleteNamed(db.Driver, tx,
			"UPDATE sdk_config_rules SET position = :position WHERE id = :id AND project_id = :project_id",
			lit.P{"position": position, "id": id, "project_id": projectId}); err != nil {
			return err
		}
	}
	return db.NotifySdkConfigChanged(tx, projectId)
}

// ListCaptures keeps active captures visible ahead of recent expired history.
func (r *sdkConfigRepository) ListCaptures(tx *sql.Tx, projectId uuid.UUID, limit int) ([]*models.SdkCapture, error) {
	return lit.SelectNamed[models.SdkCapture](tx,
		"SELECT "+sdkCaptureColumns+" FROM sdk_captures WHERE project_id = :project_id ORDER BY CASE WHEN cancelled_at IS NULL AND expires_at > :now THEN 0 ELSE 1 END, created_at DESC LIMIT :limit",
		lit.P{"project_id": projectId, "now": time.Now().UTC(), "limit": limit},
	)
}

func (r *sdkConfigRepository) ListActiveCaptures(tx *sql.Tx, projectId uuid.UUID, now time.Time) ([]*models.SdkCapture, error) {
	return lit.SelectNamed[models.SdkCapture](tx,
		"SELECT "+sdkCaptureColumns+" FROM sdk_captures WHERE project_id = :project_id AND cancelled_at IS NULL AND expires_at > :now ORDER BY created_at DESC",
		lit.P{"project_id": projectId, "now": now},
	)
}

func (r *sdkConfigRepository) CreateCapture(tx *sql.Tx, capture *models.SdkCapture) error {
	if err := lit.InsertExistingUuid[models.SdkCapture](tx, capture); err != nil {
		return err
	}
	return db.NotifySdkConfigChanged(tx, capture.ProjectId)
}

func (r *sdkConfigRepository) CancelCapture(tx *sql.Tx, projectId, id uuid.UUID, at time.Time) error {
	if err := lit.DeleteNamed(db.Driver, tx,
		"UPDATE sdk_captures SET cancelled_at = :at WHERE id = :id AND project_id = :project_id AND cancelled_at IS NULL",
		lit.P{"at": at, "id": id, "project_id": projectId}); err != nil {
		return err
	}
	return db.NotifySdkConfigChanged(tx, projectId)
}

var SdkConfigRepository = sdkConfigRepository{}

// MarkCaptureProgress records the first acknowledgement; it never extends or reactivates a capture.
func (r *sdkConfigRepository) MarkCaptureProgress(tx *sql.Tx, projectId, id uuid.UUID, stage string, at time.Time) error {
	column := ""
	switch stage {
	case "delivered":
		column = "delivered_at"
	case "applied":
		column = "applied_at"
	case "received":
		column = "received_at"
	default:
		return fmt.Errorf("invalid capture stage %q", stage)
	}
	condition := " AND cancelled_at IS NULL AND expires_at > :at"
	if stage == "received" {
		condition = ""
	}
	return lit.DeleteNamed(db.Driver, tx,
		"UPDATE sdk_captures SET "+column+" = :at WHERE id = :id AND project_id = :project_id AND "+column+" IS NULL"+condition,
		lit.P{"at": at, "id": id, "project_id": projectId})
}
