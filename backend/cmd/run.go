package cmd

import (
	"context"
	"fmt"
	"io/fs"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/tracewayapp/traceway/backend/app/backfill"
	"github.com/tracewayapp/traceway/backend/app/cache"
	"github.com/tracewayapp/traceway/backend/app/chdb"
	"github.com/tracewayapp/traceway/backend/app/config"
	"github.com/tracewayapp/traceway/backend/app/controllers"
	"github.com/tracewayapp/traceway/backend/app/db"
	"github.com/tracewayapp/traceway/backend/app/middleware"
	"github.com/tracewayapp/traceway/backend/app/migrations"
	"github.com/tracewayapp/traceway/backend/app/models"
	"github.com/tracewayapp/traceway/backend/app/monitoring"
	"github.com/tracewayapp/traceway/backend/app/notifications"
	"github.com/tracewayapp/traceway/backend/app/oncall"
	"github.com/tracewayapp/traceway/backend/app/outbox"
	"github.com/tracewayapp/traceway/backend/app/recordings"
	"github.com/tracewayapp/traceway/backend/app/retention"
	"github.com/tracewayapp/traceway/backend/app/services"
	"github.com/tracewayapp/traceway/backend/app/services/mcpmount"
	"github.com/tracewayapp/traceway/backend/app/sourcemapbackfill"
	"github.com/tracewayapp/traceway/backend/app/storage"
	"github.com/tracewayapp/traceway/backend/app/symbolicator/sourcemap/scopes"
	"github.com/tracewayapp/traceway/backend/app/synthetics"
	"github.com/tracewayapp/traceway/backend/static"

	"github.com/coreos/go-systemd/v22/daemon"
	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
	traceway "go.tracewayapp.com"
	tracewaygin "go.tracewayapp.com/tracewaygin"
)

var PostStartupHooks []func(ctx context.Context)

