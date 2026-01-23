package cmd

import (
	"backend/app/cache"
	"backend/app/chdb"
	"backend/app/controllers"
	"backend/app/middleware"
	"backend/app/migrations"
	"backend/app/models"
	"backend/app/pgdb"
	"backend/app/services"
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

var PostStartupHooks []func(ctx context.Context)

func Run() {
	godotenv.Load()

	if err := services.InitJWT(); err != nil {
		panic(fmt.Errorf("failed to initialize JWT: %w", err))
	}

	err := pgdb.Init()
	if err != nil {
		panic(fmt.Errorf("error connecting to postgres: %w", err))
	}

	err = chdb.Init()
	if err != nil {
		panic(fmt.Errorf("error connecting to chdb: %w", err))
	}

	models.Init()

	err = migrations.Run()
	if err != nil {
		panic(fmt.Errorf("migrations run failed: %w", err))
	}

	ctx := context.Background()
	if err := cache.ProjectCache.Init(ctx); err != nil {
		panic(fmt.Errorf("projects cache could not be initialized: %w", err))
	}

	middleware.InitUseClientAuth()
	middleware.InitUseAppAuth()
	middleware.InitRequireWriteAccess()
	middleware.InitRequireProjectAccess()
	middleware.InitRequireAdminAccess()

	services.InitEmail()

	for _, hook := range PostStartupHooks {
		hook(ctx)
	}

	router := gin.Default()
	router.Use(gin.Recovery())

	router.GET("/health", func(c *gin.Context) {
		c.String(200, "OK")
	})

	apiRouterGroup := router.Group("/api")
	controllers.RegisterControllers(apiRouterGroup)

	router.GET("/version", func(ctx *gin.Context) {
		ctx.JSON(200, gin.H{"version": "0.0.1"})
	})

	apiOnly := os.Getenv("API_ONLY") == "true"

	if apiOnly {
		router.NoRoute(func(c *gin.Context) {
			c.JSON(404, gin.H{"error": "Not found"})
		})
	} else {
		staticFS, err := static.GetStaticFS()
		if err != nil {
			log.Printf("Warning: Could not load static files: %v", err)
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

	ports := os.Getenv("PORTS")
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
				port := ":" + portsList[i]
				log.Println("Starting server on " + port)
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

func notifySystemd() {
	sent, err := daemon.SdNotify(false, daemon.SdNotifyReady)
	if err != nil {
		log.Printf("Failed to notify systemd: %v", err)
	} else if sent {
		log.Println("Notified systemd that service is ready")
	}

	go func() {
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
