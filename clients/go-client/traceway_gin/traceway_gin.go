package tracewaygin

import (
	"fmt"
	"time"
	"traceway"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

func wrapAndExecute(c *gin.Context) (s *string) {
	defer func() {
		if r := recover(); r != nil {
			fmt.Println("RECOVERING")
			m := traceway.FormatRWithStack(r, traceway.CaptureStack(2))
			s = &m
		}
	}()
	fmt.Println("A1")
	c.Next()
	fmt.Println("A2")
	return nil
}

func New(connectionString string, options ...func(*traceway.TracewayOptions)) gin.HandlerFunc {
	traceway.Init(connectionString, options...)

	return func(c *gin.Context) {
		fmt.Println("INTERCEPTED")
		start := time.Now()

		path := c.Request.URL.Path
		query := c.Request.URL.RawQuery
		method := c.Request.Method
		clientIP := c.ClientIP()
		transactionId := uuid.NewString()

		stackTraceFormatted := wrapAndExecute(c)

		duration := time.Since(start)

		statusCode := c.Writer.Status()
		bodySize := c.Writer.Size()

		if query != "" {
			path = path + "?" + query
		}

		transactionEndpoint := method + " " + path

		defer recover()

		fmt.Println("HERE")
		traceway.CaptureTransaction(transactionId, transactionEndpoint, duration, start, statusCode, bodySize, clientIP)

		fmt.Println("HERE2")
		if stackTraceFormatted != nil {
			fmt.Println("HERE3")
			traceway.CaptureTransactionException(transactionId, *stackTraceFormatted)
		}
	}
}
