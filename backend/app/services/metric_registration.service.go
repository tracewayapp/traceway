package services

import (
	"database/sql"
	"fmt"

	"github.com/google/uuid"
	traceway "go.tracewayapp.com"

	"github.com/tracewayapp/traceway/backend/app/db"
	"github.com/tracewayapp/traceway/backend/app/models"
	"github.com/tracewayapp/traceway/backend/app/repositories/transactional"
)

func CollectUniqueMetricNames(points []models.MetricPoint) []string {
	seen := make(map[string]struct{}, len(points))
	var names []string
	for _, p := range points {
		if _, ok := seen[p.Name]; !ok {
			seen[p.Name] = struct{}{}
			names = append(names, p.Name)
		}
	}
	return names
}

func AutoRegisterMetrics(projectId uuid.UUID, names []string) {
	_, err := db.ExecuteTransaction(func(tx *sql.Tx) (struct{}, error) {
		return struct{}{}, transactional.MetricRegistryRepository.EnsureRegistered(tx, projectId, names)
	})
	if err != nil {
		traceway.CaptureException(fmt.Errorf("failed to auto-register metrics: %w", err))
	}
}

func AutoRegisterMetricsWithUnits(projectId uuid.UUID, entries []transactional.MetricRegistrationEntry) {
	_, err := db.ExecuteTransaction(func(tx *sql.Tx) (struct{}, error) {
		return struct{}{}, transactional.MetricRegistryRepository.EnsureRegisteredWithUnits(tx, projectId, entries)
	})
	if err != nil {
		traceway.CaptureException(fmt.Errorf("failed to auto-register OTLP metrics: %w", err))
	}
}
