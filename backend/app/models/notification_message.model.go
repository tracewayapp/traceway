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

	// Optional structured presentation for HTML-capable adapters (email).
	// When any of Intro/Details/CodeBlock is set, the email adapter renders
	// them instead of the plaintext Body; every other adapter keeps using
	// Body, so builders must keep Body complete on its own.
	Intro       string                      `json:",omitempty"`
	Details     []NotificationMessageDetail `json:",omitempty"`
	CodeBlock   string                      `json:",omitempty"`
	ActionLabel string                      `json:",omitempty"`

	// DedupToken is the stable identity of what fired within the rule
	// (exception hash, endpoint, task or metric name; empty for rule-level
	// conditions). Page dedup keys are built from it — never from URL, which
	// can embed a time window that changes every fire. Not persisted: dedup
	// only matters at page-open time.
	DedupToken string `json:"-"`
}

// NotificationMessageDetail is one label/value row rendered as a details
// table in HTML emails (exception id, hash, server, ...).
type NotificationMessageDetail struct {
	Label string
	Value string
}
