package hooks

import (
	"sync"

	"github.com/google/uuid"
	"github.com/tracewayapp/traceway/backend/v2/app/models"
)

type AiTraceInfo struct {
	TraceName      string
	TotalCost      float64
	ConversationId string
	UserId         string
	Flagged        bool
	FlaggedTerms   []string
}

type ReportEvent struct {
	OrganizationId  int
	ProjectId       uuid.UUID
	EndpointCount   int
	ErrorCount      int
	TaskCount       int
	RecordingCount  int
	TelemetryBytes  int
	RecordingBytes  int
	ExceptionHashes []string
	// Exceptions are the stored rows behind ExceptionHashes. Notification rules describe an exception from them:
	// a read right after the insert is not guaranteed to find the rows, and a miss sent "New error: Unknown Error"
	// with no details.
	Exceptions []models.ExceptionStackTrace
	AiTraces   []AiTraceInfo
}

var (
	reportHooks   []func(ReportEvent)
	reportHooksMu sync.RWMutex
)

func RegisterReportHook(fn func(ReportEvent)) {
	reportHooksMu.Lock()
	defer reportHooksMu.Unlock()
	reportHooks = append(reportHooks, fn)
}

func BroadcastReport(event ReportEvent) {
	reportHooksMu.RLock()
	hooks := reportHooks
	reportHooksMu.RUnlock()

	for _, hook := range hooks {
		hook(event)
	}
}

type IngestPermission struct {
	Exceptions bool
	Data       bool
	Replay     bool
}

var (
	ingestPermissionHook   func(orgId int) IngestPermission
	ingestPermissionHookMu sync.RWMutex
)

func RegisterIngestPermissionHook(fn func(orgId int) IngestPermission) {
	ingestPermissionHookMu.Lock()
	defer ingestPermissionHookMu.Unlock()
	ingestPermissionHook = fn
}

func IngestPermissionFor(orgId int) IngestPermission {
	ingestPermissionHookMu.RLock()
	hook := ingestPermissionHook
	ingestPermissionHookMu.RUnlock()

	if hook == nil {
		return IngestPermission{Exceptions: true, Data: true, Replay: true}
	}
	return hook(orgId)
}
