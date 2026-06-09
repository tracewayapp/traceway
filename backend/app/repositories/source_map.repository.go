package repositories

import (
	"database/sql"
	"time"

	"github.com/tracewayapp/traceway/backend/app/models"

	"github.com/google/uuid"
	"github.com/tracewayapp/lit/v2"
)

type sourceMapRepository struct{}

func (s *sourceMapRepository) Create(tx *sql.Tx, sm *models.SourceMap) (*models.SourceMap, error) {
	sm.UploadedAt = time.Now().UTC()
	id, err := lit.Insert(tx, sm)
	if err != nil {
		return nil, err
	}
	sm.Id = id
	return sm, nil
}

func (s *sourceMapRepository) Update(tx *sql.Tx, sm *models.SourceMap) error {
	return lit.UpdateNamed[models.SourceMap](tx, sm, "id = :id", lit.P{"id": sm.Id})
}

func (s *sourceMapRepository) FindByProjectAndVersion(tx *sql.Tx, projectId uuid.UUID, version string) ([]*models.SourceMap, error) {
	return lit.SelectNamed[models.SourceMap](
		tx,
		"SELECT * FROM source_maps WHERE project_id = :project_id AND version = :version",
		lit.P{"project_id": projectId, "version": version},
	)
}

func (s *sourceMapRepository) FindLatestByProject(tx *sql.Tx, projectId uuid.UUID) ([]*models.SourceMap, error) {
	return lit.SelectNamed[models.SourceMap](
		tx,
		"SELECT * FROM source_maps WHERE project_id = :project_id AND version = (SELECT version FROM source_maps WHERE project_id = :project_id ORDER BY uploaded_at DESC LIMIT 1)",
		lit.P{"project_id": projectId},
	)
}

func (s *sourceMapRepository) FindByProjectVersionAndFileName(tx *sql.Tx, projectId uuid.UUID, version, fileName string) (*models.SourceMap, error) {
	return lit.SelectSingleNamed[models.SourceMap](
		tx,
		"SELECT * FROM source_maps WHERE project_id = :project_id AND version = :version AND file_name = :file_name",
		lit.P{"project_id": projectId, "version": version, "file_name": fileName},
	)
}

func (s *sourceMapRepository) FindByProjectAndDebugId(tx *sql.Tx, projectId uuid.UUID, debugId string) ([]*models.SourceMap, error) {
	return lit.SelectNamed[models.SourceMap](
		tx,
		"SELECT * FROM source_maps WHERE project_id = :project_id AND debug_id = :debug_id",
		lit.P{"project_id": projectId, "debug_id": debugId},
	)
}

var SourceMapRepository = sourceMapRepository{}
