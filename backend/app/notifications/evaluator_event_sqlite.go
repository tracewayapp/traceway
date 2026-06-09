//go:build !pgch

package notifications

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/tracewayapp/traceway/backend/app/db"
	"github.com/tracewayapp/traceway/backend/app/hooks"
	"github.com/tracewayapp/traceway/backend/app/models"
	traceway "go.tracewayapp.com"
)

func evaluateNewError(ctx context.Context, rule *models.NotificationRuleWithChannel, event hooks.ReportEvent) {
	var cfg newErrorConfig
	json.Unmarshal(rule.Config, &cfg)

	for _, hash := range event.ExceptionHashes {
		dedupKey := fmt.Sprintf("%d:%s", rule.Id, hash)
		if dedup.isDuplicate(dedupKey, time.Duration(rule.CooldownMinutes)*time.Minute) {
			continue
		}

		var count int64
		err := db.DB.QueryRowContext(ctx,
			"SELECT COUNT(*) FROM exception_stack_traces WHERE project_id = ? AND exception_hash = ?",
			event.ProjectId.String(), hash).Scan(&count)
		if err != nil {
			traceway.CaptureException(fmt.Errorf("new_error check failed: %w", err))
			continue
		}

		if count > 1 {
			var archivedCount int64
			archErr := db.DB.QueryRowContext(ctx,
				"SELECT COUNT(*) FROM archived_exceptions WHERE project_id = ? AND exception_hash = ?",
				event.ProjectId.String(), hash).Scan(&archivedCount)
			if archErr != nil || archivedCount == 0 {
				continue
			}

			var postArchiveCount int64
			archErr = db.DB.QueryRowContext(ctx,
				"SELECT COUNT(*) FROM exception_stack_traces WHERE project_id = ? AND exception_hash = ? AND recorded_at > (SELECT MAX(archived_at) FROM archived_exceptions WHERE project_id = ? AND exception_hash = ?)",
				event.ProjectId.String(), hash, event.ProjectId.String(), hash).Scan(&postArchiveCount)
			if archErr != nil {
				traceway.CaptureException(fmt.Errorf("new_error post-archive count failed: %w", archErr))
				continue
			}

			if postArchiveCount > 1 {
				continue
			}
		}

		details := getExceptionDetails(ctx, event.ProjectId, hash)

		if shouldIgnore(details.ErrorType, cfg.IgnorePatterns) {
			continue
		}

		dedup.record(dedupKey)
		projectName := getProjectName(rule.ProjectId)
		msg := buildNewErrorMessage(details, projectName)
		dispatch(rule, msg)
	}
}

func evaluateErrorRegression(ctx context.Context, rule *models.NotificationRuleWithChannel, event hooks.ReportEvent) {
	for _, hash := range event.ExceptionHashes {
		dedupKey := fmt.Sprintf("%d:%s", rule.Id, hash)
		if dedup.isDuplicate(dedupKey, time.Duration(rule.CooldownMinutes)*time.Minute) {
			continue
		}

		var count int64
		err := db.DB.QueryRowContext(ctx,
			"SELECT COUNT(*) FROM archived_exceptions WHERE project_id = ? AND exception_hash = ?",
			event.ProjectId.String(), hash).Scan(&count)
		if err != nil {
			traceway.CaptureException(fmt.Errorf("error_regression check failed: %w", err))
			continue
		}

		if count == 0 {
			continue
		}

		details := getExceptionDetails(ctx, event.ProjectId, hash)
		dedup.record(dedupKey)
		projectName := getProjectName(rule.ProjectId)
		msg := buildErrorRegressionMessage(details, projectName)
		dispatch(rule, msg)
	}
}

func getExceptionDetails(ctx context.Context, projectId uuid.UUID, hash string) ExceptionDetails {
	var idStr, stackTrace, traceType, appVersion, serverName, attributesJSON, recordedAtStr string
	var traceIdStr sql.NullString

	err := db.DB.QueryRowContext(ctx,
		"SELECT id, trace_id, trace_type, stack_trace, attributes, app_version, server_name, recorded_at FROM exception_stack_traces WHERE project_id = ? AND exception_hash = ? ORDER BY recorded_at DESC LIMIT 1",
		projectId.String(), hash).Scan(&idStr, &traceIdStr, &traceType, &stackTrace, &attributesJSON, &appVersion, &serverName, &recordedAtStr)

	details := ExceptionDetails{
		Hash: hash,
	}

	if err != nil {
		if err == sql.ErrNoRows {
			details.ErrorType = "Unknown Error"
			return details
		}
		details.ErrorType = "Unknown Error"
		return details
	}

	details.Id = idStr
	details.StackTrace = stackTrace
	details.AppVersion = appVersion
	details.ServerName = serverName
	details.RecordedAt, _ = time.Parse(time.RFC3339Nano, recordedAtStr)
	details.TraceType = traceType

	var traceId *uuid.UUID
	if traceIdStr.Valid {
		if parsed, parseErr := uuid.Parse(traceIdStr.String); parseErr == nil {
			traceId = &parsed
		}
	}
	details.TraceName = resolveTraceName(ctx, projectId, traceId, traceType)

	if attributesJSON != "" && attributesJSON != "{}" {
		attrs := make(map[string]string)
		if jsonErr := json.Unmarshal([]byte(attributesJSON), &attrs); jsonErr == nil {
			details.Attributes = attrs
		}
	}

	details.ErrorType = extractErrorType(stackTrace)
	return details
}
