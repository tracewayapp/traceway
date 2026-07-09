//go:build transactional_pg

package pg

import (
	"database/sql"
	"errors"
	"github.com/tracewayapp/traceway/backend/app/repositories/transactional/shared"
	"strings"
	"time"

	"github.com/tracewayapp/lit/v2"
	"github.com/tracewayapp/traceway/backend/app/db"
	"github.com/tracewayapp/traceway/backend/app/models"
)

type deviceAuthorizationRepository struct{}

func (r *deviceAuthorizationRepository) Create(ex lit.Executor, deviceCode, userCode, clientId string, intervalSeconds int, expiresAt time.Time) error {
	query, args, err := lit.ParseNamedQuery(
		db.Driver,
		`INSERT INTO device_authorizations (device_code_hash, user_code, client_id, status, interval_seconds, expires_at, created_at)
			VALUES (:hash, :user_code, :client_id, 'pending', :interval, :expires_at, :created_at)`,
		lit.P{
			"hash":       shared.HashAuthToken(deviceCode),
			"user_code":  userCode,
			"client_id":  clientId,
			"interval":   intervalSeconds,
			"expires_at": shared.FormatAuthTime(expiresAt),
			"created_at": shared.FormatAuthTime(time.Now()),
		},
	)
	if err != nil {
		return err
	}
	_, err = ex.Exec(query, args...)
	return err
}

func (r *deviceAuthorizationRepository) FindByDeviceCode(ex lit.Executor, deviceCode string) (*models.DeviceAuthorization, error) {
	return r.findOne(ex, "device_code_hash = :v", shared.HashAuthToken(deviceCode))
}

func (r *deviceAuthorizationRepository) FindByUserCode(ex lit.Executor, userCode string) (*models.DeviceAuthorization, error) {
	// Codes are stored uppercase (generated from an uppercase alphabet); uppercase
	// the lookup so a lowercase-but-correct entry still matches.
	return r.findOne(ex, "user_code = :v", strings.ToUpper(userCode))
}

func (r *deviceAuthorizationRepository) findOne(ex lit.Executor, where, value string) (*models.DeviceAuthorization, error) {
	query, args, err := lit.ParseNamedQuery(
		db.Driver,
		"SELECT device_code_hash, user_code, client_id, status, user_id, interval_seconds, last_polled_at, expires_at, created_at FROM device_authorizations WHERE "+where,
		lit.P{"v": value},
	)
	if err != nil {
		return nil, err
	}

	var (
		d          models.DeviceAuthorization
		userId     sql.NullInt64
		lastPolled sql.NullString
		expiresAt  string
		createdAt  string
	)
	err = ex.QueryRow(query, args...).Scan(&d.DeviceCodeHash, &d.UserCode, &d.ClientId, &d.Status, &userId, &d.IntervalSeconds, &lastPolled, &expiresAt, &createdAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	if userId.Valid {
		uid := int(userId.Int64)
		d.UserId = &uid
	}
	if d.LastPolledAt, err = shared.ParseNullableAuthTime(lastPolled); err != nil {
		return nil, err
	}
	if d.ExpiresAt, err = shared.ParseAuthTime(expiresAt); err != nil {
		return nil, err
	}
	if d.CreatedAt, err = shared.ParseAuthTime(createdAt); err != nil {
		return nil, err
	}
	return &d, nil
}

func (r *deviceAuthorizationRepository) Approve(ex lit.Executor, userCode string, userId int) (int64, error) {
	return r.updateStatus(ex, userCode, "approved", &userId)
}

func (r *deviceAuthorizationRepository) Deny(ex lit.Executor, userCode string) (int64, error) {
	return r.updateStatus(ex, userCode, "denied", nil)
}

func (r *deviceAuthorizationRepository) updateStatus(ex lit.Executor, userCode, status string, userId *int) (int64, error) {
	var uid any
	if userId != nil {
		uid = *userId
	}
	query, args, err := lit.ParseNamedQuery(
		db.Driver,
		"UPDATE device_authorizations SET status = :status, user_id = :user_id WHERE user_code = :user_code AND status = 'pending'",
		lit.P{"status": status, "user_id": uid, "user_code": strings.ToUpper(userCode)},
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

func (r *deviceAuthorizationRepository) MarkPolled(ex lit.Executor, deviceCode string, polledAt time.Time, intervalSeconds int) error {
	query, args, err := lit.ParseNamedQuery(
		db.Driver,
		"UPDATE device_authorizations SET last_polled_at = :polled, interval_seconds = :interval WHERE device_code_hash = :hash",
		lit.P{"polled": shared.FormatAuthTime(polledAt), "interval": intervalSeconds, "hash": shared.HashAuthToken(deviceCode)},
	)
	if err != nil {
		return err
	}
	_, err = ex.Exec(query, args...)
	return err
}

func (r *deviceAuthorizationRepository) Delete(ex lit.Executor, deviceCode string) (int64, error) {
	query, args, err := lit.ParseNamedQuery(
		db.Driver,
		"DELETE FROM device_authorizations WHERE device_code_hash = :hash",
		lit.P{"hash": shared.HashAuthToken(deviceCode)},
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

func (r *deviceAuthorizationRepository) PruneExpired(ex lit.Executor, now time.Time) (int64, error) {
	query, args, err := lit.ParseNamedQuery(
		db.Driver,
		"DELETE FROM device_authorizations WHERE expires_at < :cutoff",
		lit.P{"cutoff": shared.FormatAuthTime(now)},
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

var DeviceAuthorizationRepository = deviceAuthorizationRepository{}
