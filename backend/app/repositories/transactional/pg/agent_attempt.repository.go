//go:build transactional_pg

package pg

import (
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/tracewayapp/traceway/backend/app/db"
	"github.com/tracewayapp/traceway/backend/app/models"

	"github.com/google/uuid"
	"github.com/tracewayapp/lit/v2"
)

type agentAttemptRepository struct{}

const agentAttemptColumns = "id, organization_id, project_id, repository_id, profile_id, number, kind, subject_kind, subject_ref, executor, status, resume, base_branch, fix_branch, agent, model, cost_usd, input_tokens, output_tokens, turns, claimed_by, lease_expires_at, requested_by, approved_by, error, report_key, created_at, started_at, finished_at, updated_at"

// statusList renders a status set as a SQL IN list. The statuses are
// package constants, never user input.
func statusList(statuses []string) string {
	quoted := make([]string, 0, len(statuses))
	for _, status := range statuses {
		quoted = append(quoted, "'"+status+"'")
	}
	return "(" + strings.Join(quoted, ", ") + ")"
}

func (r *agentAttemptRepository) Create(tx *sql.Tx, attempt *models.AgentAttempt) error {
	return lit.InsertExistingUuid(tx, attempt)
}

func (r *agentAttemptRepository) FindById(tx *sql.Tx, id uuid.UUID) (*models.AgentAttempt, error) {
	return lit.SelectSingleNamed[models.AgentAttempt](
		tx,
		"SELECT "+agentAttemptColumns+" FROM agent_attempts WHERE id = :id",
		lit.P{"id": id},
	)
}

// FindActiveBySubject returns the one active attempt for a subject, or nil.
func (r *agentAttemptRepository) FindActiveBySubject(tx *sql.Tx, projectId uuid.UUID, subjectKind string, subjectRef string) (*models.AgentAttempt, error) {
	return lit.SelectSingleNamed[models.AgentAttempt](
		tx,
		"SELECT "+agentAttemptColumns+" FROM agent_attempts WHERE project_id = :project_id AND subject_kind = :subject_kind AND subject_ref = :subject_ref AND status IN "+statusList(models.AttemptActiveStatuses)+" ORDER BY created_at DESC LIMIT 1",
		lit.P{"project_id": projectId, "subject_kind": subjectKind, "subject_ref": subjectRef},
	)
}

// NextNumber returns the attempt number the next attempt for a subject gets.
func (r *agentAttemptRepository) NextNumber(tx *sql.Tx, projectId uuid.UUID, subjectKind string, subjectRef string) (int, error) {
	result, err := lit.SelectSingleNamed[models.CountResult](
		tx,
		"SELECT COALESCE(MAX(number), 0) + 1 AS count FROM agent_attempts WHERE project_id = :project_id AND subject_kind = :subject_kind AND subject_ref = :subject_ref",
		lit.P{"project_id": projectId, "subject_kind": subjectKind, "subject_ref": subjectRef},
	)
	if err != nil {
		return 0, err
	}
	if result == nil {
		return 1, nil
	}
	return result.Count, nil
}

// FindBySubject lists every attempt for a subject, newest first: the
// attempts card on the issue page.
func (r *agentAttemptRepository) FindBySubject(tx *sql.Tx, projectId uuid.UUID, subjectKind string, subjectRef string) ([]*models.AgentAttempt, error) {
	return lit.SelectNamed[models.AgentAttempt](
		tx,
		"SELECT "+agentAttemptColumns+" FROM agent_attempts WHERE project_id = :project_id AND subject_kind = :subject_kind AND subject_ref = :subject_ref ORDER BY number DESC",
		lit.P{"project_id": projectId, "subject_kind": subjectKind, "subject_ref": subjectRef},
	)
}

// FindByProject pages a project's attempts newest first; an empty status
// means every status.
func (r *agentAttemptRepository) FindByProject(tx *sql.Tx, projectId uuid.UUID, status string, limit int, offset int) ([]*models.AgentAttempt, error) {
	return lit.SelectNamed[models.AgentAttempt](
		tx,
		"SELECT "+agentAttemptColumns+" FROM agent_attempts WHERE project_id = :project_id AND (:status = '' OR status = :status) ORDER BY created_at DESC LIMIT :limit OFFSET :offset",
		lit.P{"project_id": projectId, "status": status, "limit": limit, "offset": offset},
	)
}

