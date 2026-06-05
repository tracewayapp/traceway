package main

import (
	"errors"
	"fmt"
	"math/rand/v2"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	tracewaybackend "github.com/tracewayapp/traceway/backend"
	traceway "go.tracewayapp.com"
	tracewaygin "go.tracewayapp.com/tracewaygin"
)

const (
	appPort       = 8080
	tracewayToken = "backend-dev-token"
)

func main() {
	// 1: initialize traceway backend
	go tracewaybackend.Run(
		tracewaybackend.WithPort(8082),
		tracewaybackend.WithDefaultUser("admin@localhost.com", "admin"),
		tracewaybackend.WithDefaultProject("Example App", "gin", tracewayToken),
		tracewaybackend.DisableLogging(),
	)

	// 2: initialize gin with traceway middleware
	router := gin.Default()
	router.Use(tracewaygin.New(
		tracewayToken+"@http://localhost:8082/api/report",
		tracewaygin.WithOnErrorRecording(tracewaygin.RecordingQuery|tracewaygin.RecordingBody|tracewaygin.RecordingHeader|tracewaygin.RecordingUrl),
	))

	// 3: create a simple test endpoint
	router.GET("/hello/:name", func(c *gin.Context) {
		name := c.Param("name")

		time.Sleep(time.Duration(20+rand.IntN(80)) * time.Millisecond)

		if name == "error" {
			err := errors.New("something went wrong")
			traceway.CaptureExceptionWithContext(c.Request.Context(), err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		c.JSON(http.StatusOK, gin.H{"message": "Hello, " + name + "!"})
	})

	fmt.Println()
	fmt.Println("===========================================")
	fmt.Printf("  App:        http://localhost:%d/hello/world\n", appPort)
	fmt.Printf("  Error:      http://localhost:%d/hello/error\n", appPort)
	fmt.Println("  Dashboard:  http://localhost:8082")
	fmt.Println("  Login:      admin@localhost.com / admin")
	fmt.Println("===========================================")
	fmt.Println()

	router.Run(fmt.Sprintf(":%d", appPort))
}
