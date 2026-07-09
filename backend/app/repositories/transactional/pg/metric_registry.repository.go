//go:build transactional_pg

package pg

import (
	"database/sql"
	"github.com/tracewayapp/traceway/backend/app/repositories/transactional/shared"
	"sync"
	"time"

	"github.com/tracewayapp/traceway/backend/app/models"

	"github.com/google/uuid"
	"github.com/tracewayapp/lit/v2"
)

type metricRegistryRepository struct {
	knownMetrics sync.Map
}

func (r *metricRegistryRepository) EnsureRegistered(tx *sql.Tx, projectId uuid.UUID, names []string) error {
	for _, name := range names {
		key := projectId.String() + ":" + name
		if _, loaded := r.knownMetrics.Load(key); loaded {
			continue
		}

		existing, err := r.FindByProjectAndName(tx, projectId, name)
		if err != nil {
			return err
		}
		if existing != nil {
			r.knownMetrics.Store(key, true)
			continue
		}

		metricType := defaultMetricType(name)
		unit := defaultUnit(name)

		entry := &models.MetricRegistry{
			ProjectId:   projectId,
			Name:        name,
			MetricType:  metricType,
			Unit:        unit,
			Description: "",
			CreatedAt:   time.Now().UTC(),
		}

		_, err = lit.Insert[models.MetricRegistry](tx, entry)
		if err != nil {
			return err
		}
		r.knownMetrics.Store(key, true)
	}
	return nil
}

func (r *metricRegistryRepository) FindByProject(tx *sql.Tx, projectId uuid.UUID) ([]*models.MetricRegistry, error) {
	return lit.SelectNamed[models.MetricRegistry](
		tx,
		"SELECT id, project_id, name, metric_type, unit, description, created_at FROM metric_registry WHERE project_id = :project_id ORDER BY name ASC",
		lit.P{"project_id": projectId},
	)
}

func (r *metricRegistryRepository) FindByProjectAndName(tx *sql.Tx, projectId uuid.UUID, name string) (*models.MetricRegistry, error) {
	return lit.SelectSingleNamed[models.MetricRegistry](
		tx,
		"SELECT id, project_id, name, metric_type, unit, description, created_at FROM metric_registry WHERE project_id = :project_id AND name = :name",
		lit.P{"project_id": projectId, "name": name},
	)
}

func (r *metricRegistryRepository) Update(tx *sql.Tx, entry *models.MetricRegistry) error {
	return lit.UpdateNamed(tx, entry, "id = :id", lit.P{"id": entry.Id})
}

func (r *metricRegistryRepository) EnsureRegisteredWithUnits(tx *sql.Tx, projectId uuid.UUID, entries []shared.MetricRegistrationEntry) error {
	for _, entry := range entries {
		key := projectId.String() + ":" + entry.Name
		if _, loaded := r.knownMetrics.Load(key); loaded {
			continue
		}

		existing, err := r.FindByProjectAndName(tx, projectId, entry.Name)
		if err != nil {
			return err
		}
		if existing != nil {
			r.knownMetrics.Store(key, true)
			continue
		}

		metricType := entry.MetricType
		if metricType == "" {
			metricType = defaultMetricType(entry.Name)
		}
		unit := entry.Unit
		if unit == "" {
			unit = defaultUnit(entry.Name)
		}

		rec := &models.MetricRegistry{
			ProjectId:   projectId,
			Name:        entry.Name,
			MetricType:  metricType,
			Unit:        unit,
			Description: "",
			CreatedAt:   time.Now().UTC(),
		}

		_, err = lit.Insert[models.MetricRegistry](tx, rec)
		if err != nil {
			return err
		}
		r.knownMetrics.Store(key, true)
	}
	return nil
}

func defaultMetricType(name string) string {
	switch name {
	case models.MetricNameNumGC:
		return "counter"
	case models.MetricNameGCPauseTotal:
		return "counter"
	default:
		return "gauge"
	}
}

func defaultUnit(name string) string {
	switch name {
	case models.MetricNameCpuUsage:
		return "%"
	case models.MetricNameMemoryUsage, models.MetricNameMemoryTotal:
		return "MB"
	case models.MetricNameGoRoutines, models.MetricNameHeapObjects:
		return "count"
	case models.MetricNameNumGC:
		return "count"
	case models.MetricNameGCPauseTotal:
		return "ns"
	default:
		return ""
	}
}

func (r *metricRegistryRepository) KnownCount() int {
	count := 0
	r.knownMetrics.Range(func(_, _ any) bool {
		count++
		return true
	})
	return count
}

var MetricRegistryRepository = &metricRegistryRepository{}
