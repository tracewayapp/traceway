package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"math/rand/v2"
	"net/http"
	"os"
	"strconv"
	"time"

	traceway "go.tracewayapp.com"
	tracewaydb "go.tracewayapp.com/tracewaydb"
	tracewaygin "go.tracewayapp.com/tracewaygin"

	"github.com/gin-gonic/gin"
	_ "github.com/mattn/go-sqlite3"
)

type CustomError struct {
	Code    int
	Message string
}

func (e *CustomError) Error() string {
	return fmt.Sprintf("CustomError[%d]: %s", e.Code, e.Message)
}

type User struct {
	Id        int    `json:"id"`
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
	Email     string `json:"email"`
}

var db *sql.DB

func initDB() {
	var err error
	db, err = sql.Open("sqlite3", ":memory:")
	if err != nil {
		panic(err)
	}
	_, err = db.Exec(`CREATE TABLE users (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		first_name TEXT NOT NULL,
		last_name TEXT NOT NULL,
		email TEXT UNIQUE NOT NULL
	)`)
	if err != nil {
		panic(err)
	}
}

func innerFunction() error {
	return traceway.NewStackTraceErrorf("error from inner function")
}

func middleFunction() error {
	return innerFunction()
}

func outerFunction() error {
	return middleFunction()
}

func main() {
	initDB()
	testGin()
}

type JsonRecordingTest struct {
	Name string
}

func testGin() {
	endpoint := os.Getenv("TRACEWAY_ENDPOINT")
	if endpoint == "" {
		endpoint = "default_token_change_me@http://localhost:8082/api/report"
	}

	router := gin.Default()

	router.Use(tracewaygin.New(
		endpoint,
		tracewaygin.WithDebug(true),
		// tracewaygin.WithRepanic(true),
		tracewaygin.WithOnErrorRecording(tracewaygin.RecordingUrl|tracewaygin.RecordingQuery|tracewaygin.RecordingHeader|tracewaygin.RecordingBody),
	))

	router.POST("/test-recording/:param", func(ctx *gin.Context) {
		var data JsonRecordingTest

		if err := ctx.ShouldBindJSON(&data); err != nil {
			ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		if data.Name != "good" {
			panic("Bad") // lol
		}
	})

	router.GET("/test-task", func(ctx *gin.Context) {
		go func() {
			traceway.MeasureTask("traceway data processor", func(twctx context.Context) {
				span := traceway.StartSpan(twctx, "loading data")
				time.Sleep(time.Second * time.Duration(rand.Float64()*2))
				span.End()

				for i := range 10 {
					traceway.CaptureMessageWithContext(twctx, "data loaded successfully "+strconv.Itoa(i))
				}

				traceway.CaptureExceptionWithContext(twctx, errors.New("what an error"))
			})
		}()
	})
	router.GET("/test-json", func(ctx *gin.Context) {
		attrs := traceway.GetAttributesFromContext(ctx)
		attrs.SetTag("json tag", veryLongJsonForTestin)
		traceway.CaptureMessageWithContext(ctx, "test json")
	})

	router.GET("/test-message", func(ctx *gin.Context) {
		for i := range 10 {
			traceway.CaptureMessageWithContext(ctx, "test message "+strconv.Itoa(i))
		}

		traceway.CaptureExceptionWithContext(ctx, errors.New("test message exception"))
	})

	router.GET("/test-50k", func(ctx *gin.Context) {
		for i := range 50_000 {
			traceway.CaptureMessage("I:" + strconv.Itoa(i))
		}
	})

	router.GET("/test-exception", func(ctx *gin.Context) {
		time.Sleep(time.Duration(rand.IntN(2000)) * time.Millisecond)
		panic("Cool")
	})

	router.GET("/test-self-report-attributes", func(ctx *gin.Context) {
		traceway.CaptureExceptionWithAttributes(errors.New("Test"), map[string]string{"Cool": "Pretty cool"}, nil)
	})

	router.GET("/test-self-report-context", func(ctx *gin.Context) {
		attrs := traceway.GetAttributesFromContext(ctx)
		attrs.SetTag("Interesting", "Pretty Cool")

		traceway.CaptureExceptionWithContext(ctx, errors.New("Test2"))
	})

	router.GET("/test-ok", func(ctx *gin.Context) {
		ctx.JSON(200, gin.H{
			"status": "ok",
		})
	})
	router.GET("/test-not-found", func(ctx *gin.Context) {
		ctx.JSON(404, gin.H{
			"status": "not-found",
		})
	})

	router.GET("/test-param/:param", func(ctx *gin.Context) {
		ctx.JSON(200, gin.H{
			"param": ctx.Param("param"),
		})
	})

	router.GET("/test-spans", func(ctx *gin.Context) {
		dbAndCacheSpan := traceway.StartSpan(ctx, "db.and.cache")

		span := traceway.StartSpan(ctx, "db.query")
		time.Sleep(time.Duration(50+rand.IntN(100)) * time.Millisecond)
		span.End()

		span = traceway.StartSpan(ctx, "cache.set")
		time.Sleep(time.Duration(10+rand.IntN(30)) * time.Millisecond)
		span.End()

		span = traceway.StartSpan(ctx, "http.external_api")
		time.Sleep(time.Duration(100+rand.IntN(200)) * time.Millisecond)
		span.End()

		dbAndCacheSpan.End()

		ctx.JSON(200, gin.H{
			"status":  "ok",
			"message": "Spans captured",
		})
	})

	router.GET("/metrics", func(ctx *gin.Context) {
		traceway.PrintCollectionFrameMetrics()
	})

	router.GET("/test-cerror-simple", func(ctx *gin.Context) {
		ctx.Error(errors.New("simple error without stack"))
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "simple error"})
	})

	router.GET("/test-cerror-wrapped", func(ctx *gin.Context) {
		base := errors.New("base error")
		wrapped := fmt.Errorf("layer 1: %w", base)
		wrapped2 := fmt.Errorf("layer 2: %w", wrapped)
		ctx.Error(wrapped2)
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "wrapped error"})
	})

	router.GET("/test-cerror-stacktrace", func(ctx *gin.Context) {
		err := traceway.NewStackTraceErrorf("error with stack trace")
		ctx.Error(err)
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "stacktrace error"})
	})

	router.GET("/test-cerror-stacktrace-wrapped", func(ctx *gin.Context) {
		base := traceway.NewStackTraceErrorf("base error with stack")
		wrapped := fmt.Errorf("wrapped with fmt: %w", base)
		ctx.Error(wrapped)
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "wrapped stacktrace error"})
	})

	router.GET("/test-cerror-multiple", func(ctx *gin.Context) {
		ctx.Error(errors.New("first error"))
		ctx.Error(traceway.NewStackTraceErrorf("second error with stack"))
		ctx.Error(fmt.Errorf("third error: %w", errors.New("nested")))
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "multiple errors"})
	})

	router.GET("/test-cerror-nested", func(ctx *gin.Context) {
		err := outerFunction()
		ctx.Error(err)
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "nested function error"})
	})

	router.GET("/test-cerror-custom", func(ctx *gin.Context) {
		err := &CustomError{Code: 500, Message: "something went wrong"}
		ctx.Error(err)
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "custom error"})
	})

	// CRUD endpoints for testing TwDB spans
	router.GET("/users", listUsers)
	router.GET("/users/:id", getUser)
	router.POST("/users", createUser)
	router.PUT("/users/:id", updateUser)
	router.DELETE("/users/:id", deleteUser)

	router.Run()
}

