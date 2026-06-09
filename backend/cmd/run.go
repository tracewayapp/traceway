package cmd

import (
	"context"
	"fmt"
	"io/fs"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

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
	"github.com/tracewayapp/traceway/backend/app/retention"
	"github.com/tracewayapp/traceway/backend/app/services"
	"github.com/tracewayapp/traceway/backend/app/sourcemapbackfill"
	"github.com/tracewayapp/traceway/backend/app/storage"
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

		applyOAuthEnvOverrides(cfg)
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

	notifications.StartEvaluator(ctx)
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

	if monitoringTracewayUrl := cfg.MonitoringTracewayURL; monitoringTracewayUrl != "" {
		router.Use(tracewaygin.New(
			monitoringTracewayUrl,
			tracewaygin.WithOnErrorRecording(tracewaygin.RecordingQuery|tracewaygin.RecordingBody|tracewaygin.RecordingHeader|tracewaygin.RecordingUrl),
		))
		monitoring.StartClickHouseReporter(ctx)
		monitoring.StartBackendReporter(ctx)
	}

	router.GET("/health", func(c *gin.Context) {
		c.String(200, "OK")
	})

	apiRouterGroup := router.Group("/api")
	controllers.RegisterControllers(apiRouterGroup)

	router.GET("/version", func(ctx *gin.Context) {
		ctx.JSON(200, gin.H{"version": "0.0.1"})
	})

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
				if err := router.Run(port); err != nil {
					panic(fmt.Errorf("Error starting server on port %s: %v", port, err))
				}
			}()
		}
	}

	notifySystemd()
	if err := router.Run(":" + portsList[0]); err != nil {
		panic(fmt.Errorf("Error starting server on port %s: %v", portsList[0], err))
	}
}

// applyOAuthEnvOverrides lets the dev-options path pick up OAuth/OIDC env vars
// so embedded examples (e.g. devtesting-embedded) can be configured for SSO
// testing without exposing every OIDC knob as a WithXxx option.
func applyOAuthEnvOverrides(cfg *config.Cfg) {
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
