package repositories

import (
	"backend/app/models"
	"database/sql"
	"time"

	"github.com/google/uuid"
	"github.com/tracewayapp/go-lightning/lit"
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
	return lit.Update[models.SourceMap](tx, sm, "id = $1", sm.Id)
}

func (s *sourceMapRepository) FindByProjectAndVersion(tx *sql.Tx, projectId uuid.UUID, version string) ([]*models.SourceMap, error) {
	return lit.Select[models.SourceMap](
		tx,
		"SELECT * FROM source_maps WHERE project_id = $1 AND version = $2",
		projectId,
		version,
	)
}

func (s *sourceMapRepository) FindLatestByProject(tx *sql.Tx, projectId uuid.UUID) ([]*models.SourceMap, error) {
	return lit.Select[models.SourceMap](
		tx,
		"SELECT * FROM source_maps WHERE project_id = $1 AND version = (SELECT version FROM source_maps WHERE project_id = $1 ORDER BY uploaded_at DESC LIMIT 1)",
		projectId,
	)
}

func (s *sourceMapRepository) FindByProjectVersionAndFileName(tx *sql.Tx, projectId uuid.UUID, version, fileName string) (*models.SourceMap, error) {
	return lit.SelectSingle[models.SourceMap](
		tx,
		"SELECT * FROM source_maps WHERE project_id = $1 AND version = $2 AND file_name = $3",
		projectId,
		version,
		fileName,
	)
}

var SourceMapRepository = sourceMapRepository{}
