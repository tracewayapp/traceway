package middleware

import (
	"context"
	"net/http"

	"github.com/tracewayapp/traceway/backend/app/db"

	"github.com/gin-gonic/gin"
	traceway "go.tracewayapp.com"
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

	txHandle, err := db.DB.Begin()
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

// OnCommit queues fn to run after the Transactional middleware successfully
// commits the request transaction; queued fns are dropped on rollback. Use it
// for side effects that must only fire once the transaction's writes are
// visible to other connections (e.g. waking the outbox drain worker, which
// would otherwise poll before the enqueued row exists and go back to sleep).
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
