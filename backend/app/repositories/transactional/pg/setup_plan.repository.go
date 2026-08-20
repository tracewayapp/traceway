//go:build transactional_pg

package pg

import (
	"database/sql"
	"errors"
	"github.com/tracewayapp/traceway/backend/app/repositories/transactional/shared"
	"time"

	"github.com/tracewayapp/lit/v2"
	"github.com/tracewayapp/traceway/backend/app/db"
)

type setupPlanRepository struct{}

// Upsert replaces every previous plan for (user, org) with a fresh pending
// one: a new submission always starts a new review cycle.
func (r *setupPlanRepository) Upsert(ex lit.Executor, id string, userId, organizationId int, payload string) error {
	query, args, err := lit.ParseNamedQuery(
		db.Driver,
		"DELETE FROM setup_plans WHERE user_id = :user_id AND organization_id = :org_id",
		lit.P{"user_id": userId, "org_id": organizationId},
	)
	if err != nil {
		return err
	}
	if _, err = ex.Exec(query, args...); err != nil {
		return err
	}

	query, args, err = lit.ParseNamedQuery(
		db.Driver,
		`INSERT INTO setup_plans (id, user_id, organization_id, payload, status, created_at)
			VALUES (:id, :user_id, :org_id, :payload, :status, :created_at)`,
		lit.P{
			"id":         id,
			"user_id":    userId,
			"org_id":     organizationId,
			"payload":    payload,
			"status":     "pending",
			"created_at": shared.FormatAuthTime(time.Now()),
		},
	)
	if err != nil {
		return err
	}
	_, err = ex.Exec(query, args...)
	return err
}

const setupPlanColumns = `p.id, p.user_id, p.organization_id, u.email, p.payload, p.status,
			p.reject_reason, p.result, p.created_at, p.decided_at, p.decided_by`

func (r *setupPlanRepository) scanRow(row *sql.Row) (*shared.SetupPlanRow, error) {
	var (
		plan         shared.SetupPlanRow
		rejectReason sql.NullString
		result       sql.NullString
		createdAt    string
		decidedAt    sql.NullString
		decidedBy    sql.NullInt64
	)
	err := row.Scan(&plan.Id, &plan.UserId, &plan.OrganizationId, &plan.RequestedByEmail, &plan.Payload,
		&plan.Status, &rejectReason, &result, &createdAt, &decidedAt, &decidedBy)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	plan.RejectReason = rejectReason.String
	plan.Result = result.String
	if plan.CreatedAt, err = shared.ParseAuthTime(createdAt); err != nil {
		return nil, err
	}
	if plan.DecidedAt, err = shared.ParseNullableAuthTime(decidedAt); err != nil {
		return nil, err
	}
	if decidedBy.Valid {
		by := int(decidedBy.Int64)
		plan.DecidedBy = &by
	}
	return &plan, nil
}

func (r *setupPlanRepository) FindLatestByUserAndOrganization(ex lit.Executor, userId, organizationId int) (*shared.SetupPlanRow, error) {
	query, args, err := lit.ParseNamedQuery(
		db.Driver,
		`SELECT `+setupPlanColumns+`
			FROM setup_plans p
			JOIN users u ON u.id = p.user_id
			WHERE p.user_id = :user_id AND p.organization_id = :org_id
			ORDER BY p.created_at DESC
			LIMIT 1`,
		lit.P{"user_id": userId, "org_id": organizationId},
	)
	if err != nil {
		return nil, err
	}
	return r.scanRow(ex.QueryRow(query, args...))
}

func (r *setupPlanRepository) FindReviewableByOrganization(ex lit.Executor, organizationId int) (*shared.SetupPlanRow, error) {
	query, args, err := lit.ParseNamedQuery(
		db.Driver,
		`SELECT `+setupPlanColumns+`
			FROM setup_plans p
			JOIN users u ON u.id = p.user_id
			WHERE p.organization_id = :org_id
			ORDER BY CASE WHEN p.status = :pending THEN 0 ELSE 1 END, p.created_at DESC
			LIMIT 1`,
		lit.P{"org_id": organizationId, "pending": "pending"},
	)
	if err != nil {
		return nil, err
	}
	return r.scanRow(ex.QueryRow(query, args...))
}

func (r *setupPlanRepository) FindById(ex lit.Executor, id string) (*shared.SetupPlanRow, error) {
	query, args, err := lit.ParseNamedQuery(
		db.Driver,
		`SELECT `+setupPlanColumns+`
			FROM setup_plans p
			JOIN users u ON u.id = p.user_id
			WHERE p.id = :id`,
		lit.P{"id": id},
	)
	if err != nil {
		return nil, err
	}
	return r.scanRow(ex.QueryRow(query, args...))
}

// Decide transitions a pending plan to approved or rejected. The status guard
// makes concurrent decisions race-safe: the loser affects zero rows.
func (r *setupPlanRepository) Decide(ex lit.Executor, id, status, rejectReason, result string, decidedBy int, decidedAt time.Time) (int64, error) {
	query, args, err := lit.ParseNamedQuery(
		db.Driver,
		`UPDATE setup_plans
			SET status = :status, reject_reason = :reason, result = :result, decided_at = :decided_at, decided_by = :decided_by
			WHERE id = :id AND status = :pending`,
		lit.P{
			"status":     status,
			"reason":     rejectReason,
			"result":     result,
			"decided_at": shared.FormatAuthTime(decidedAt),
			"decided_by": decidedBy,
			"id":         id,
			"pending":    "pending",
		},
	)
	if err != nil {
		return 0, err
	}
	res, err := ex.Exec(query, args...)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

// PruneOld drops decided plans after 7 days and never-decided ones after 24
// hours; a setup session is ephemeral by design.
func (r *setupPlanRepository) PruneOld(ex lit.Executor, now time.Time) (int64, error) {
	query, args, err := lit.ParseNamedQuery(
		db.Driver,
		`DELETE FROM setup_plans
			WHERE (status <> :pending AND decided_at < :decided_cutoff)
			   OR (status = :pending AND created_at < :pending_cutoff)`,
		lit.P{
			"pending":        "pending",
			"decided_cutoff": shared.FormatAuthTime(now.Add(-7 * 24 * time.Hour)),
			"pending_cutoff": shared.FormatAuthTime(now.Add(-24 * time.Hour)),
		},
	)
	if err != nil {
		return 0, err
	}
	res, err := ex.Exec(query, args...)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

var SetupPlanRepository = setupPlanRepository{}
