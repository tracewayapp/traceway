package middleware

import (
	"backend/app/pgdb"
	"database/sql"
	"net/http"

	"github.com/gin-gonic/gin"
)

const TransactionContextKey = "dbTx"

func Transactional(c *gin.Context) {
	txHandle, err := pgdb.DB.Begin()

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

	c.Set(TransactionContextKey, txHandle)

	c.Next()

	if status := c.Writer.Status(); status == http.StatusOK || status == http.StatusCreated || status == http.StatusSeeOther {
		if err := txHandle.Commit(); err != nil {
			c.AbortWithStatus(http.StatusInternalServerError)
			panic(err)
		}
	} else {
		txHandle.Rollback()
	}
}

func GetTx(c *gin.Context) *sql.Tx {
	if id, exists := c.Get(TransactionContextKey); exists {
		return id.(*sql.Tx)
	}
	return nil
}
