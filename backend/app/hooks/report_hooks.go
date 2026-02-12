package hooks

import "sync"

type ReportEvent struct {
	OrganizationId int
	EndpointCount  int
	ErrorCount     int
	TaskCount      int
	RecordingCount int
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

type CanReportHook func(orgId int) bool

var (
	canReportHook   CanReportHook
	canReportHookMu sync.RWMutex
)

func RegisterCanReportHook(fn CanReportHook) {
	canReportHookMu.Lock()
	defer canReportHookMu.Unlock()
	canReportHook = fn
}

func CanReport(orgId int) bool {
	canReportHookMu.RLock()
	hook := canReportHook
	canReportHookMu.RUnlock()

	if hook == nil {
		return true
	}
	return hook(orgId)
}