func (r *agentAttemptRepository) CountByProject(tx *sql.Tx, projectId uuid.UUID, status string) (int, error) {
	result, err := lit.SelectSingleNamed[models.CountResult](
		tx,
		"SELECT COUNT(*) AS count FROM agent_attempts WHERE project_id = :project_id AND (:status = '' OR status = :status)",
		lit.P{"project_id": projectId, "status": status},
	)
	if err != nil {
		return 0, err
	}
	if result == nil {
		return 0, nil
	}
	return result.Count, nil
}

// CountByProjectAndStatus counts one status for the sidebar badge.
func (r *agentAttemptRepository) CountByProjectAndStatus(tx *sql.Tx, projectId uuid.UUID, status string) (int, error) {
	return r.CountByProject(tx, projectId, status)
}

// FindClaimable returns queued attempts oldest first.
func (r *agentAttemptRepository) FindClaimable(tx *sql.Tx, limit int) ([]*models.AgentAttempt, error) {
	return lit.SelectNamed[models.AgentAttempt](
		tx,
		"SELECT "+agentAttemptColumns+" FROM agent_attempts WHERE status = 'queued' ORDER BY created_at ASC LIMIT :limit",
		lit.P{"limit": limit},
	)
}

// Claim transitions one attempt queued -> claimed under a lease. The status
// guard makes the claim lose against a concurrent claimer; the caller must
// not execute when it lost.
func (r *agentAttemptRepository) Claim(tx *sql.Tx, id uuid.UUID, claimedBy string, leaseUntil time.Time, now time.Time) (bool, error) {
	return guardedStatusUpdate(
		tx,
		"UPDATE agent_attempts SET status = 'claimed', executor = :executor, claimed_by = :claimed_by, lease_expires_at = :lease_until, started_at = COALESCE(started_at, :now), updated_at = :now WHERE id = :id AND status = 'queued'",
		lit.P{"executor": claimedBy, "claimed_by": claimedBy + "/" + uuid.NewString(), "lease_until": leaseUntil.UTC(), "now": now.UTC(), "id": id},
	)
}

// RenewLease extends the lease of an attempt the caller still holds. Zero
// rows means the lease was reclaimed and handed elsewhere; the caller must
// stop working on it.
func (r *agentAttemptRepository) RenewLease(tx *sql.Tx, id uuid.UUID, claimedBy string, leaseUntil time.Time, now time.Time) (bool, error) {
	return guardedStatusUpdate(
		tx,
		"UPDATE agent_attempts SET lease_expires_at = :lease_until, updated_at = :now WHERE id = :id AND claimed_by = :claimed_by AND lease_expires_at > :now AND status IN "+statusList(models.AttemptLeasedStatuses),
		lit.P{"lease_until": leaseUntil.UTC(), "now": now.UTC(), "id": id, "claimed_by": claimedBy},
	)
}

// FindLeaseExpired lists the attempts ReclaimStale is about to return to
// the queue, so the reclaim can be recorded on each of them.
func (r *agentAttemptRepository) FindLeaseExpired(tx *sql.Tx, now time.Time) ([]*models.AgentAttempt, error) {
	return lit.SelectNamed[models.AgentAttempt](
		tx,
		"SELECT "+agentAttemptColumns+" FROM agent_attempts WHERE status IN "+statusList(models.AttemptLeasedStatuses)+" AND lease_expires_at IS NOT NULL AND lease_expires_at < :now ORDER BY lease_expires_at ASC",
		lit.P{"now": now.UTC()},
	)
}

// ReclaimStale returns attempts whose executor stopped renewing the lease to
// the queue, flagged to resume from the transcript and branch so far.
func (r *agentAttemptRepository) ReclaimStale(tx *sql.Tx, now time.Time) (int64, error) {
	query, args, err := lit.ParseNamedQuery(
		db.Driver,
		"UPDATE agent_attempts SET status = 'queued', resume = true, claimed_by = '', lease_expires_at = NULL, updated_at = :now WHERE status IN "+statusList(models.AttemptLeasedStatuses)+" AND lease_expires_at IS NOT NULL AND lease_expires_at < :now",
		lit.P{"now": now.UTC()},
	)
	if err != nil {
		return 0, err
	}
	result, err := tx.Exec(query, args...)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}