func Run(opts ...Option) {
	var cfg *config.Cfg
	var o *options

	if len(opts) == 0 {
		godotenv.Load()
		cfg = config.LoadFromEnv()
	} else {
		o = &options{sqlitePath: ":memory:"}
		for _, opt := range opts {
			opt(o)
		}
		port := o.port
		if port == 0 {
			port = 8082
		}
		cfg = &config.Cfg{
			JWTSecret:   "traceway-dev-secret-key-min-32-chars!",
			DBType:      "sqlite",
			SQLitePath:  o.sqlitePath,
			StorageType: "local",
			StoragePath: "./storage",
			APIOnly:     "false",
			Ports:       fmt.Sprintf("%d", port),
		}
		if o.serverURL == "" {
			o.serverURL = fmt.Sprintf("http://localhost:%d", port)
		}
		cfg.AppBaseURL = o.serverURL
		cfg.MonitoringTracewayURL = o.monitoringTracewayURL
		if o.disableLogging {
			config.LoggingEnabled = false
		}

		applyEnvOverrides(cfg)
	}
	config.Init(cfg)

	gin.SetMode(resolveGinMode(os.Getenv("GIN_MODE")))

	if err := services.InitJWT(); err != nil {
		panic(fmt.Errorf("failed to initialize JWT: %w", err))
	}

	err := db.Init()
	if err != nil {
		panic(fmt.Errorf("error connecting to database: %w", err))
	}

	err = chdb.Init()
	if err != nil {
		panic(fmt.Errorf("error connecting to chdb: %w", err))
	}

	models.Init(db.Driver)

	if err := storage.Init(); err != nil {
		panic(fmt.Errorf("failed to initialize storage: %w", err))
	}

	err = migrations.Run(cfg.DBType)
	if err != nil {
		panic(fmt.Errorf("migrations run failed: %w", err))
	}

	if err := backfill.RunDashboards(); err != nil {
		panic(fmt.Errorf("dashboards backfill failed: %w", err))
	}

	if o != nil {
		if err := seed(o); err != nil {
			panic(fmt.Errorf("seeding failed: %w", err))
		}
	}

	ctx := context.Background()
	if err := cache.ProjectCache.Init(ctx); err != nil {
		panic(fmt.Errorf("projects cache could not be initialized: %w", err))
	}
	services.InitSourceMapCache(
		parsePositiveInt(cfg.SourceMapCacheMaxEntries, 200),
		int64(parsePositiveInt(cfg.SourceMapCacheMaxBytesMB, 500))*1024*1024,
	)
	switch cfg.SourceMapCacheType {
	case "", "memory":
	case "disk":
		dir := cfg.SourceMapDiskCachePath
		if dir == "" {
			dir = "./twcache"
		}
		maxBytes := int64(parsePositiveInt(cfg.SourceMapDiskCacheMaxMB, 2048)) * 1024 * 1024
		if err := services.EnableSymbolicatorDiskCache(dir, maxBytes); err != nil {
			panic(fmt.Errorf("source map disk cache init failed: %w", err))
		}
	default:
		panic(fmt.Errorf("unknown SOURCEMAP_CACHE_TYPE: %s", cfg.SourceMapCacheType))
	}
	if cfg.SymbolicatorParser != "" {
		if err := scopes.SetParser(cfg.SymbolicatorParser); err != nil {
			panic(fmt.Errorf("symbolicator parser init failed: %w", err))
		}
	}

	middleware.InitUseClientAuth()
	middleware.InitUseAppAuth()
	middleware.InitRequireWriteAccess()
	middleware.InitRequireProjectAccess()
	middleware.InitRequireAdminAccess()
	middleware.InitRequireOrganizationAccess()
	middleware.InitUseSourceMapAuth()
	middleware.InitUseRunnerAuth()

	services.InitEmail()
	services.InitTurnstile()
	services.InitOAuth()

	for _, hook := range PostStartupHooks {
		hook(ctx)
	}

	outbox.RegisterSender(notifications.AdapterSend)
	outbox.RegisterTerminalHook(notifications.OnOutboxTerminal)
	notifications.RegisterPageOpener(oncall.OpenPageFromDispatch)
	notifications.RegisterPageResolver(oncall.AutoResolveByDedupKey)
	synthetics.RegisterNotifier(notifications.OnCheckStateChange)
	outbox.StartDrain(ctx)
	oncall.StartEscalator(ctx)
	notifications.StartEvaluator(ctx)
	synthetics.Start(ctx)
	retention.Start(ctx)
	recordings.Start(ctx)
	sourcemapbackfill.Start(ctx)

	var router *gin.Engine
	if o != nil && o.disableLogging {
		router = gin.New()
		router.Use(gin.Recovery())
	} else {
		router = gin.Default()
	}

	proxies := parseTrustedProxies(cfg.TrustedProxies)
	if err := router.SetTrustedProxies(proxies); err != nil {
		config.Logf("Invalid TRUSTED_PROXIES %q, ignoring X-Forwarded-For entirely (every per-IP rate limit now keys on your proxy's address): %v", cfg.TrustedProxies, err)
		router.SetTrustedProxies(nil)
		proxies = nil
	} else if len(proxies) == 0 {
		config.Logf("Trusted proxies: none (X-Forwarded-For ignored; client IP is the immediate peer)")
	} else {
		config.Logf("Trusted proxies: %s", strings.Join(proxies, ", "))
	}
	router.Use(warnOnUntrustedForwardedFor(proxies))

	router.Use(middleware.SecurityHeaders)

	if monitoringTracewayUrl := cfg.MonitoringTracewayURL; monitoringTracewayUrl != "" {
		twmw := tracewaygin.New(
			monitoringTracewayUrl,
			tracewaygin.WithOnErrorRecording(tracewaygin.RecordingQuery|tracewaygin.RecordingBody|tracewaygin.RecordingHeader|tracewaygin.RecordingUrl),
		)

		selfToken := monitoringTracewayUrl
		if i := strings.IndexByte(selfToken, '@'); i >= 0 {
			selfToken = selfToken[:i]
		}
		selfAuth := "Bearer " + selfToken

		router.Use(func(c *gin.Context) {
			if c.GetHeader("Authorization") == selfAuth {
				c.Next()
				return
			}
			// The runner poll long-polls for up to 25s; recording it would
			// register as 25s endpoint samples and wreck self-monitoring p95.
			if c.Request.URL.Path == "/api/runners/poll" {
				c.Next()
				return
			}
			twmw(c)
		})

		monitoring.StartClickHouseReporter(ctx)
		monitoring.StartBackendReporter(ctx)
		monitoring.StartTelemetryDBReporter(ctx)
		monitoring.StartOutboxReporter(ctx)
		monitoring.StartSyntheticsReporter(ctx)
	}

	router.GET("/health", func(c *gin.Context) {
		c.String(200, "OK")
	})

	apiRouterGroup := router.Group("/api")
	controllers.RegisterControllers(apiRouterGroup)

	router.GET("/version", func(ctx *gin.Context) {
		ctx.JSON(200, gin.H{"version": "0.0.1"})
	})

	wellKnown := []string{http.MethodGet, http.MethodOptions}
	router.Match(wellKnown, "/.well-known/oauth-authorization-server", middleware.WellKnownCors, controllers.WellKnownController.AuthorizationServer)
	router.Match(wellKnown, "/.well-known/oauth-protected-resource", middleware.WellKnownCors, controllers.WellKnownController.ProtectedResource)
	router.Match(wellKnown, "/.well-known/oauth-protected-resource"+mcpmount.Path, middleware.WellKnownCors, controllers.WellKnownController.ProtectedResourceMCP)

	mcpHandler := mcpmount.GinHandler(router, "0.0.1")
	mcpMethods := []string{http.MethodGet, http.MethodPost, http.MethodDelete, http.MethodOptions}
	router.Match(mcpMethods, mcpmount.Path, middleware.MCPCors, mcpHandler)
	router.Match(mcpMethods, mcpmount.Path+"/", middleware.MCPCors, mcpHandler)

	apiOnly := cfg.APIOnly == "true"

	if apiOnly {
		router.NoRoute(func(c *gin.Context) {
			c.JSON(404, gin.H{"error": "Not found"})
		})
	} else {
		staticFS, err := static.GetStaticFS()
		if err != nil {
			config.Logf("Warning: Could not load static files: %v", err)
			staticFS = nil
		}

		if staticFS != nil {
			router.StaticFS("/assets", http.FS(mustSubFS(staticFS, "assets")))
			router.StaticFS("/_app", http.FS(mustSubFS(staticFS, "_app")))
			router.GET("/favicon.ico", serveStaticFile(staticFS, "favicon.ico"))
			router.GET("/robots.txt", serveStaticFile(staticFS, "robots.txt"))
		}

		router.NoRoute(createSPAHandler(staticFS))
	}

	ports := cfg.Ports
	if ports == "" {
		ports = "80,8082"
	}
	portsList := strings.Split(ports, ",")
	if len(portsList) == 0 {
		panic(fmt.Errorf("ports env variable is invalid - no ports found"))
	}

	if len(portsList) > 1 {
		for i := 1; i < len(portsList); i++ {
			if len(portsList[i]) == 0 {
				continue
			}
			go func() {
				defer traceway.Recover()

				port := ":" + portsList[i]
				config.Logln("Starting server on " + port)
				serveHTTP(router, port)
			}()
		}
	}

	notifySystemd()
	serveHTTP(router, ":"+portsList[0])
}

