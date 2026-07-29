package cmd

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"net/http"
	_ "net/http/pprof"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
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
	"github.com/tracewayapp/traceway/backend/app/recordings"
	"github.com/tracewayapp/traceway/backend/app/repositories/telemetry"
	"github.com/tracewayapp/traceway/backend/app/retention"
	"github.com/tracewayapp/traceway/backend/app/services"
	"github.com/tracewayapp/traceway/backend/app/services/mcpmount"
	"github.com/tracewayapp/traceway/backend/app/sourcemapbackfill"
	"github.com/tracewayapp/traceway/backend/app/storage"
	"github.com/tracewayapp/traceway/backend/app/symbolicator/sourcemap/scopes"
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
		gin.SetMode(gin.ReleaseMode)
		if o.disableLogging {
			config.LoggingEnabled = false
		}

		applyEnvOverrides(cfg)
	}
	config.Init(cfg)

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
	middleware.InitUseSourceMapAuth()

	services.InitEmail()
	services.InitTurnstile()
	services.InitOAuth()

	for _, hook := range PostStartupHooks {
		hook(ctx)
	}

	telemetry.StartWriters(ctx)
	notifications.StartEvaluator(ctx)
	retention.Start(ctx)
	recordings.Start(ctx)
	sourcemapbackfill.Start(ctx)

	// Opt-in pprof for profiling ingest under load. Localhost only; gin does
	// not use http.DefaultServeMux, so the pprof handlers are unreachable
	// through the public router.
	if pprofPort := strings.TrimSpace(cfg.PprofPort); pprofPort != "" {
		go func() {
			defer traceway.Recover()
			if err := http.ListenAndServe("127.0.0.1:"+pprofPort, nil); err != nil {
				traceway.CaptureException(fmt.Errorf("pprof server on port %s: %w", pprofPort, err))
			}
		}()
	}

	var router *gin.Engine
	if o != nil && o.disableLogging {
		router = gin.New()
		router.Use(gin.Recovery())
	} else {
		router = gin.Default()
	}

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
			twmw(c)
		})

		monitoring.StartClickHouseReporter(ctx)
		monitoring.StartBackendReporter(ctx)
		monitoring.StartTelemetryDBReporter(ctx)
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

	// Explicit http.Servers instead of router.Run so SIGTERM/SIGINT can
	// drain in-flight requests and then flush the telemetry write queues —
	// otherwise every restart drops the acked rows sitting in the DuckDB
	// write queues. Extra-port failures stay non-fatal; a primary-port
	// failure still aborts startup.
	var servers []*http.Server
	primaryErr := make(chan error, 1)
	startServer := func(port string, primary bool) {
		srv := &http.Server{Addr: ":" + port, Handler: router.Handler()}
		servers = append(servers, srv)
		go func() {
			defer traceway.Recover()
			config.Logln("Starting server on :" + port)
			err := srv.ListenAndServe()
			if err == nil || errors.Is(err, http.ErrServerClosed) {
				return
			}
			err = fmt.Errorf("error starting server on port %s: %w", port, err)
			if primary {
				primaryErr <- err
			} else {
				traceway.CaptureException(err)
			}
		}()
	}
	for i := 1; i < len(portsList); i++ {
		if len(portsList[i]) == 0 {
			continue
		}
		startServer(portsList[i], false)
	}

	notifySystemd()
	startServer(portsList[0], true)

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)
	select {
	case err := <-primaryErr:
		panic(err)
	case sig := <-quit:
		config.Logln("Received " + sig.String() + ", shutting down")
	}

	// Bounded so a long-lived streaming connection (MCP) can't stall the
	// shutdown; the writers still get their flush after ctx expiry.
	shutdownCtx, cancelShutdown := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancelShutdown()
	for _, srv := range servers {
		if err := srv.Shutdown(shutdownCtx); err != nil {
			traceway.CaptureException(fmt.Errorf("http server shutdown on %s: %w", srv.Addr, err))
		}
	}
	telemetry.StopWriters()
}

func applyEnvOverrides(cfg *config.Cfg) {
	for _, m := range []struct {
		envVar string
		dest   *string
	}{
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
