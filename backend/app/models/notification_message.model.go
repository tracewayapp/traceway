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

	Email *NotificationEmail `json:",omitempty"`

	// GitHub is the GitHub adapter's payload: on a rule delivery it names what
	// the created issue tracks, so archiving that exception can close the issue
	// again; on a close delivery it names the issue to close. Only the GitHub
	// adapter reads it.
	GitHub *NotificationGitHub `json:",omitempty"`

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

// NotificationGitHub is carried by messages a GitHub channel can act on. Like
// the rest of NotificationMessage it is persisted in notification_outbox, so
// the shape is a wire format: add fields, never repurpose them.
type NotificationGitHub struct {
	// IssueKey is the exception hash a created issue tracks, and ProjectId the
	// project it belongs to. Empty when the firing rule is not issue-shaped:
	// nothing would ever archive that issue, so nothing is recorded for it.
	IssueKey  string `json:",omitempty"`
	ProjectId string `json:",omitempty"`
	ChannelId int    `json:",omitempty"`

	// CloseNumber makes the delivery close that issue in Owner/Repo instead of
	// creating a new one. Owner and Repo come from the recorded issue rather
	// than the channel's current config, so a repository changed since the
	// issue was opened still closes the right one.
	CloseNumber int    `json:",omitempty"`
	Owner       string `json:",omitempty"`
	Repo        string `json:",omitempty"`
}

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
