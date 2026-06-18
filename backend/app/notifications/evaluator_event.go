//go:build pgch

package notifications

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/tracewayapp/traceway/backend/app/chdb"
	"github.com/tracewayapp/traceway/backend/app/hooks"
	"github.com/tracewayapp/traceway/backend/app/models"
	traceway "go.tracewayapp.com"
)

func evaluateNewError(ctx context.Context, rule *models.NotificationRuleWithChannel, event hooks.ReportEvent) {
	var cfg newErrorConfig
	json.Unmarshal(rule.Config, &cfg)

	for hash, batchOccurrences := range countOccurrences(event.ExceptionHashes) {
		dedupKey := errorDedupKey(rule.Id, hash)
		if dedup.isDuplicate(dedupKey, time.Duration(rule.CooldownMinutes)*time.Minute) {
			continue
		}

		var count uint64
		err := chdb.Conn.QueryRow(ctx,
			"SELECT count() FROM exception_stack_traces WHERE project_id = ? AND exception_hash = ?",
			event.ProjectId, hash).Scan(&count)
		if err != nil {
			traceway.CaptureException(fmt.Errorf("new_error check failed: %w", err))
			continue
		}

		if count > uint64(batchOccurrences) {
			var archivedCount uint64
			archErr := chdb.Conn.QueryRow(ctx,
				"SELECT count() FROM archived_exceptions FINAL WHERE project_id = ? AND exception_hash = ?",
				event.ProjectId, hash).Scan(&archivedCount)
			if archErr != nil || archivedCount == 0 {
				continue
			}

			var postArchiveCount uint64
			archErr = chdb.Conn.QueryRow(ctx,
				"SELECT count() FROM exception_stack_traces WHERE project_id = ? AND exception_hash = ? AND recorded_at > (SELECT max(archived_at) FROM archived_exceptions FINAL WHERE project_id = ? AND exception_hash = ?)",
				event.ProjectId, hash, event.ProjectId, hash).Scan(&postArchiveCount)
			if archErr != nil {
				traceway.CaptureException(fmt.Errorf("new_error post-archive count failed: %w", archErr))
				continue
			}

			if postArchiveCount > uint64(batchOccurrences) {
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
	for hash := range countOccurrences(event.ExceptionHashes) {
		dedupKey := errorDedupKey(rule.Id, hash)
		if dedup.isDuplicate(dedupKey, time.Duration(rule.CooldownMinutes)*time.Minute) {
			continue
		}

		// Check if this hash was previously archived (resolved)
		var count uint64
		err := chdb.Conn.QueryRow(ctx,
			"SELECT count() FROM archived_exceptions FINAL WHERE project_id = ? AND exception_hash = ?",
			event.ProjectId, hash).Scan(&count)
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
	var id uuid.UUID
	var traceId *uuid.UUID
	var stackTrace, traceType, appVersion, serverName, attributesJSON string
	var recordedAt time.Time

	err := chdb.Conn.QueryRow(ctx,
		"SELECT id, trace_id, trace_type, stack_trace, attributes, app_version, server_name, recorded_at FROM exception_stack_traces WHERE project_id = ? AND exception_hash = ? ORDER BY recorded_at DESC LIMIT 1",
		projectId, hash).Scan(&id, &traceId, &traceType, &stackTrace, &attributesJSON, &appVersion, &serverName, &recordedAt)

	details := ExceptionDetails{
		Hash: hash,
	}

	if err != nil {
		details.ErrorType = "Unknown Error"
		return details
	}

	details.Id = id.String()
	details.StackTrace = stackTrace
	details.AppVersion = appVersion
	details.ServerName = serverName
	details.RecordedAt = recordedAt
	details.TraceType = traceType
	details.TraceName = resolveTraceName(ctx, projectId, traceId, traceType, &details.RecordedAt)

	if attributesJSON != "" && attributesJSON != "{}" {
		attrs := make(map[string]string)
		if jsonErr := json.Unmarshal([]byte(attributesJSON), &attrs); jsonErr == nil {
			details.Attributes = attrs
		}
	}

	details.ErrorType = extractErrorType(stackTrace)
	return details
}
