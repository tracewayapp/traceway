//go:build !transactional_pg

package transactional

import (
	"testing"
	"time"

	"github.com/tracewayapp/traceway/backend/app/db"
	"github.com/tracewayapp/traceway/backend/app/repositories/transactional/shared"
)

func setupSetupPlanTables(t *testing.T) {
	t.Helper()
	setupSetupTokenTables(t)
	if _, err := db.DB.Exec(`CREATE TABLE IF NOT EXISTS setup_plans (
		id TEXT PRIMARY KEY,
		user_id INTEGER NOT NULL,
		organization_id INTEGER NOT NULL,
		payload TEXT NOT NULL,
		status TEXT NOT NULL,
		reject_reason TEXT,
		result TEXT,
		created_at DATETIME NOT NULL,
		decided_at DATETIME,
		decided_by INTEGER
	)`); err != nil {
		t.Fatalf("failed to create setup_plans table: %v", err)
	}
}

const testPlanPayload = `{"projects":[{"name":"Api","framework":"opentelemetry"}]}`

func TestSetupPlanUpsertReplacesPrevious(t *testing.T) {
	setupTestDB(t)
	setupSetupPlanTables(t)
	userId := insertSetupTestUser(t, "plan@example.com")

	if err := SetupPlanRepository.Upsert(db.DB, "plan-1", userId, 1, testPlanPayload); err != nil {
		t.Fatalf("failed to upsert plan-1: %v", err)
	}
	if _, err := SetupPlanRepository.Decide(db.DB, "plan-1", "approved", "", "[]", userId, time.Now()); err != nil {
		t.Fatalf("failed to decide plan-1: %v", err)
	}
	if err := SetupPlanRepository.Upsert(db.DB, "plan-2", userId, 1, testPlanPayload); err != nil {
		t.Fatalf("failed to upsert plan-2: %v", err)
	}

	var count int
	if err := db.DB.QueryRow("SELECT COUNT(*) FROM setup_plans WHERE user_id = ? AND organization_id = 1", userId).Scan(&count); err != nil {
		t.Fatalf("count failed: %v", err)
	}
	if count != 1 {
		t.Fatalf("plans for (user, org) = %d, want 1 (upsert replaces)", count)
	}

	plan, err := SetupPlanRepository.FindLatestByUserAndOrganization(db.DB, userId, 1)
	if err != nil {
		t.Fatalf("find latest failed: %v", err)
	}
	if plan == nil || plan.Id != "plan-2" || plan.Status != "pending" {
		t.Fatalf("latest plan = %+v, want fresh pending plan-2", plan)
	}
	if plan.RequestedByEmail != "plan@example.com" {
		t.Errorf("requestedByEmail = %q", plan.RequestedByEmail)
	}
}

func TestSetupPlanDecideGuard(t *testing.T) {
	setupTestDB(t)
	setupSetupPlanTables(t)
	userId := insertSetupTestUser(t, "plan@example.com")

	if err := SetupPlanRepository.Upsert(db.DB, "plan-1", userId, 1, testPlanPayload); err != nil {
		t.Fatalf("failed to upsert: %v", err)
	}

	rows, err := SetupPlanRepository.Decide(db.DB, "plan-1", "rejected", "wrong split", "", userId, time.Now())
	if err != nil || rows != 1 {
		t.Fatalf("first decide: (%d, %v), want (1, nil)", rows, err)
	}

	rows, err = SetupPlanRepository.Decide(db.DB, "plan-1", "approved", "", "[]", userId, time.Now())
	if err != nil {
		t.Fatalf("second decide errored: %v", err)
	}
	if rows != 0 {
		t.Fatalf("second decide affected %d rows, want 0 (already decided)", rows)
	}

	plan, err := SetupPlanRepository.FindById(db.DB, "plan-1")
	if err != nil || plan == nil {
		t.Fatalf("find by id: (%v, %v)", plan, err)
	}
	if plan.Status != "rejected" || plan.RejectReason != "wrong split" {
		t.Errorf("plan = %+v, want rejected with reason preserved", plan)
	}
	if plan.DecidedAt == nil || plan.DecidedBy == nil || *plan.DecidedBy != userId {
		t.Errorf("decided metadata missing: %+v", plan)
	}
}

func TestSetupPlanPruneOld(t *testing.T) {
	setupTestDB(t)
	setupSetupPlanTables(t)
	userId := insertSetupTestUser(t, "plan@example.com")

	now := time.Now().UTC()
	insert := func(id, status string, createdAt time.Time, decidedAt *time.Time) {
		t.Helper()
		var decided any
		if decidedAt != nil {
			decided = shared.FormatAuthTime(*decidedAt)
		}
		if _, err := db.DB.Exec(
			"INSERT INTO setup_plans (id, user_id, organization_id, payload, status, created_at, decided_at) VALUES (?, ?, ?, ?, ?, ?, ?)",
			id, userId, 1, testPlanPayload, status, shared.FormatAuthTime(createdAt), decided,
		); err != nil {
			t.Fatalf("insert %s: %v", id, err)
		}
	}

	oldDecision := now.Add(-8 * 24 * time.Hour)
	freshDecision := now.Add(-time.Hour)
	insert("stale-approved", "approved", oldDecision, &oldDecision)
	insert("fresh-approved", "approved", freshDecision, &freshDecision)
	insert("stale-pending", "pending", now.Add(-25*time.Hour), nil)
	insert("fresh-pending", "pending", now.Add(-time.Hour), nil)

	pruned, err := SetupPlanRepository.PruneOld(db.DB, now)
	if err != nil {
		t.Fatalf("prune failed: %v", err)
	}
	if pruned != 2 {
		t.Fatalf("pruned = %d, want 2", pruned)
	}
	for _, id := range []string{"fresh-approved", "fresh-pending"} {
		if plan, _ := SetupPlanRepository.FindById(db.DB, id); plan == nil {
			t.Errorf("%s must survive the prune", id)
		}
	}
	for _, id := range []string{"stale-approved", "stale-pending"} {
		if plan, _ := SetupPlanRepository.FindById(db.DB, id); plan != nil {
			t.Errorf("%s should be pruned", id)
		}
	}
}
