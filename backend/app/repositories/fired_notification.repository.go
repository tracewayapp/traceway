//go:build pgch

package repositories

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/tracewayapp/traceway/backend/app/chdb"
	"github.com/tracewayapp/traceway/backend/app/models"
)

type FiredNotification struct {
	ProjectId   uuid.UUID
	RuleId      int
	RuleType    string
	RuleName    string
	ChannelType string
	ChannelName string
	Severity    string
	Subject     string
	Body        string
	Status      string
	ErrorMsg    string
	Endpoint    string
	URL         string
	FiredAt     time.Time
}

type firedNotificationRepository struct{}

func (r *firedNotificationRepository) Insert(ctx context.Context, n FiredNotification) error {
	batch, err := chdb.Conn.PrepareBatch(
		chdb.BatchCtx(),
		"INSERT INTO fired_notifications (project_id, rule_id, rule_type, rule_name, channel_type, channel_name, severity, subject, body, status, error_message, endpoint, url, fired_at)",
	)
	if err != nil {
		return err
	}
	if err := batch.Append(
		n.ProjectId, int32(n.RuleId), n.RuleType, n.RuleName,
		n.ChannelType, n.ChannelName, n.Severity, n.Subject, n.Body,
		n.Status, n.ErrorMsg, n.Endpoint, n.URL, n.FiredAt,
	); err != nil {
		return err
	}
	return batch.Send()
}

func (r *firedNotificationRepository) FindByProject(ctx context.Context, projectId uuid.UUID, page int, pageSize int, search string, from *time.Time, to *time.Time) ([]*models.NotificationHistoryEntry, int64, error) {
	where := "WHERE project_id = ?"
	args := []interface{}{projectId}

	if search != "" {
		where += " AND (rule_name ILIKE ? OR channel_name ILIKE ? OR subject ILIKE ?)"
		pattern := "%" + search + "%"
		args = append(args, pattern, pattern, pattern)
	}
	if from != nil {
		where += " AND fired_at >= ?"
		args = append(args, *from)
	}
	if to != nil {
		where += " AND fired_at <= ?"
		args = append(args, *to)
	}

	var total uint64
	if err := chdb.Conn.QueryRow(ctx, "SELECT count() FROM fired_notifications "+where, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	query := "SELECT rule_id, rule_type, rule_name, channel_type, channel_name, severity, subject, body, status, error_message, url, fired_at FROM fired_notifications " +
		where + " ORDER BY fired_at DESC LIMIT ? OFFSET ?"
	args = append(args, pageSize, (page-1)*pageSize)

	rows, err := chdb.Conn.Query(ctx, query, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var items []*models.NotificationHistoryEntry
	for rows.Next() {
		var ruleId int32
		entry := &models.NotificationHistoryEntry{}
		if err := rows.Scan(
			&ruleId, &entry.RuleType, &entry.RuleName, &entry.ChannelType, &entry.ChannelName,
			&entry.Severity, &entry.Subject, &entry.Body, &entry.Status, &entry.ErrorMessage,
			&entry.URL, &entry.CreatedAt,
		); err != nil {
			return nil, 0, err
		}
		entry.RuleId = int(ruleId)
		items = append(items, entry)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}

	return items, int64(total), nil
}

func (r *firedNotificationRepository) FindLastFiredPerRule(ctx context.Context) (map[int]time.Time, error) {
	rows, err := chdb.Conn.Query(ctx, "SELECT rule_id, max(fired_at) FROM fired_notifications WHERE status = 'sent' GROUP BY rule_id")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make(map[int]time.Time)
	for rows.Next() {
		var ruleId int32
		var firedAt time.Time
		if err := rows.Scan(&ruleId, &firedAt); err != nil {
			return nil, err
		}
		result[int(ruleId)] = firedAt
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return result, nil
}

var FiredNotificationRepository = firedNotificationRepository{}
