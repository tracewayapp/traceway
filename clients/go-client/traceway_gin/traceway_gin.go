package tracewaygin

import (
	"context"
	"net/http"
	"os"
	"sync"
	"time"
	"traceway"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// Cached values for default scope tags
var (
	cachedHostname    string
	cachedEnvironment string
	initOnce          sync.Once
)

func initCachedValues() {
	initOnce.Do(func() {
		hostname, err := os.Hostname()
		if err != nil {
			cachedHostname = "unknown"
		} else {
			cachedHostname = hostname
		}

		cachedEnvironment = os.Getenv("TRACEWAY_ENV")
		if cachedEnvironment == "" {
			cachedEnvironment = os.Getenv("GO_ENV")
		}
		if cachedEnvironment == "" {
			cachedEnvironment = "development"
		}
	})
}

func wrapAndExecute(c *gin.Context) (s *string) {
	defer func() {
		if r := recover(); r != nil {
			m := traceway.FormatRWithStack(r, traceway.CaptureStack(2))
			s = &m
			// we don't propagate just report
			c.AbortWithStatus(http.StatusInternalServerError)
		}
	}()
	c.Next()
	return nil
}

func New(connectionString string, options ...func(*traceway.TracewayOptions)) gin.HandlerFunc {
	traceway.Init(connectionString, options...)
	initCachedValues()

	return func(c *gin.Context) {
		start := time.Now()

		method := c.Request.Method
		clientIP := c.ClientIP()

		// Create transaction context
		txn := &traceway.TransactionContext{
			Id: uuid.NewString(),
		}

		// Create request-scoped scope with defaults
		scope := traceway.NewScope()

		// Store scope and transaction in both gin.Context and request context
		ctx := context.WithValue(c.Request.Context(), string(traceway.CtxScopeKey), scope)
		ctx = context.WithValue(ctx, string(traceway.CtxTransactionKey), txn)
		c.Request = c.Request.WithContext(ctx)
		c.Set(string(traceway.CtxScopeKey), scope)
		c.Set(string(traceway.CtxTransactionKey), txn)

		stackTraceFormatted := wrapAndExecute(c)

		duration := time.Since(start)

		statusCode := c.Writer.Status()
		bodySize := c.Writer.Size()

		if bodySize < 0 {
			bodySize = 0
		}

		// Use the registered route pattern (e.g., /users/:id) instead of actual path
		routePath := c.FullPath()
		if routePath == "" {
			// Fallback to actual path for unmatched routes
			routePath = c.Request.URL.Path
		}

		transactionEndpoint := method + " " + routePath

		defer recover()

		// Capture transaction with scope
		traceway.CaptureTransactionWithScope(txn, transactionEndpoint, duration, start, statusCode, bodySize, clientIP, scope.GetTags())

		if stackTraceFormatted != nil {
			exceptionTags := scope.GetTags()
			exceptionTags["user_agent"] = c.Request.UserAgent() // we'll only store the user agent IF an exception happens
			traceway.CaptureTransactionExceptionWithScope(txn.Id, *stackTraceFormatted, exceptionTags)
		}
	}
}
