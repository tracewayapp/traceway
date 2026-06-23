package main

import (
	"database/sql"
	"net/http"
	"os"

	tracewaygin "go.tracewayapp.com/tracewaygin"

	"github.com/gin-gonic/gin"
	_ "github.com/mattn/go-sqlite3"
)

var db *sql.DB

func main() {
	initDB()

	endpoint := os.Getenv("TRACEWAY_ENDPOINT")
	if endpoint == "" {
		endpoint = "default_token_change_me@http://localhost:8082/api/report"
	}

	router := gin.Default()
	router.Use(corsMiddleware())
	router.Use(tracewaygin.New(
		endpoint,
		tracewaygin.WithDebug(true),
		tracewaygin.WithServerName("shop-demo"),
		tracewaygin.WithVersion("0.1.0"),
		tracewaygin.WithOnErrorRecording(tracewaygin.RecordingUrl|tracewaygin.RecordingQuery|tracewaygin.RecordingHeader|tracewaygin.RecordingBody),
	))

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

	registerFrontend(router)

	router.Run(":8090")
}

func corsMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("Access-Control-Allow-Methods", "GET, POST, DELETE, OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Content-Type, Authorization")
		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		c.Next()
	}
}
