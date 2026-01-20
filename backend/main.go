package main

import (
	"backend/app/cache"
	"backend/app/chdb"
	"backend/app/controllers"
	"backend/app/middleware"
	"backend/app/migrations"
	"backend/static"
	"context"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/coreos/go-systemd/v22/daemon"
	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
)

func main() {
	// we don't actually care for the .env file existing
	// because in production we can just deploy with container variables
	// so the error is ignored
	godotenv.Load()

	appToken := os.Getenv("APP_TOKEN")
	if appToken == "" {
		// without the api token we must not start
		// this will be addressed when we add users
		panic("APP_TOKEN environment variable is not set")
	}

	err = chdb.Init()
	if err != nil {
		// if clickhouse could not be connected to there is no reason to start the backend
		// the panic here is valid
		panic(fmt.Errorf("error connecting to chdb: %w", err))
	}

	err = migrations.Run()
	if err != nil {
		// if migrations fail - that means the backend could not be started - and thus we have to panic and stop the backend
		panic(fmt.Errorf("migrations run failed: %w", err))
	}

	// if projects cannot be loaded there is no point in starting our backend
	ctx := context.Background()
	if err := cache.ProjectCache.Init(ctx); err != nil {
		panic(fmt.Errorf("projects cache could not be initialized: %w", err))
	}

	middleware.InitUseClientAuth()

	router := gin.Default()

	router.Use(gin.Recovery())

	// Health endpoint
	router.GET("/health", func(c *gin.Context) {
		c.String(200, "OK")
	})

	apiRouterGroup := router.Group("/api")
	controllers.RegisterControllers(apiRouterGroup)

	router.GET("/version", func(ctx *gin.Context) {
		ctx.JSON(200, gin.H{"version": "0.0.1"})
	})

	// Check if running in API-only mode (no static file serving)
	apiOnly := os.Getenv("API_ONLY") == "true"

	if apiOnly {
		router.NoRoute(func(c *gin.Context) {
			c.JSON(404, gin.H{"error": "Not found"})
		})
	} else {
		// Set up static file serving and SPA fallback
		staticFS, err := static.GetStaticFS()
		if err != nil {
			log.Printf("Warning: Could not load static files: %v", err)
			staticFS = nil
		}

		if staticFS != nil {
			// Serve static files
			router.StaticFS("/assets", http.FS(mustSubFS(staticFS, "assets")))
			router.StaticFS("/_app", http.FS(mustSubFS(staticFS, "_app")))

			// Serve root static files (favicon.ico, etc.)
			router.GET("/favicon.ico", serveStaticFile(staticFS, "favicon.ico"))
			router.GET("/robots.txt", serveStaticFile(staticFS, "robots.txt"))
		}

		// SPA fallback handler for all unmatched routes
		router.NoRoute(createSPAHandler(staticFS))
	}

	// the backend will run on what is in the env, by default if nothing is specified we'll run on 80 and 8082
	ports := os.Getenv("PORTS")
	if ports == "" {
		ports = "80,8082"
	}
	portsList := strings.Split(ports, ",")
	if len(portsList) == 0 {
		// if ports are bad we have to panic
		panic(fmt.Errorf("ports env variable is invalid - no ports found"))
	}

	if len(portsList) > 1 {
		for i := 1; i < len(portsList); i++ {
			if len(portsList[i]) == 0 {
				continue
			}
			go func() {
				port := ":" + portsList[i]
				log.Println("Starting server on " + port)

				if err := router.Run(port); err != nil {
					// if a port is bound or invalid - we have to panic and stop the app
					panic(fmt.Errorf("Error starting server on port %s: %v", port, err))
				}
			}()
		}
	}

	notifySystemd()
	if err := router.Run(":" + portsList[0]); err != nil {
		// if a port is bound or invalid - we have to panic and stop the app
		panic(fmt.Errorf("Error starting server on port %s: %v", portsList[0], err))
	}
}

// notifySystemd sends the ready notification and starts the watchdog goroutine
func notifySystemd() {
	// Notify systemd that the service is ready
	sent, err := daemon.SdNotify(false, daemon.SdNotifyReady)
	if err != nil {
		log.Printf("Failed to notify systemd: %v", err)
	} else if sent {
		log.Println("Notified systemd that service is ready")
	}

	// Start watchdog goroutine - notify every 15 seconds (half of WatchdogSec=30)
	go func() {
		ticker := time.NewTicker(15 * time.Second)
		defer ticker.Stop()
		for range ticker.C {
			daemon.SdNotify(false, daemon.SdNotifyWatchdog)
		}
	}()
}

// mustSubFS returns a sub-filesystem or panics
func mustSubFS(fsys fs.FS, dir string) fs.FS {
	sub, err := fs.Sub(fsys, dir)
	if err != nil {
		// Return empty FS if subdirectory doesn't exist
		return emptyFS{}
	}
	return sub
}

type emptyFS struct{}

func (emptyFS) Open(name string) (fs.File, error) {
	return nil, fs.ErrNotExist
}

// serveStaticFile returns a handler that serves a specific static file
func serveStaticFile(staticFS fs.FS, filename string) gin.HandlerFunc {
	return func(c *gin.Context) {
		data, err := fs.ReadFile(staticFS, filename)
		if err != nil {
			c.Status(404)
			return
		}

		// Determine content type
		contentType := "application/octet-stream"
		if strings.HasSuffix(filename, ".ico") {
			contentType = "image/x-icon"
		} else if strings.HasSuffix(filename, ".txt") {
			contentType = "text/plain"
		}

		c.Data(200, contentType, data)
	}
}

// createSPAHandler returns a handler for SPA fallback routing
func createSPAHandler(staticFS fs.FS) gin.HandlerFunc {
	return func(c *gin.Context) {
		path := c.Request.URL.Path
		accept := c.GetHeader("Accept")

		// API routes always return 404 JSON
		if strings.HasPrefix(path, "/api") {
			c.JSON(404, gin.H{"error": "Not found"})
			return
		}

		// If Accept header is application/json without text/html or */*, return 404 JSON
		if strings.Contains(accept, "application/json") &&
			!strings.Contains(accept, "text/html") &&
			!strings.Contains(accept, "*/*") {
			c.JSON(404, gin.H{"error": "Not found"})
			return
		}

		// If no static files embedded, return 404
		if staticFS == nil {
			c.JSON(404, gin.H{"error": "Not found"})
			return
		}

		// Try to serve the exact file first (for things like /favicon.ico that might not be registered)
		cleanPath := strings.TrimPrefix(path, "/")
		if cleanPath != "" {
			if data, err := fs.ReadFile(staticFS, cleanPath); err == nil {
				contentType := detectContentType(cleanPath)
				c.Data(200, contentType, data)
				return
			}
		}

		// Serve index.html for SPA routing
		indexData, err := fs.ReadFile(staticFS, "index.html")
		if err != nil {
			c.JSON(404, gin.H{"error": "Not found"})
			return
		}

		c.Data(200, "text/html; charset=utf-8", indexData)
	}
}

// detectContentType returns the content type based on file extension
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
