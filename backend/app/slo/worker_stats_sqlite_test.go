//go:build !transactional_pg && !telemetry_ch && !telemetry_duckdb

package slo

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/tracewayapp/traceway/backend/app/db"
	"github.com/tracewayapp/traceway/backend/app/models"
	"github.com/tracewayapp/traceway/backend/app/repositories/telemetry"
	"github.com/tracewayapp/traceway/backend/app/repositories/transactional"
)

func TestIncrementalWindowSumMatchesAFullRead(t *testing.T) {
	projectId := setupSLODB(t)
	ctx := context.Background()
	start := time.Now().UTC().Truncate(time.Hour).Add(-6 * time.Hour).Add(30 * time.Second)
	window := 7 * 24 * time.Hour
	var seed []models.Endpoint
	for minute := -90; minute < 180; minute++ {
		at := start.Add(-window).Add(time.Duration(minute) * time.Minute)
		seed = append(seed, endpointAt(projectId, 200, at), endpointAt(projectId, 500, at))
	}
	for minute := 1; minute < 600; minute += 7 {
		seed = append(seed, endpointAt(projectId, 200, start.Add(-time.Duration(minute)*time.Minute)))
	}
	if err := telemetry.EndpointRepository.InsertAsync(ctx, seed); err != nil {
		t.Fatal(err)
	}
	entity := createTestSLO(t, projectId, models.SLIKindEndpoint, `{"endpoint":"","good":"availability"}`, 99, 7, 1)

	incremental := 0
	for minute := 0; minute < 180; minute++ {
		now := start.Add(time.Duration(minute) * time.Minute)
		var late []models.Endpoint
		if minute%5 == 0 {
			late = append(late, endpointAt(projectId, 500, now.Add(-3*time.Minute)))
		}
		if minute == 90 {
			late = append(late, endpointAt(projectId, 500, now.Add(-20*time.Hour)))
		}
		if len(late) > 0 {
			if err := telemetry.EndpointRepository.InsertAsync(ctx, late); err != nil {
				t.Fatal(err)
			}
		}
		if minute == 91 {
			_, err := db.ExecuteTransaction(func(tx *sql.Tx) (struct{}, error) {
				_, err := tx.Exec("UPDATE slos SET last_bucket_at = ? WHERE id = ?", now.Add(-21*time.Hour).Truncate(time.Minute), entity.Id)
				return struct{}{}, err
			})
			if err != nil {
				t.Fatal(err)
			}
		}
		RunOnce(ctx, now)
		if lastTick.FullSnapshots == 0 {
			incremental++
		}

		got := reloadSLO(t, entity.Id)
		lastClosed := now.Truncate(time.Minute)
		full, err := sumRange(ctx, got, lastClosed.Add(-window), lastClosed)
		if err != nil {
			t.Fatal(err)
		}
		sli, _ := full.SLI()
		if got.CurrentSli == nil || *got.CurrentSli != sli*100 {
			t.Fatalf("minute %d: incremental SLI %v, full read %v (%d/%d)", minute, derefSLI(got.CurrentSli), sli*100, full.Good, full.Total)
		}
	}
	if incremental < 150 {
		t.Fatalf("only %d of 180 ticks used the cached stable part of the window", incremental)
	}
}

func TestTickStatsCountTimeoutsAndTheSlowestSLO(t *testing.T) {
	projectId := setupSLODB(t)
	now := time.Now().UTC().Truncate(time.Minute).Add(30 * time.Second)
	first := createTestSLO(t, projectId, models.SLIKindEndpoint, `{"endpoint":"","good":"availability"}`, 99, 7, 1)
	second := createTestSLO(t, projectId, models.SLIKindTask, `{}`, 99, 7, 1)
	expired, cancel := context.WithDeadline(context.Background(), time.Now().Add(-time.Second))
	defer cancel()
	before := timeoutsTotal

	stats := computeAll(expired, []*models.SLO{first, second}, now)
	if stats.Timeouts != 2 || stats.Errors != 2 || timeoutsTotal-before != 2 {
		t.Fatalf("both SLOs time out: %+v", stats)
	}
	if stats.Slowest == nil || (stats.Slowest.Id != first.Id && stats.Slowest.Id != second.Id) {
		t.Fatalf("the slowest SLO is reported: %+v", stats.Slowest)
	}
	if reloadSLO(t, first.Id).ComputedAt != nil {
		t.Fatal("a timed-out SLO keeps its old snapshot")
	}
}

func TestFreshnessReportsStaleAndPendingSLOs(t *testing.T) {
	projectId := setupSLODB(t)
	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Minute).Add(30 * time.Second)
	fresh := createTestSLO(t, projectId, models.SLIKindEndpoint, `{"endpoint":"","good":"availability"}`, 99, 7, 1)
	stale := createTestSLO(t, projectId, models.SLIKindEndpoint, `{"endpoint":"","good":"availability"}`, 99, 7, 1)
	createTestSLO(t, projectId, models.SLIKindEndpoint, `{"endpoint":"","good":"availability"}`, 99, 7, 1)
	RunOnce(ctx, now)
	_, err := db.ExecuteTransaction(func(tx *sql.Tx) (struct{}, error) {
		_, err := tx.Exec("UPDATE slos SET computed_at = ? WHERE id = ?", now.Add(-10*time.Minute), stale.Id)
		if err != nil {
			return struct{}{}, err
		}
		_, err = tx.Exec("UPDATE slos SET computed_at = NULL, history_from = NULL WHERE id NOT IN (?, ?)", fresh.Id, stale.Id)
		return struct{}{}, err
	})
	if err != nil {
		t.Fatal(err)
	}

	freshness, err := ReadFreshness(ctx, now)
	if err != nil {
		t.Fatal(err)
	}
	if freshness.Enabled != 3 || freshness.Stale != 2 || freshness.BackfillPending != 1 {
		t.Fatalf("freshness = %+v", freshness)
	}
	if freshness.MaxStalenessSec < 600 {
		t.Fatalf("the oldest computation is at least ten minutes old: %+v", freshness)
	}
	entities, err := db.ExecuteTransaction(func(tx *sql.Tx) ([]*models.SLO, error) {
		return transactional.SLORepository.FindEnabled(tx)
	})
	if err != nil || len(entities) != 3 {
		t.Fatalf("enabled SLOs: %d, %v", len(entities), err)
	}
}
