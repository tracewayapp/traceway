//go:build telemetry_ch

package notifications

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/tracewayapp/traceway/backend/v2/app/chdb"
	"github.com/tracewayapp/traceway/backend/v2/app/hooks"
	"github.com/tracewayapp/traceway/backend/v2/app/models"
	"github.com/tracewayapp/traceway/backend/v2/app/traceway"
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
			"SELECT count() FROM exceptions_v2 WHERE project_id = ? AND exception_hash = ?",
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
				"SELECT count() FROM exceptions_v2 WHERE project_id = ? AND exception_hash = ? AND recorded_at > (SELECT max(archived_at) FROM archived_exceptions FINAL WHERE project_id = ? AND exception_hash = ?)",
				event.ProjectId, hash, event.ProjectId, hash).Scan(&postArchiveCount)
			if archErr != nil {
				traceway.CaptureException(fmt.Errorf("new_error post-archive count failed: %w", archErr))
				continue
			}

			if postArchiveCount > uint64(batchOccurrences) {
				continue
			}
		}

		details := exceptionDetails(ctx, event, hash)

		if shouldIgnore(details.ErrorType, cfg.IgnorePatterns) {
			continue
		}

		projectName := getProjectName(ctx, rule.ProjectId)
		msg := buildNewErrorMessage(details, projectName)
		// Record before dispatch: a persistently failing dispatch retries once
		// per cooldown window, never on every ingest event.
		dedup.record(dedupKey)
		dispatch(ctx, rule, msg)
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

		details := exceptionDetails(ctx, event, hash)
		projectName := getProjectName(ctx, rule.ProjectId)
		msg := buildErrorRegressionMessage(details, projectName)
		// Record before dispatch: a persistently failing dispatch retries once
		// per cooldown window, never on every ingest event.
		dedup.record(dedupKey)
		dispatch(ctx, rule, msg)
	}
}

func getExceptionDetails(ctx context.Context, projectId uuid.UUID, hash string) ExceptionDetails {
	exception := models.ExceptionStackTrace{ProjectId: projectId, ExceptionHash: hash}
	var attributesJSON string
	err := chdb.Conn.QueryRow(ctx,
		"SELECT id, trace_id, span_id, stack_trace, attributes, app_version, server_name, recorded_at FROM exceptions_v2 WHERE project_id = ? AND exception_hash = ? ORDER BY recorded_at DESC LIMIT 1",
		projectId, hash).Scan(&exception.Id, &exception.TraceId, &exception.SpanId, &exception.StackTrace, &attributesJSON, &exception.AppVersion, &exception.ServerName, &exception.RecordedAt)
	if err != nil {
		return unknownException(hash, err)
	}
	exception.Attributes = parseStoredAttributes(attributesJSON)
	return describeException(ctx, exception)
}
