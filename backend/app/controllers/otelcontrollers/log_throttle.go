package otelcontrollers

import (
	"time"

	"github.com/google/uuid"
	"github.com/tracewayapp/traceway/backend/v2/app/clientlogs"
	"github.com/tracewayapp/traceway/backend/v2/app/models"
	"github.com/tracewayapp/traceway/backend/v2/app/monitoring"
	collogspb "go.opentelemetry.io/proto/otlp/collector/logs/v1"
	logspb "go.opentelemetry.io/proto/otlp/logs/v1"
)

func throttleLogs(project *models.Project, projectID uuid.UUID, ip string, req *collogspb.ExportLogsServiceRequest) (int, *models.ExceptionStackTrace) {
	count := 0
	var priorityCounts [3]int
	for _, resource := range req.ResourceLogs {
		for _, scope := range resource.ScopeLogs {
			count += len(scope.LogRecords)
			for _, record := range scope.LogRecords {
				priorityCounts[logPriority(record)]++
			}
		}
	}
	monitoring.RecordLogsReceived(count)
	if !clientlogs.IsClient(project) {
		return 0, nil
	}
	limiter := clientlogs.Default()
	accepted, flag, saturated := limiter.Allow(projectID, ip, count, time.Now())
	dropped := count - accepted
	monitoring.RecordLogsDropped(dropped, saturated)
	if dropped == 0 {
		return 0, nil
	}
	var budgets [3]int
	for i, count := range priorityCounts {
		budgets[i] = min(accepted, count)
		accepted -= budgets[i]
	}
	var review *models.ExceptionStackTrace
	for _, resource := range req.ResourceLogs {
		for _, scope := range resource.ScopeLogs {
			kept := scope.LogRecords[:0]
			for _, record := range scope.LogRecords {
				priority := logPriority(record)
				if budgets[priority] > 0 {
					budgets[priority]--
					kept = append(kept, record)
					continue
				}
				if flag && review == nil {
					attrs := extractAttributes(resource.GetResource().GetAttributes())
					if attrs == nil {
						attrs = map[string]string{}
					}
					for k, v := range extractAttributes(record.GetAttributes()) {
						attrs[k] = v
					}
					row := limiter.Review(projectID, ip, dropped, sessionIdFrom(record.GetAttributes(), resource.GetResource().GetAttributes()), attrs)
					review = &row
				}
			}
			// Clear the tail so rejected payloads can be collected before storage finishes.
			clear(scope.LogRecords[len(kept):])
			scope.LogRecords = kept
		}
	}
	return dropped, review
}

// Lifecycle and exception events share OTLP's log transport. Preserve them before ordinary logs when a batch exceeds its budget.
func logPriority(record *logspb.LogRecord) int {
	name := record.GetEventName()
	if name == "" {
		name = getStringAttribute(record.GetAttributes(), attrEventName)
	}
	switch name {
	case eventSessionStart, eventSessionEnd, eventSessionUpdate:
		return 0
	case eventException:
		return 1
	default:
		return 2
	}
}
