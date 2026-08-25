//go:build telemetry_firebolt

package firebolt

import (
	"context"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/tracewayapp/traceway/backend/app/db"
	"github.com/tracewayapp/traceway/backend/app/models"
	"github.com/tracewayapp/traceway/backend/app/repositories/telemetry/shared"
	"github.com/tracewayapp/traceway/backend/app/repositories/telemetry/sqlitetypes"
)

type firedNotificationRepository struct{}

var firedNotificationColumns = []string{"project_id", "rule_id", "rule_type", "rule_name", "channel_type", "channel_name", "severity", "subject", "body", "status", "error_message", "endpoint", "fired_at", "url"}

func (r *firedNotificationRepository) Insert(ctx context.Context, n shared.FiredNotification) error {
	rows := [][]any{{
		n.ProjectId.String(),
		int64(n.RuleId),
		n.RuleType,
		n.RuleName,
		n.ChannelType,
		n.ChannelName,
		n.Severity,
		n.Subject,
		n.Body,
		n.Status,
		n.ErrorMsg,
		n.Endpoint,
		n.FiredAt.UTC(),
		n.URL,
	}}
	return insertRows(ctx, "fired_notifications", firedNotificationColumns, rows)
}

func (r *firedNotificationRepository) FindByProject(ctx context.Context, projectId uuid.UUID, page int, pageSize int, search string, from *time.Time, to *time.Time) ([]*models.NotificationHistoryEntry, int64, error) {
	where := "WHERE project_id = ?"
	args := []interface{}{projectId.String()}

	if search != "" {
		where += " AND (LOWER(rule_name) LIKE ? OR LOWER(channel_name) LIKE ? OR LOWER(subject) LIKE ?)"
		pattern := "%" + strings.ToLower(search) + "%"
		args = append(args, pattern, pattern, pattern)
	}
	if from != nil {
		where += " AND fired_at >= ?"
		args = append(args, from.UTC())
	}
	if to != nil {
		where += " AND fired_at <= ?"
		args = append(args, to.UTC())
	}

	var total int64
	if err := db.TelemetryDB.QueryRowContext(ctx, "SELECT COUNT(*) FROM fired_notifications "+where, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	query := "SELECT rule_id, rule_type, rule_name, channel_type, channel_name, severity, subject, body, status, error_message, url, fired_at FROM fired_notifications " +
		where + " ORDER BY fired_at DESC LIMIT ? OFFSET ?"
	args = append(args, pageSize, (page-1)*pageSize)

	rows, err := db.TelemetryDB.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var items []*models.NotificationHistoryEntry
	for rows.Next() {
		entry := &models.NotificationHistoryEntry{}
		var firedAt sqlitetypes.SQLiteTime
		if err := rows.Scan(
			&entry.RuleId, &entry.RuleType, &entry.RuleName, &entry.ChannelType, &entry.ChannelName,
			&entry.Severity, &entry.Subject, &entry.Body, &entry.Status, &entry.ErrorMessage,
			&entry.URL, &firedAt,
		); err != nil {
			return nil, 0, err
		}
		entry.CreatedAt = firedAt.Time
		items = append(items, entry)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}

	return items, total, nil
}

func (r *firedNotificationRepository) FindLastFiredPerRule(ctx context.Context) (map[int]time.Time, error) {
	rows, err := db.TelemetryDB.QueryContext(ctx, "SELECT rule_id, MAX(fired_at) FROM fired_notifications WHERE status = 'sent' GROUP BY rule_id")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make(map[int]time.Time)
	for rows.Next() {
		var ruleId int
		var firedAt sqlitetypes.SQLiteTime
		if err := rows.Scan(&ruleId, &firedAt); err != nil {
			return nil, err
		}
		result[ruleId] = firedAt.Time
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return result, nil
}

var FiredNotificationRepository = &firedNotificationRepository{}
