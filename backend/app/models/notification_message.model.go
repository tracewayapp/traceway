package models

type NotificationSeverity string

const (
	NotificationSeverityInfo     NotificationSeverity = "info"
	NotificationSeverityWarning  NotificationSeverity = "warning"
	NotificationSeverityCritical NotificationSeverity = "critical"
)

// NotificationMessage is what adapters deliver. Field names are the persisted
// JSON shape of notification_outbox.message, so renaming a field is a
// wire-format change for rows in flight across a restart or upgrade.
type NotificationMessage struct {
	Subject  string
	Body     string
	HTMLBody string
	Severity NotificationSeverity
	RuleType string
	RuleName string
	URL      string
	Endpoint string

	// DedupToken is the stable identity of what fired within the rule
	// (exception hash, endpoint, task or metric name; empty for rule-level
	// conditions). Page dedup keys are built from it — never from URL, which
	// can embed a time window that changes every fire. Not persisted: dedup
	// only matters at page-open time.
	DedupToken string `json:"-"`
}