func resolveGinMode(value string) string {
	switch strings.TrimSpace(value) {
	case gin.DebugMode:
		return gin.DebugMode
	case gin.TestMode:
		return gin.TestMode
	default:
		return gin.ReleaseMode
	}
}

var defaultTrustedProxies = []string{
	"10.0.0.0/8", "172.16.0.0/12", "192.168.0.0/16",
	"127.0.0.0/8", "fc00::/7", "::1",
}

func parseTrustedProxies(env string) []string {
	trimmed := strings.TrimSpace(env)
	if trimmed == "" {
		return defaultTrustedProxies
	}
	if strings.EqualFold(trimmed, "none") {
		return nil
	}
	var proxies []string
	for _, p := range strings.Split(env, ",") {
		if p = strings.TrimSpace(p); p != "" {
			proxies = append(proxies, p)
		}
	}
	if len(proxies) == 0 {
		return defaultTrustedProxies
	}
	return proxies
}

func warnOnUntrustedForwardedFor(proxies []string) gin.HandlerFunc {
	detect := newUntrustedForwarderDetector(proxies)
	return func(c *gin.Context) {
		if peer, first := detect(c); first {
			config.Logf("X-Forwarded-For received from %s, which is not in TRUSTED_PROXIES. The header is ignored, so per-IP rate limits and session client IPs currently see that address instead of the real client. If %s is your proxy or CDN, add its range to TRUSTED_PROXIES (keeping the private ranges you rely on).", peer, peer)
		}
		c.Next()
	}
}

