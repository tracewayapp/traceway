package services

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/google/uuid"

	"github.com/tracewayapp/traceway/backend/v2/app/db"
	"github.com/tracewayapp/traceway/backend/v2/app/models"
	"github.com/tracewayapp/traceway/backend/v2/app/repositories/transactional"
	"github.com/tracewayapp/traceway/backend/v2/app/traceway"
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

func AutoRegisterMetrics(ctx context.Context, projectId uuid.UUID, names []string) {
	_, err := db.ExecuteTransactionContext(ctx, func(tx *sql.Tx) (struct{}, error) {
		return struct{}{}, transactional.MetricRegistryRepository.EnsureRegistered(tx, projectId, names)
	})
	if err != nil {
		traceway.CaptureException(fmt.Errorf("failed to auto-register metrics: %w", err))
	}
}

func AutoRegisterMetricsWithUnits(ctx context.Context, projectId uuid.UUID, entries []transactional.MetricRegistrationEntry) {
	_, err := db.ExecuteTransactionContext(ctx, func(tx *sql.Tx) (struct{}, error) {
		return struct{}{}, transactional.MetricRegistryRepository.EnsureRegisteredWithUnits(tx, projectId, entries)
	})
	if err != nil {
		traceway.CaptureException(fmt.Errorf("failed to auto-register OTLP metrics: %w", err))
	}
}