func listUsers(c *gin.Context) {
	twdb := tracewaydb.NewTwDB(c.Request.Context(), db)
	rows, err := twdb.QueryContext(c.Request.Context(), "SELECT id, first_name, last_name, email FROM users")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer rows.Close()

	var users []User
	for rows.Next() {
		var u User
		if err := rows.Scan(&u.Id, &u.FirstName, &u.LastName, &u.Email); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		users = append(users, u)
	}
	c.JSON(http.StatusOK, users)
}

func getUser(c *gin.Context) {
	id := c.Param("id")
	twdb := tracewaydb.NewTwDB(c.Request.Context(), db)
	row := twdb.QueryRowContext(c.Request.Context(), "SELECT id, first_name, last_name, email FROM users WHERE id = ?", id)

	var u User
	if err := row.Scan(&u.Id, &u.FirstName, &u.LastName, &u.Email); err != nil {
		if err == sql.ErrNoRows {
			c.JSON(http.StatusNotFound, gin.H{"error": "user not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, u)
}

func createUser(c *gin.Context) {
	var u User
	if err := c.ShouldBindJSON(&u); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	twdb := tracewaydb.NewTwDB(c.Request.Context(), db)
	result, err := twdb.ExecContext(c.Request.Context(), "INSERT INTO users (first_name, last_name, email) VALUES (?, ?, ?)", u.FirstName, u.LastName, u.Email)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	id, _ := result.LastInsertId()
	u.Id = int(id)
	c.JSON(http.StatusCreated, u)
}

func updateUser(c *gin.Context) {
	id := c.Param("id")
	var u User
	if err := c.ShouldBindJSON(&u); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	twdb := tracewaydb.NewTwDB(c.Request.Context(), db)
	result, err := twdb.ExecContext(c.Request.Context(), "UPDATE users SET first_name = ?, last_name = ?, email = ? WHERE id = ?", u.FirstName, u.LastName, u.Email, id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "user not found"})
		return
	}

	idInt, _ := strconv.Atoi(id)
	u.Id = idInt
	c.JSON(http.StatusOK, u)
}

func deleteUser(c *gin.Context) {
	id := c.Param("id")
	twdb := tracewaydb.NewTwDB(c.Request.Context(), db)
	result, err := twdb.ExecContext(c.Request.Context(), "DELETE FROM users WHERE id = ?", id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "user not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "user deleted"})
}

var veryLongJsonForTestin = `{"str": "traceway", "obj": {"id": 1}, "obj2": {"id": 1}, "obj3": {"id": 1}, "obj4": {"id": 1}, "obj5loremipsumdoloret": {"id": "I'm baby tumeric VHS Brooklyn, echo park literally you probably haven't heard of them crucifix taiyaki chambray roof party man bun knausgaard waistcoat squid health goth. Gastropub godard bodega boys snackwave asymmetrical la croix. Whatever try-hard pour-over humblebrag austin microdosing organic bruh. Keffiyeh mukbang yuccie, 90's humblebrag roof party godard kale chips lo-fi sriracha aesthetic.", "id2": "ImbabytumericVHSBrooklynechoparkliterallyyouprobablyhaventheardofthemcrucifixtaiyakichambrayroofpartymanbunknausgaardwaistcoatsquidhealthgothGastropubgodardbodegaboyssnackwaveasymmetricallacroixWhatevertryhardpouroverhumblebragaustinmicrodosingorganicbruhKeffiyehmukbangyuccieshumblebragroofpartygodardkalechipslofisrirachaaesthetic"}, "arr": [1, 2, "", {"key": 1, "key2": "example"}]}`
