//go:build transactional_pg

package pg

import (
	"database/sql"

	"github.com/tracewayapp/traceway/backend/app/db"
	"github.com/tracewayapp/traceway/backend/app/models"

	"github.com/google/uuid"
	"github.com/tracewayapp/lit/v2"
)

type notificationChannelRepository struct{}

func (r *notificationChannelRepository) FindByProject(tx *sql.Tx, projectId uuid.UUID) ([]*models.NotificationChannel, error) {
	return lit.SelectNamed[models.NotificationChannel](
		tx,
		"SELECT id, project_id, name, channel_type, config, enabled, created_by, created_at, updated_at FROM notification_channels WHERE project_id = :project_id ORDER BY created_at DESC",
		lit.P{"project_id": projectId},
	)
}

func (r *notificationChannelRepository) FindById(tx *sql.Tx, id int) (*models.NotificationChannel, error) {
	return lit.SelectSingleNamed[models.NotificationChannel](
		tx,
		"SELECT id, project_id, name, channel_type, config, enabled, created_by, created_at, updated_at FROM notification_channels WHERE id = :id",
		lit.P{"id": id},
	)
}

func (r *notificationChannelRepository) Create(tx *sql.Tx, channel *models.NotificationChannel) (int, error) {
	return lit.Insert[models.NotificationChannel](tx, channel)
}

func (r *notificationChannelRepository) Update(tx *sql.Tx, channel *models.NotificationChannel) error {
	return lit.UpdateNamed(tx, channel, "id = :id", lit.P{"id": channel.Id})
}

func (r *notificationChannelRepository) Delete(tx *sql.Tx, id int) error {
	return lit.DeleteNamed(db.Driver, tx, "DELETE FROM notification_channels WHERE id = :id", lit.P{"id": id})
}

func (r *notificationChannelRepository) FindEnabledByProject(tx *sql.Tx, projectId uuid.UUID) ([]*models.NotificationChannel, error) {
	return lit.SelectNamed[models.NotificationChannel](
		tx,
		"SELECT id, project_id, name, channel_type, config, enabled, created_by, created_at, updated_at FROM notification_channels WHERE project_id = :project_id AND enabled = true",
		lit.P{"project_id": projectId},
	)
}

var NotificationChannelRepository = notificationChannelRepository{}
