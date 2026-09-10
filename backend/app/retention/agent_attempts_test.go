//go:build !transactional_pg && !telemetry_ch && !telemetry_duckdb

package retention

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/tracewayapp/traceway/backend/app/agent"
	"github.com/tracewayapp/traceway/backend/app/db"
	"github.com/tracewayapp/traceway/backend/app/dbtest"
	"github.com/tracewayapp/traceway/backend/app/models"
	"github.com/tracewayapp/traceway/backend/app/repositories/transactional"
	"github.com/tracewayapp/traceway/backend/app/storage"
)

func TestPruneAgentAttemptsDropsEventsAndBlobsOfOldTerminalAttempts(t *testing.T) {
	dbtest.SetupSQLite(t)
	dir := t.TempDir()
	local, err := storage.NewLocalStorage(dir)
	if err != nil {
		t.Fatal(err)
	}
	prev := storage.Store
	storage.Store = local
	t.Cleanup(func() { storage.Store = prev })

	now := time.Now().UTC()
	old, recent := seedRetentionAttempts(t, now)
	ctx := context.Background()
	for _, attempt := range []*models.AgentAttempt{old, recent} {
		for _, key := range agent.AttemptBlobKeys(attempt.Id) {
			if err := local.Write(ctx, key, []byte("blob")); err != nil {
				t.Fatal(err)
			}
		}
	}

	pruned, err := db.ExecuteTransaction(func(tx *sql.Tx) (int64, error) {
		return pruneAgentAttempts(ctx, tx, now.AddDate(0, 0, -90))
	})
	if err != nil {
		t.Fatalf("prune: %v", err)
	}
	if pruned != 1 {
		t.Fatalf("pruned %d attempts, want 1", pruned)
	}

	_, err = db.ExecuteTransaction(func(tx *sql.Tx) (struct{}, error) {
		oldRow, _ := transactional.AgentAttemptRepository.FindById(tx, old.Id)
		if oldRow == nil || oldRow.ReportKey != "" || oldRow.Status != models.AttemptMerged {
			t.Errorf("old attempt after prune = %+v (row must stay, report_key must clear)", oldRow)
		}
		if seq, _ := transactional.AgentAttemptEventRepository.LatestSeq(tx, old.Id); seq != 0 {
			t.Errorf("old attempt still has %d events", seq)
		}
		if msgs, _ := transactional.AgentMessageRepository.ListAfter(tx, old.Id, 0, "", 10); len(msgs) != 1 {
			t.Errorf("old attempt lost its messages: %d", len(msgs))
		}
		recentRow, _ := transactional.AgentAttemptRepository.FindById(tx, recent.Id)
		if recentRow.ReportKey == "" {
			t.Error("recent attempt was pruned")
		}
		if seq, _ := transactional.AgentAttemptEventRepository.LatestSeq(tx, recent.Id); seq != 1 {
			t.Errorf("recent attempt events = %d", seq)
		}
		return struct{}{}, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, key := range agent.AttemptBlobKeys(old.Id) {
		if _, err := os.Stat(filepath.Join(dir, key)); !os.IsNotExist(err) {
			t.Errorf("blob %s still exists", key)
		}
	}
	for _, key := range agent.AttemptBlobKeys(recent.Id) {
		if _, err := os.Stat(filepath.Join(dir, key)); err != nil {
			t.Errorf("recent blob %s is gone: %v", key, err)
		}
	}

	again, err := db.ExecuteTransaction(func(tx *sql.Tx) (int64, error) {
		return pruneAgentAttempts(ctx, tx, now.AddDate(0, 0, -90))
	})
	if err != nil || again != 0 {
		t.Fatalf("second prune = %d, %v", again, err)
	}
}

func seedRetentionAttempts(t *testing.T, now time.Time) (old, recent *models.AgentAttempt) {
	t.Helper()
	pair, err := db.ExecuteTransaction(func(tx *sql.Tx) ([2]*models.AgentAttempt, error) {
		org, err := transactional.OrganizationRepository.Create(tx, "Org", "UTC")
		if err != nil {
			return [2]*models.AgentAttempt{}, err
		}
		project, err := transactional.ProjectRepository.CreateWithOrganization(tx, "Project", "opentelemetry", org.Id)
		if err != nil {
			return [2]*models.AgentAttempt{}, err
		}
		var out [2]*models.AgentAttempt
		for i, finishedAgo := range []time.Duration{120 * 24 * time.Hour, 24 * time.Hour} {
			finished := now.Add(-finishedAgo)
			attempt := &models.AgentAttempt{
				Id: uuid.New(), OrganizationId: org.Id, ProjectId: project.Id, Number: i + 1,
				Kind: models.AttemptKindFix, SubjectKind: models.SubjectKindTracewayException, SubjectRef: "hash" + string(rune('a'+i)),
				Status: models.AttemptMerged, ReportKey: "agent/x/report.md",
				CreatedAt: finished.Add(-time.Hour), FinishedAt: &finished, UpdatedAt: finished,
			}
			if err := transactional.AgentAttemptRepository.Create(tx, attempt); err != nil {
				return out, err
			}
			if _, err := transactional.AgentAttemptEventRepository.Append(tx, attempt.Id, "progress", models.JSONText(`{}`), finished); err != nil {
				return out, err
			}
			if _, err := transactional.AgentMessageRepository.Create(tx, &models.AgentMessage{AttemptId: attempt.Id, Direction: models.MessageOutbound, Provider: "web", Kind: models.MessageKindProgress, Body: "kept", CreatedAt: finished}); err != nil {
				return out, err
			}
			out[i] = attempt
		}
		return out, nil
	})
	if err != nil {
		t.Fatalf("seed: %v", err)
	}
	return pair[0], pair[1]
}
