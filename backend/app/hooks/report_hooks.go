package hooks

import "sync"

type ReportEvent struct {
	OrganizationId int
	EndpointCount  int
	ErrorCount     int
	TaskCount      int
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