func newUntrustedForwarderDetector(proxies []string) func(*gin.Context) (string, bool) {
	nets := trustedProxyNets(proxies)
	var once sync.Once
	return func(c *gin.Context) (string, bool) {
		if c.GetHeader("X-Forwarded-For") == "" && c.GetHeader("X-Real-IP") == "" {
			return "", false
		}
		peer := c.RemoteIP()
		ip := net.ParseIP(peer)
		if ip == nil || containsIP(nets, ip) {
			return "", false
		}
		first := false
		once.Do(func() { first = true })
		return peer, first
	}
}

func trustedProxyNets(proxies []string) []*net.IPNet {
	var nets []*net.IPNet
	for _, p := range proxies {
		if _, n, err := net.ParseCIDR(p); err == nil {
			nets = append(nets, n)
			continue
		}
		if ip := net.ParseIP(p); ip != nil {
			bits := 32
			if ip.To4() == nil {
				bits = 128
			}
			nets = append(nets, &net.IPNet{IP: ip, Mask: net.CIDRMask(bits, bits)})
		}
	}
	return nets
}

func containsIP(nets []*net.IPNet, ip net.IP) bool {
	for _, n := range nets {
		if n.Contains(ip) {
			return true
		}
	}
	return false
}

const serverReadTimeout = 60 * time.Second

func serveHTTP(router *gin.Engine, port string) {
	srv := &http.Server{
		Addr:              port,
		Handler:           router,
		ReadHeaderTimeout: 15 * time.Second,
		ReadTimeout:       serverReadTimeout,
		IdleTimeout:       2 * time.Minute,
	}
	if err := srv.ListenAndServe(); err != nil {
		panic(fmt.Errorf("Error starting server on port %s: %v", port, err))
	}
}

