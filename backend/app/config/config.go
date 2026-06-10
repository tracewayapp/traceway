package config

import "os"

type Cfg struct {
	JWTSecret string

	DBType           string
	PostgresHost     string
	PostgresPort     string
	PostgresDatabase string
	PostgresUsername  string
	PostgresPassword string
	PostgresSSLMode  string
	SQLitePath       string

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
	SessionRecordingRetentionDays   string
	SessionRecordingUploadWorkers   string
	SessionRecordingUploadQueueSize string

	SourceMapCacheMaxEntries string
	SourceMapCacheMaxBytesMB string
	SourceMapCacheType       string
	SourceMapDiskCachePath   string
	SourceMapDiskCacheMaxMB  string
	SymbolicatorParser       string

	SMTPEnabled  string
	SMTPHost     string
	SMTPPort     string
	SMTPUsername string
	SMTPPassword string
	SMTPFrom     string

	AppBaseURL            string
	CloudMode             string
	MonitoringTracewayURL string
	APIOnly               string
	Ports                 string
	TurnstileSecretKey    string

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

func LoadFromEnv() *Cfg {
	return &Cfg{
		JWTSecret: os.Getenv("JWT_SECRET"),

		DBType:           os.Getenv("DB_TYPE"),
		PostgresHost:     os.Getenv("POSTGRES_HOST"),
		PostgresPort:     os.Getenv("POSTGRES_PORT"),
		PostgresDatabase: os.Getenv("POSTGRES_DATABASE"),
		PostgresUsername:  os.Getenv("POSTGRES_USERNAME"),
		PostgresPassword: os.Getenv("POSTGRES_PASSWORD"),
		PostgresSSLMode:  os.Getenv("POSTGRES_SSLMODE"),
		SQLitePath:       os.Getenv("SQLITE_PATH"),

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
		SessionRecordingRetentionDays:   os.Getenv("SESSION_RECORDING_RETENTION_DAYS"),
		SessionRecordingUploadWorkers:   os.Getenv("SESSION_RECORDING_UPLOAD_WORKERS"),
		SessionRecordingUploadQueueSize: os.Getenv("SESSION_RECORDING_UPLOAD_QUEUE_SIZE"),

		SourceMapCacheMaxEntries: os.Getenv("SOURCEMAP_CACHE_MAX_ENTRIES"),
		SourceMapCacheMaxBytesMB: os.Getenv("SOURCEMAP_CACHE_MAX_BYTES_MB"),
		SourceMapCacheType:       os.Getenv("SOURCEMAP_CACHE_TYPE"),
		SourceMapDiskCachePath:   os.Getenv("SOURCEMAP_DISK_CACHE_PATH"),
		SourceMapDiskCacheMaxMB:  os.Getenv("SOURCEMAP_DISK_CACHE_MAX_MB"),
		SymbolicatorParser:       os.Getenv("SYMBOLICATOR_PARSER"),

		SMTPEnabled:  os.Getenv("SMTP_ENABLED"),
		SMTPHost:     os.Getenv("SMTP_HOST"),
		SMTPPort:     os.Getenv("SMTP_PORT"),
		SMTPUsername: os.Getenv("SMTP_USERNAME"),
		SMTPPassword: os.Getenv("SMTP_PASSWORD"),
		SMTPFrom:     os.Getenv("SMTP_FROM"),

		AppBaseURL:            os.Getenv("APP_BASE_URL"),
		CloudMode:             os.Getenv("CLOUD_MODE"),
		MonitoringTracewayURL: os.Getenv("MONITORING_TRACEWAY_URL"),
		APIOnly:               os.Getenv("API_ONLY"),
		Ports:                 os.Getenv("PORTS"),
		TurnstileSecretKey:    os.Getenv("TURNSTILE_SECRET_KEY"),

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
