package middleware

import (
	"context"
	"net/http"

	"github.com/tracewayapp/traceway/backend/app/db"

	"github.com/gin-gonic/gin"
)

func Transactional(c *gin.Context) {
	if !bufferRequestBody(c, maxTransactionalBodyBytes) {
		return
	}

	txHandle, err := db.DB.Begin()

	if err != nil {
		c.AbortWithStatus(http.StatusInternalServerError)
		panic(err)
	}

	defer func() {
		if r := recover(); r != nil {
			txHandle.Rollback()
			c.AbortWithStatus(http.StatusInternalServerError)
			panic(r)
		}
	}()

	c.Set(db.TransactionContextKey, txHandle)

	c.Request = c.Request.WithContext(context.WithValue(c.Request.Context(), db.TransactionContextKey, txHandle))

	c.Next()

	if status := c.Writer.Status(); status >= 200 && status < 400 {
		if err := txHandle.Commit(); err != nil {
			c.AbortWithStatus(http.StatusInternalServerError)
			panic(err)
		}
		runCommitHooks(c)
	} else {
		txHandle.Rollback()
	}
}

const commitHooksContextKey = "txCommitHooks"

// OnCommit queues fn to run after the Transactional middleware successfully
// commits the request transaction; queued fns are dropped on rollback. Use it
// for side effects that must not observe the write before it lands: waking the
// outbox drain worker, which would otherwise poll before the enqueued row
// exists and go back to sleep, or filling a process-local cache that concurrent
// requests read (cache.ProjectCache, whose entry would outlive a rollback).
// Only routes carrying this middleware run the queued fns; a handler that
// commits its own transaction via db.ExecuteTransaction should call its side
// effect directly instead.
func OnCommit(c *gin.Context, fn func()) {
	hooks, _ := c.Get(commitHooksContextKey)
	fns, _ := hooks.([]func())
	c.Set(commitHooksContextKey, append(fns, fn))
}

func runCommitHooks(c *gin.Context) {
	hooks, _ := c.Get(commitHooksContextKey)
	fns, _ := hooks.([]func())
	for _, fn := range fns {
		fn()
	}
}