// applyEnvOverrides forwards env vars to a config the embedded Run(opts...)
// path built without LoadFromEnv; new Cfg fields belong in this table.
func applyEnvOverrides(cfg *config.Cfg) {
	for _, m := range []struct {
		envVar string
		dest   *string
	}{
		{"SMTP_ENABLED", &cfg.SMTPEnabled},
		{"SMTP_HOST", &cfg.SMTPHost},
		{"SMTP_PORT", &cfg.SMTPPort},
		{"SMTP_USERNAME", &cfg.SMTPUsername},
		{"SMTP_PASSWORD", &cfg.SMTPPassword},
		{"SMTP_FROM", &cfg.SMTPFrom},
		{"ONCALL_POLL_SECONDS", &cfg.OncallPollSeconds},
		{"OUTBOX_POLL_SECONDS", &cfg.OutboxPollSeconds},
		{"SYNTHETICS_POLL_SECONDS", &cfg.SyntheticsPollSeconds},
		{"SYNTHETICS_BROWSER_MODE", &cfg.SyntheticsBrowserMode},
		{"SYNTHETICS_BROWSER_SANDBOX", &cfg.SyntheticsBrowserSandbox},
		{"SYNTHETICS_HTTP_CONCURRENCY", &cfg.SyntheticsHTTPConcurrency},
		{"SYNTHETICS_BROWSER_CONCURRENCY", &cfg.SyntheticsBrowserConcurrency},
		{"SYNTHETICS_ALLOW_PRIVATE_TARGETS", &cfg.SyntheticsAllowPrivateTargets},
		{"SYNTHETICS_PLAYWRIGHT_DIR", &cfg.SyntheticsPlaywrightDir},
		{"SYNTHETICS_SCREENSHOT_RETENTION_DAYS", &cfg.SyntheticsScreenshotRetentionDays},
		{"SYNTHETICS_RUNNER_SECRET", &cfg.SyntheticsRunnerSecret},
		{"HEALTH_DEEP_TOKEN", &cfg.HealthDeepToken},
		{"TWILIO_ACCOUNT_SID", &cfg.TwilioAccountSID},
		{"TWILIO_AUTH_TOKEN", &cfg.TwilioAuthToken},
		{"TWILIO_FROM_NUMBER", &cfg.TwilioFromNumber},
		{"TWILIO_MESSAGING_SERVICE_SID", &cfg.TwilioMessagingServiceSID},
		{"ALLOW_PRIVATE_NOTIFICATION_TARGETS", &cfg.AllowPrivateNotificationTargets},
		{"TRUSTED_PROXIES", &cfg.TrustedProxies},
		{"REPORT_MAX_BODY_MB", &cfg.ReportMaxBodyMB},
		{"OAUTH_SESSION_SECRET", &cfg.OAuthSessionSecret},
		{"GOOGLE_CLIENT_ID", &cfg.GoogleClientID},
		{"GOOGLE_CLIENT_SECRET", &cfg.GoogleClientSecret},
		{"GITHUB_CLIENT_ID", &cfg.GitHubClientID},
		{"GITHUB_CLIENT_SECRET", &cfg.GitHubClientSecret},
		{"OIDC_CLIENT_ID", &cfg.OIDCClientID},
		{"OIDC_CLIENT_SECRET", &cfg.OIDCClientSecret},
		{"OIDC_DISCOVERY_URL", &cfg.OIDCDiscoveryURL},
		{"OIDC_DISPLAY_NAME", &cfg.OIDCDisplayName},
		{"OIDC_AUTO_CREATE_USERS", &cfg.OIDCAutoCreateUsers},
		{"OIDC_ORG_CLAIM", &cfg.OIDCOrgClaim},
		{"OIDC_EXTRA_SCOPES", &cfg.OIDCExtraScopes},
		{"OIDC_ROLE_CLAIM", &cfg.OIDCRoleClaim},
		{"OIDC_ROLE_MAP", &cfg.OIDCRoleMap},
		{"OIDC_AUTH_URL", &cfg.OIDCAuthURL},
		{"OIDC_TOKEN_URL", &cfg.OIDCTokenURL},
		{"OIDC_USER_INFO_URL", &cfg.OIDCUserInfoURL},
		{"DISABLE_PASSWORD_LOGIN", &cfg.DisablePasswordLogin},
		{"NOTIFICATION_POLL_SECONDS", &cfg.NotificationPollSeconds},
		{"SOURCEMAP_CACHE_MAX_ENTRIES", &cfg.SourceMapCacheMaxEntries},
		{"SOURCEMAP_CACHE_MAX_BYTES_MB", &cfg.SourceMapCacheMaxBytesMB},
		{"SOURCEMAP_CACHE_TYPE", &cfg.SourceMapCacheType},
		{"SOURCEMAP_DISK_CACHE_PATH", &cfg.SourceMapDiskCachePath},
		{"SOURCEMAP_DISK_CACHE_MAX_MB", &cfg.SourceMapDiskCacheMaxMB},
		{"SYMBOLICATOR_PARSER", &cfg.SymbolicatorParser},
	} {
		if v := os.Getenv(m.envVar); v != "" {
			*m.dest = v
		}
	}
}

