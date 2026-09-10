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

	// ProjectId is the project the rule fired in, set at dispatch. Adapters
	// that answer back into the dashboard (the Slack app's Fix it button)
	// need it because the URL alone does not carry the project.
	ProjectId string `json:",omitempty"`

	Email *NotificationEmail `json:",omitempty"`

	// DedupToken is the stable identity of what fired within the rule
	// (exception hash, endpoint, task or metric name; empty for rule-level
	// conditions). Page dedup keys are built from it — never from URL, which
	// can embed a time window that changes every fire. Not persisted: dedup
	// only matters at page-open time.
	DedupToken string `json:"-"`
}

const (
	EmailTemplateNewError        = "new_error"
	EmailTemplateErrorRegression = "error_regression"
	EmailTemplateCheckDown       = "check_down"
	EmailTemplateCheckRecovered  = "check_recovered"
	EmailTemplateAlert           = "alert"
	EmailTemplateAiFlagged       = "ai_flagged"
	EmailTemplatePage            = "page"
	EmailTemplateTest            = "test"
)

type NotificationEmail struct {
	Template  string
	Exception *EmailException `json:",omitempty"`
	Check     *EmailCheck     `json:",omitempty"`
	Alert     *EmailAlert     `json:",omitempty"`
	Flagged   *EmailFlagged   `json:",omitempty"`
	Page      *EmailPage      `json:",omitempty"`
	Test      *EmailTest      `json:",omitempty"`
}

type EmailField struct {
	Label string
	Value string
}

type EmailException struct {
	ProjectName string
	ErrorType   string
	ExceptionId string
	Hash        string
	OccurredAt  string
	AppVersion  string
	ServerName  string
	TraceLabel  string
	TraceName   string
	Attributes  []EmailField `json:",omitempty"`
	StackTrace  string
}

type EmailCheck struct {
	ProjectName         string
	CheckName           string
	ConsecutiveFailures int
	LastError           string
}

type EmailAlert struct {
	ProjectName string
	Headline    string
	ScopeLabel  string
	Scope       string
	Observed    string
	Threshold   string
	WindowMins  int
}

type EmailFlagged struct {
	ProjectName    string
	ConversationId string
	UserId         string
	Terms          []string
}

type EmailPage struct {
	BodyText        string
	RuleName        string
	EventCount      int
	EscalationLevel int
}

type EmailTest struct {
	Target string
}
