package main

import (
	"context"
	"database/sql"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	_ "github.com/mattn/go-sqlite3"
	"go.opentelemetry.io/contrib/instrumentation/github.com/gin-gonic/gin/otelgin"
)

var db *sql.DB

func main() {
	initDB()

	ctx := context.Background()
	shutdownTelemetry, err := initTelemetry(ctx)
	if err != nil {
		slog.Error("telemetry init failed", "error", err.Error())
		os.Exit(1)
	}

	router := gin.Default()
	router.Use(corsMiddleware())
	router.Use(otelgin.Middleware(serviceName, otelgin.WithGinFilter(traceFilter)))
	router.Use(otelRecovery())
	router.Use(distributedTraceMiddleware())

	router.GET("/api/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	router.GET("/api/products", listProducts)
	router.GET("/api/products/:id", getProduct)

	router.GET("/api/cart", getCart)
	router.POST("/api/cart", addToCart)
	router.DELETE("/api/cart/:id", removeFromCart)

	router.POST("/api/coupon", applyCoupon)
	router.POST("/api/checkout", checkout)

	router.POST("/api/support/chat", supportChat)

	registerFrontend(router)

	srv := &http.Server{Addr: ":8090", Handler: router}
	go func() {
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			slog.Error("server failed", "error", err.Error())
			os.Exit(1)
		}
	}()

	stopCtx, stop := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
	defer stop()
	<-stopCtx.Done()

	slog.Info("shutting down, flushing telemetry")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_ = srv.Shutdown(shutdownCtx)
	shutdownTelemetry(shutdownCtx)
}