func parsePositiveInt(s string, def int) int {
	s = strings.TrimSpace(s)
	if s == "" {
		return def
	}
	v, err := strconv.Atoi(s)
	if err != nil || v < 1 {
		return def
	}
	return v
}

func notifySystemd() {
	sent, err := daemon.SdNotify(false, daemon.SdNotifyReady)
	if err != nil {
		config.Logf("Failed to notify systemd: %v", err)
	} else if sent {
		config.Logln("Notified systemd that service is ready")
	}

	go func() {
		defer traceway.Recover()

		ticker := time.NewTicker(15 * time.Second)
		defer ticker.Stop()
		for range ticker.C {
			daemon.SdNotify(false, daemon.SdNotifyWatchdog)
		}
	}()
}

func mustSubFS(fsys fs.FS, dir string) fs.FS {
	sub, err := fs.Sub(fsys, dir)
	if err != nil {
		return emptyFS{}
	}
	return sub
}

type emptyFS struct{}

func (emptyFS) Open(name string) (fs.File, error) {
	return nil, fs.ErrNotExist
}

func serveStaticFile(staticFS fs.FS, filename string) gin.HandlerFunc {
	return func(c *gin.Context) {
		data, err := fs.ReadFile(staticFS, filename)
		if err != nil {
			c.Status(404)
			return
		}
		contentType := "application/octet-stream"
		if strings.HasSuffix(filename, ".ico") {
			contentType = "image/x-icon"
		} else if strings.HasSuffix(filename, ".txt") {
			contentType = "text/plain"
		}
		c.Data(200, contentType, data)
	}
}

func createSPAHandler(staticFS fs.FS) gin.HandlerFunc {
	return func(c *gin.Context) {
		path := c.Request.URL.Path
		accept := c.GetHeader("Accept")

		if strings.HasPrefix(path, "/api") {
			c.JSON(404, gin.H{"error": "Not found"})
			return
		}

		if strings.Contains(accept, "application/json") &&
			!strings.Contains(accept, "text/html") &&
			!strings.Contains(accept, "*/*") {
			c.JSON(404, gin.H{"error": "Not found"})
			return
		}

		if staticFS == nil {
			c.JSON(404, gin.H{"error": "Not found"})
			return
		}

		cleanPath := strings.TrimPrefix(path, "/")
		if cleanPath != "" {
			if data, err := fs.ReadFile(staticFS, cleanPath); err == nil {
				contentType := detectContentType(cleanPath)
				c.Data(200, contentType, data)
				return
			}
		}

		indexData, err := fs.ReadFile(staticFS, "index.html")
		if err != nil {
			c.JSON(404, gin.H{"error": "Not found"})
			return
		}
		c.Data(200, "text/html; charset=utf-8", indexData)
	}
}

func detectContentType(filename string) string {
	switch {
	case strings.HasSuffix(filename, ".html"):
		return "text/html; charset=utf-8"
	case strings.HasSuffix(filename, ".css"):
		return "text/css; charset=utf-8"
	case strings.HasSuffix(filename, ".js"):
		return "application/javascript; charset=utf-8"
	case strings.HasSuffix(filename, ".json"):
		return "application/json"
	case strings.HasSuffix(filename, ".png"):
		return "image/png"
	case strings.HasSuffix(filename, ".jpg"), strings.HasSuffix(filename, ".jpeg"):
		return "image/jpeg"
	case strings.HasSuffix(filename, ".svg"):
		return "image/svg+xml"
	case strings.HasSuffix(filename, ".ico"):
		return "image/x-icon"
	case strings.HasSuffix(filename, ".woff"):
		return "font/woff"
	case strings.HasSuffix(filename, ".woff2"):
		return "font/woff2"
	default:
		return "application/octet-stream"
	}
}
