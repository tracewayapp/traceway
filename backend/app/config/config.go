package config

import (
	"os"
	"strconv"
	"strings"
	"time"
)

type Cfg struct {
	JWTSecret string

	DBType           string
	PostgresHost     string
	PostgresPort     string
	PostgresDatabase string
	PostgresUsername string
	PostgresPassword string
	PostgresSSLMode  string
	SQLitePath       string

	DuckDBMemoryLimit         string
	DuckDBThreads             string
	DuckDBCheckpointThreshold string

	ClickhouseServer   string
	ClickhouseDatabase string
	ClickhouseUsername string
	ClickhousePassword string
	ClickhouseTLS      string

	StorageType string
	StoragePath string
	S3Bucket    string
	S3Region    string
	S3AccessKey string
	S3SecretKey string
	S3Endpoint  string

	SQLiteRetentionDays             string
	DuckDBRetentionDays             string
	LogRecordsMaxRows               string
	SessionRecordingRetentionDays   string
	SessionRecordingUploadWorkers   string
	SessionRecordingUploadQueueSize string

	ProfileArchiveRaw    string
	ProfileRetentionDays string

	SourceMapCacheMaxEntries string
	SourceMapCacheMaxBytesMB string
	SourceMapCacheType       string
	SourceMapDiskCachePath   string
	SourceMapDiskCacheMaxMB  string
	SymbolicatorParser       string

	NotificationPollSeconds string
	OncallPollSeconds       string
	OutboxPollSeconds       string

	SyntheticsPollSeconds             string
	SyntheticsBrowserMode             string
	SyntheticsBrowserSandbox          string
	SyntheticsHTTPConcurrency         string
	SyntheticsBrowserConcurrency      string
	SyntheticsAllowPrivateTargets     string
	SyntheticsPlaywrightDir           string
	SyntheticsScreenshotRetentionDays string
	SyntheticsRunnerSecret            string

	HealthDeepToken string

	AllowPrivateNotificationTargets string

	TwilioAccountSID          string
	TwilioAuthToken           string
	TwilioFromNumber          string
	TwilioMessagingServiceSID string

	SMTPEnabled  string
	SMTPHost     string
	SMTPPort     string
	SMTPUsername string
	SMTPPassword string
	SMTPFrom     string

	AppBaseURL            string
	EmailPreviewEnabled   string
	CloudMode             string
	MonitoringTracewayURL string
	APIOnly               string
	Ports                 string
	TurnstileSecretKey    string
	TrustedProxies        string
	ReportMaxBodyMB       string

	GoogleClientID     string
	GoogleClientSecret string
	GitHubClientID     string
	GitHubClientSecret string
	OAuthSessionSecret string

	OIDCClientID        string
	OIDCClientSecret    string
	OIDCDiscoveryURL    string
	OIDCDisplayName     string
	OIDCAutoCreateUsers string
	OIDCOrgClaim        string
	OIDCExtraScopes     string
	OIDCRoleClaim       string
	OIDCRoleMap         string
	OIDCAuthURL         string
	OIDCTokenURL        string
	OIDCUserInfoURL     string

	DisablePasswordLogin string
}

var Config *Cfg

func Init(c *Cfg) { Config = c }

// PollSeconds parses a poll-interval value with a 5-second floor; empty or
// invalid values fall back to defaultSeconds.
func PollSeconds(value string, defaultSeconds int) time.Duration {
	seconds := defaultSeconds
	if value != "" {
		if parsed, err := strconv.Atoi(value); err == nil && parsed >= 5 {
			seconds = parsed
		}
	}
	return time.Duration(seconds) * time.Second
}

const maxConfigurableMB = 1 << 20

func SizeMB(value string, defaultMB int) int64 {
	mb := int64(defaultMB)
	if parsed, err := strconv.ParseInt(strings.TrimSpace(value), 10, 64); err == nil && parsed > 0 && parsed <= maxConfigurableMB {
		mb = parsed
	}
	return mb * 1024 * 1024
}

// PasswordLoginDisabled reports whether the instance is SSO-only
// (DISABLE_PASSWORD_LOGIN=true), which turns off password login,
// registration, and the password-reset flow.
func (c *Cfg) PasswordLoginDisabled() bool {
	return c.DisablePasswordLogin == "true"
}

