//go:build !pgch

package repositories

import (
	"context"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/tracewayapp/lit/v2"
	"github.com/tracewayapp/traceway/backend/app/db"
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

type firedNotificationRow struct {
	ProjectId   uuid.UUID  `lit:"project_id"`
	RuleId      int        `lit:"rule_id"`
	RuleType    string     `lit:"rule_type"`
	RuleName    string     `lit:"rule_name"`
	ChannelType string     `lit:"channel_type"`
	ChannelName string     `lit:"channel_name"`
	Severity    string     `lit:"severity"`
	Subject     string     `lit:"subject"`
	Body        string     `lit:"body"`
	Status      string     `lit:"status"`
	ErrorMsg    string     `lit:"error_message"`
	Endpoint    string     `lit:"endpoint"`
	URL         string     `lit:"url"`
	FiredAt     SQLiteTime `lit:"fired_at"`
}

func init() {
	models.ExtensionModelRegistrations = append(models.ExtensionModelRegistrations, func(driver lit.Driver) {
		lit.RegisterModel[firedNotificationRow](driver)
	})
}

type firedNotificationRepository struct{}

func (r *firedNotificationRepository) Insert(ctx context.Context, n FiredNotification) error {
	row := firedNotificationRow{
		ProjectId:   n.ProjectId,
		RuleId:      n.RuleId,
		RuleType:    n.RuleType,
		RuleName:    n.RuleName,
		ChannelType: n.ChannelType,
		ChannelName: n.ChannelName,
		Severity:    n.Severity,
		Subject:     n.Subject,
		Body:        n.Body,
		Status:      n.Status,
		ErrorMsg:    n.ErrorMsg,
		Endpoint:    n.Endpoint,
		URL:         n.URL,
		FiredAt:     NewSQLiteTime(n.FiredAt),
	}

	query, args, err := lit.ParseNamedQuery(db.Driver,
		`INSERT INTO fired_notifications (project_id, rule_id, rule_type, rule_name, channel_type, channel_name, severity, subject, body, status, error_message, endpoint, url, fired_at)
		VALUES (:project_id, :rule_id, :rule_type, :rule_name, :channel_type, :channel_name, :severity, :subject, :body, :status, :error_message, :endpoint, :url, :fired_at)`,
		lit.P{
			"project_id":    row.ProjectId,
			"rule_id":       row.RuleId,
			"rule_type":     row.RuleType,
			"rule_name":     row.RuleName,
			"channel_type":  row.ChannelType,
			"channel_name":  row.ChannelName,
			"severity":      row.Severity,
			"subject":       row.Subject,
			"body":          row.Body,
			"status":        row.Status,
			"error_message": row.ErrorMsg,
			"endpoint":      row.Endpoint,
			"url":           row.URL,
			"fired_at":      row.FiredAt,
		})
	if err != nil {
		return err
	}
	_, err = db.TelemetryDB.ExecContext(ctx, query, args...)
	return err
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
		args = append(args, from.UTC().Format(time.RFC3339Nano))
	}
	if to != nil {
		where += " AND fired_at <= ?"
		args = append(args, to.UTC().Format(time.RFC3339Nano))
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
		var firedAt SQLiteTime
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
		var firedAt SQLiteTime
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

var FiredNotificationRepository = firedNotificationRepository{}
