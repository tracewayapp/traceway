package hooks

import (
	"sync"

	"github.com/google/uuid"
)

type AiTraceInfo struct {
	TraceName string
	TotalCost float64
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
	AiTraces        []AiTraceInfo
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