// TwilioEnabled reports whether SMS sending is configured: account credentials
// plus at least one sender (a from-number or a messaging service).
func (c *Cfg) TwilioEnabled() bool {
	return c.TwilioAccountSID != "" && c.TwilioAuthToken != "" &&
		(c.TwilioFromNumber != "" || c.TwilioMessagingServiceSID != "")
}

func LoadFromEnv() *Cfg {
	return &Cfg{
		JWTSecret: os.Getenv("JWT_SECRET"),

		DBType:           os.Getenv("DB_TYPE"),
		PostgresHost:     os.Getenv("POSTGRES_HOST"),
		PostgresPort:     os.Getenv("POSTGRES_PORT"),
		PostgresDatabase: os.Getenv("POSTGRES_DATABASE"),
		PostgresUsername: os.Getenv("POSTGRES_USERNAME"),
		PostgresPassword: os.Getenv("POSTGRES_PASSWORD"),
		PostgresSSLMode:  os.Getenv("POSTGRES_SSLMODE"),
		SQLitePath:       os.Getenv("SQLITE_PATH"),

		DuckDBMemoryLimit:         os.Getenv("DUCKDB_MEMORY_LIMIT"),
		DuckDBThreads:             os.Getenv("DUCKDB_THREADS"),
		DuckDBCheckpointThreshold: os.Getenv("DUCKDB_CHECKPOINT_THRESHOLD"),

		ClickhouseServer:   os.Getenv("CLICKHOUSE_SERVER"),
		ClickhouseDatabase: os.Getenv("CLICKHOUSE_DATABASE"),
		ClickhouseUsername: os.Getenv("CLICKHOUSE_USERNAME"),
		ClickhousePassword: os.Getenv("CLICKHOUSE_PASSWORD"),
		ClickhouseTLS:      os.Getenv("CLICKHOUSE_TLS"),

		StorageType: os.Getenv("STORAGE_TYPE"),
		StoragePath: os.Getenv("STORAGE_PATH"),
		S3Bucket:    os.Getenv("S3_BUCKET"),
		S3Region:    os.Getenv("S3_REGION"),
		S3AccessKey: os.Getenv("S3_ACCESS_KEY"),
		S3SecretKey: os.Getenv("S3_SECRET_KEY"),
		S3Endpoint:  os.Getenv("S3_ENDPOINT"),

		SQLiteRetentionDays:             os.Getenv("SQLITE_RETENTION_DAYS"),
		DuckDBRetentionDays:             os.Getenv("DUCKDB_RETENTION_DAYS"),
		LogRecordsMaxRows:               os.Getenv("LOG_RECORDS_MAX_ROWS"),
		SessionRecordingRetentionDays:   os.Getenv("SESSION_RECORDING_RETENTION_DAYS"),
		SessionRecordingUploadWorkers:   os.Getenv("SESSION_RECORDING_UPLOAD_WORKERS"),
		SessionRecordingUploadQueueSize: os.Getenv("SESSION_RECORDING_UPLOAD_QUEUE_SIZE"),

		ProfileArchiveRaw:    os.Getenv("PROFILE_ARCHIVE_RAW"),
		ProfileRetentionDays: os.Getenv("PROFILE_RETENTION_DAYS"),

		SourceMapCacheMaxEntries: os.Getenv("SOURCEMAP_CACHE_MAX_ENTRIES"),
		SourceMapCacheMaxBytesMB: os.Getenv("SOURCEMAP_CACHE_MAX_BYTES_MB"),
		SourceMapCacheType:       os.Getenv("SOURCEMAP_CACHE_TYPE"),
		SourceMapDiskCachePath:   os.Getenv("SOURCEMAP_DISK_CACHE_PATH"),
		SourceMapDiskCacheMaxMB:  os.Getenv("SOURCEMAP_DISK_CACHE_MAX_MB"),
		SymbolicatorParser:       os.Getenv("SYMBOLICATOR_PARSER"),

		NotificationPollSeconds: os.Getenv("NOTIFICATION_POLL_SECONDS"),
		OncallPollSeconds:       os.Getenv("ONCALL_POLL_SECONDS"),
		OutboxPollSeconds:       os.Getenv("OUTBOX_POLL_SECONDS"),

		SyntheticsPollSeconds:             os.Getenv("SYNTHETICS_POLL_SECONDS"),
		SyntheticsBrowserMode:             os.Getenv("SYNTHETICS_BROWSER_MODE"),
		SyntheticsBrowserSandbox:          os.Getenv("SYNTHETICS_BROWSER_SANDBOX"),
		SyntheticsHTTPConcurrency:         os.Getenv("SYNTHETICS_HTTP_CONCURRENCY"),
		SyntheticsBrowserConcurrency:      os.Getenv("SYNTHETICS_BROWSER_CONCURRENCY"),
		SyntheticsAllowPrivateTargets:     os.Getenv("SYNTHETICS_ALLOW_PRIVATE_TARGETS"),
		SyntheticsPlaywrightDir:           os.Getenv("SYNTHETICS_PLAYWRIGHT_DIR"),
		SyntheticsScreenshotRetentionDays: os.Getenv("SYNTHETICS_SCREENSHOT_RETENTION_DAYS"),
		SyntheticsRunnerSecret:            os.Getenv("SYNTHETICS_RUNNER_SECRET"),

		HealthDeepToken: os.Getenv("HEALTH_DEEP_TOKEN"),

		AllowPrivateNotificationTargets: os.Getenv("ALLOW_PRIVATE_NOTIFICATION_TARGETS"),

		TwilioAccountSID:          os.Getenv("TWILIO_ACCOUNT_SID"),
		TwilioAuthToken:           os.Getenv("TWILIO_AUTH_TOKEN"),
		TwilioFromNumber:          os.Getenv("TWILIO_FROM_NUMBER"),
		TwilioMessagingServiceSID: os.Getenv("TWILIO_MESSAGING_SERVICE_SID"),

		SMTPEnabled:  os.Getenv("SMTP_ENABLED"),
		SMTPHost:     os.Getenv("SMTP_HOST"),
		SMTPPort:     os.Getenv("SMTP_PORT"),
		SMTPUsername: os.Getenv("SMTP_USERNAME"),
		SMTPPassword: os.Getenv("SMTP_PASSWORD"),
		SMTPFrom:     os.Getenv("SMTP_FROM"),

		AppBaseURL:            os.Getenv("APP_BASE_URL"),
		EmailPreviewEnabled:   os.Getenv("EMAIL_PREVIEW_ENABLED"),
		CloudMode:             os.Getenv("CLOUD_MODE"),
		MonitoringTracewayURL: os.Getenv("MONITORING_TRACEWAY_URL"),
		APIOnly:               os.Getenv("API_ONLY"),
		Ports:                 os.Getenv("PORTS"),
		TurnstileSecretKey:    os.Getenv("TURNSTILE_SECRET_KEY"),
		TrustedProxies:        os.Getenv("TRUSTED_PROXIES"),
		ReportMaxBodyMB:       os.Getenv("REPORT_MAX_BODY_MB"),

		GoogleClientID:     os.Getenv("GOOGLE_CLIENT_ID"),
		GoogleClientSecret: os.Getenv("GOOGLE_CLIENT_SECRET"),
		GitHubClientID:     os.Getenv("GITHUB_CLIENT_ID"),
		GitHubClientSecret: os.Getenv("GITHUB_CLIENT_SECRET"),
		OAuthSessionSecret: os.Getenv("OAUTH_SESSION_SECRET"),

		OIDCClientID:        os.Getenv("OIDC_CLIENT_ID"),
		OIDCClientSecret:    os.Getenv("OIDC_CLIENT_SECRET"),
		OIDCDiscoveryURL:    os.Getenv("OIDC_DISCOVERY_URL"),
		OIDCDisplayName:     os.Getenv("OIDC_DISPLAY_NAME"),
		OIDCAutoCreateUsers: os.Getenv("OIDC_AUTO_CREATE_USERS"),
		OIDCOrgClaim:        os.Getenv("OIDC_ORG_CLAIM"),
		OIDCExtraScopes:     os.Getenv("OIDC_EXTRA_SCOPES"),
		OIDCRoleClaim:       os.Getenv("OIDC_ROLE_CLAIM"),
		OIDCRoleMap:         os.Getenv("OIDC_ROLE_MAP"),
		OIDCAuthURL:         os.Getenv("OIDC_AUTH_URL"),
		OIDCTokenURL:        os.Getenv("OIDC_TOKEN_URL"),
		OIDCUserInfoURL:     os.Getenv("OIDC_USER_INFO_URL"),

		DisablePasswordLogin: os.Getenv("DISABLE_PASSWORD_LOGIN"),
	}
}