// Transition moves an attempt from any of the given statuses to another,
// guarded so a concurrent transition wins and the caller learns it lost. A
// terminal target also stamps finished_at and releases the lease.
func (r *agentAttemptRepository) Transition(tx *sql.Tx, id uuid.UUID, from []string, to string, now time.Time) (bool, error) {
	if len(from) == 0 {
		return false, fmt.Errorf("agent attempt transition to %s needs at least one source status", to)
	}
	assignments := "status = :to, updated_at = :now"
	if !models.AttemptIsActive(to) {
		assignments += ", finished_at = :now, claimed_by = '', lease_expires_at = NULL"
	}
	return guardedStatusUpdate(
		tx,
		"UPDATE agent_attempts SET "+assignments+" WHERE id = :id AND status IN "+statusList(from),
		lit.P{"to": to, "now": now.UTC(), "id": id},
	)
}

// Approve releases a pending attempt into the queue.
func (r *agentAttemptRepository) Approve(tx *sql.Tx, id uuid.UUID, approvedBy int, now time.Time) (bool, error) {
	return guardedStatusUpdate(
		tx,
		"UPDATE agent_attempts SET status = 'queued', approved_by = :approved_by, updated_at = :now WHERE id = :id AND status = 'pending_approval'",
		lit.P{"approved_by": approvedBy, "now": now.UTC(), "id": id},
	)
}

// SetResume marks whether the next executor continues an existing session
// (a message arrived, or a lease was reclaimed) instead of starting over.
func (r *agentAttemptRepository) SetResume(tx *sql.Tx, id uuid.UUID, resume bool, now time.Time) error {
	query, args, err := lit.ParseNamedQuery(
		db.Driver,
		"UPDATE agent_attempts SET resume = :resume, updated_at = :now WHERE id = :id",
		lit.P{"resume": resume, "now": now.UTC(), "id": id},
	)
	if err != nil {
		return err
	}
	return lit.UpdateNative(tx, query, args...)
}

// SetOutcome writes the columns an executor owns without touching the
// status, which the control plane owns.
func (r *agentAttemptRepository) SetOutcome(tx *sql.Tx, id uuid.UUID, outcome models.AttemptOutcome, now time.Time) error {
	query, args, err := lit.ParseNamedQuery(
		db.Driver,
		"UPDATE agent_attempts SET executor = :executor, agent = :agent, model = :model, base_branch = :base_branch, fix_branch = :fix_branch, cost_usd = :cost_usd, input_tokens = :input_tokens, output_tokens = :output_tokens, turns = :turns, report_key = :report_key, error = :error, updated_at = :now WHERE id = :id",
		lit.P{
			"executor": outcome.Executor, "agent": outcome.Agent, "model": outcome.Model,
			"base_branch": outcome.BaseBranch, "fix_branch": outcome.FixBranch,
			"cost_usd": outcome.CostUSD, "input_tokens": outcome.InputTokens, "output_tokens": outcome.OutputTokens, "turns": outcome.Turns,
			"report_key": outcome.ReportKey, "error": outcome.Error, "now": now.UTC(), "id": id,
		},
	)
	if err != nil {
		return err
	}
	return lit.UpdateNative(tx, query, args...)
}

func (r *agentAttemptRepository) CountByStatus(tx *sql.Tx) ([]*models.AgentAttemptStatusCount, error) {
	return lit.SelectNamed[models.AgentAttemptStatusCount](
		tx,
		"SELECT status, COUNT(*) AS count FROM agent_attempts GROUP BY status ORDER BY status ASC",
		lit.P{},
	)
}

// FindTerminalFinishedBefore lists attempts done long enough ago for the
// retention worker to drop their events and blobs.
func (r *agentAttemptRepository) FindTerminalFinishedBefore(tx *sql.Tx, cutoff time.Time, limit int) ([]*models.AgentAttempt, error) {
	return lit.SelectNamed[models.AgentAttempt](
		tx,
		"SELECT "+agentAttemptColumns+" FROM agent_attempts WHERE status IN "+statusList(models.AttemptTerminalStatuses)+" AND finished_at IS NOT NULL AND finished_at < :cutoff AND report_key <> '' ORDER BY finished_at ASC LIMIT :limit",
		lit.P{"cutoff": cutoff.UTC(), "limit": limit},
	)
}

// ClearReportKey records that an attempt's blobs were pruned.
func (r *agentAttemptRepository) ClearReportKey(tx *sql.Tx, id uuid.UUID, now time.Time) error {
	query, args, err := lit.ParseNamedQuery(
		db.Driver,
		"UPDATE agent_attempts SET report_key = '', updated_at = :now WHERE id = :id",
		lit.P{"now": now.UTC(), "id": id},
	)
	if err != nil {
		return err
	}
	return lit.UpdateNative(tx, query, args...)
}

var AgentAttemptRepository = agentAttemptRepository{}
