package middleware

import (
	"context"
	"net/http"

	"github.com/tracewayapp/traceway/backend/v2/app/db"

	"github.com/gin-gonic/gin"
	"github.com/tracewayapp/traceway/backend/v2/app/traceway"
)

// Transactional opens a main-DB transaction for the request and commits it
// when the handler answers 2xx/3xx, rolling back otherwise. The handler's
// response is buffered and only released once Commit() has succeeded; a
// failed commit answers 500 with an empty body instead of the 2xx the
// handler rendered for rows that never persisted. Handlers under it must not
// stream, flush or hijack.
func Transactional(c *gin.Context) {
	if !bufferRequestBody(c, maxTransactionalBodyBytes) {
		return
	}

	// WithoutCancel: the transaction takes the request span from this context,
	// but a client disconnect must not roll it back mid-handler.
	txHandle, err := db.DB.BeginTx(context.WithoutCancel(c.Request.Context()), nil)

	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("begin transaction: %w", err))
		return
	}

	buf := bufferResponse(c)

	defer func() {
		if r := recover(); r != nil {
			txHandle.Rollback()
			buf.discard()
			c.AbortWithStatus(http.StatusInternalServerError)
			panic(r)
		}
	}()

	c.Set(db.TransactionContextKey, txHandle)

	c.Request = c.Request.WithContext(context.WithValue(c.Request.Context(), db.TransactionContextKey, txHandle))

	c.Next()

	if status := c.Writer.Status(); status >= 200 && status < 400 {
		if err := txHandle.Commit(); err != nil {
			buf.discard()
			c.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("commit transaction: %w", err))
			return
		}
		// Hooks run before the response leaves: the project cache must hold
		// the row before the client can send its next request.
		runCommitHooks(c)
	} else {
		txHandle.Rollback()
	}
	buf.release()
}

const commitHooksContextKey = "txCommitHooks"

// OnCommit queues fn until Transactional successfully commits the request.
// Use it to publish a cache entry or wake the outbox worker once the data is
// durable. Transactional discards queued hooks on rollback.
func OnCommit(c *gin.Context, fn func()) {
	hooks, _ := c.Get(commitHooksContextKey)
	fns, _ := hooks.([]func())
	c.Set(commitHooksContextKey, append(fns, fn))
}

func runCommitHooks(c *gin.Context) {
	hooks, _ := c.Get(commitHooksContextKey)
	fns, _ := hooks.([]func())
	for _, fn := range fns {
		runCommitHook(c, fn)
	}
}

// The transaction is already committed, so a hook failure must neither cancel
// the remaining hooks nor turn a persisted request into a 500.
func runCommitHook(c *gin.Context, fn func()) {
	defer func() {
		if r := recover(); r != nil {
			_ = c.Error(traceway.NewStackTraceErrorf("commit hook panicked: %v", r))
		}
	}()
	fn()
}
