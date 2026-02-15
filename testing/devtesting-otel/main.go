package main

import (
	"context"
	"errors"
	"fmt"
	"math/rand/v2"
	"net/http"
	"os"
	"runtime"
	"time"

	"github.com/gin-gonic/gin"
	"go.opentelemetry.io/contrib/instrumentation/github.com/gin-gonic/gin/otelgin"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
	"go.opentelemetry.io/otel/trace"
)

type CustomError struct {
	Code    int
	Message string
}

func (e *CustomError) Error() string {
	return fmt.Sprintf("CustomError[%d]: %s", e.Code, e.Message)
}

func initTracer() func() {
	token := os.Getenv("TRACEWAY_TOKEN")
	if token == "" {
		token = "default_token_change_me"
	}

	backend := os.Getenv("TRACEWAY_BACKEND")
	if backend == "" {
		backend = "http://localhost:8082"
	}

	exporter, err := otlptracehttp.New(
		context.Background(),
		otlptracehttp.WithEndpointURL(backend+"/api/otel/v1/traces"),
		otlptracehttp.WithHeaders(map[string]string{
			"Authorization": "Bearer " + token,
		}),
	)
	if err != nil {
		panic(fmt.Sprintf("failed to create OTLP exporter: %v", err))
	}

	res, err := resource.New(
		context.Background(),
		resource.WithAttributes(
			semconv.ServiceName("devtesting-otel"),
		),
	)
	if err != nil {
		panic(fmt.Sprintf("failed to create resource: %v", err))
	}

	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(res),
	)
	otel.SetTracerProvider(tp)

	return func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		tp.Shutdown(ctx)
	}
}

func main() {
	shutdown := initTracer()
	defer shutdown()

	router := gin.Default()
	router.Use(otelgin.Middleware("devtesting-otel"))

	router.GET("/test-ok", testOk)
	router.GET("/test-not-found", testNotFound)
	router.GET("/test-exception", testException)
	router.GET("/test-spans", testSpans)
	router.GET("/test-param/:param", testParam)
	router.GET("/test-cerror-simple", testCerrorSimple)
	router.GET("/test-cerror-stacktrace", testCerrorStacktrace)
	router.GET("/test-cerror-wrapped", testCerrorWrapped)
	router.GET("/test-cerror-custom", testCerrorCustom)

	router.Run(":8080")
}

func testOk(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

func testNotFound(c *gin.Context) {
	c.JSON(http.StatusNotFound, gin.H{"status": "not-found"})
}

func testException(c *gin.Context) {
	time.Sleep(time.Duration(rand.IntN(2000)) * time.Millisecond)

	err := errors.New("Cool")
	span := trace.SpanFromContext(c.Request.Context())
	span.RecordError(err, trace.WithStackTrace(true))
	span.SetStatus(codes.Error, err.Error())
	c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
}

func testSpans(c *gin.Context) {
	tracer := otel.Tracer("devtesting-otel")

	ctx, parentSpan := tracer.Start(c.Request.Context(), "db.and.cache")

	_, dbSpan := tracer.Start(ctx, "db.query")
	time.Sleep(time.Duration(50+rand.IntN(100)) * time.Millisecond)
	dbSpan.End()

	_, cacheSpan := tracer.Start(ctx, "cache.set")
	time.Sleep(time.Duration(10+rand.IntN(30)) * time.Millisecond)
	cacheSpan.End()

	_, httpSpan := tracer.Start(ctx, "http.external_api")
	time.Sleep(time.Duration(100+rand.IntN(200)) * time.Millisecond)
	httpSpan.End()

	parentSpan.End()

	c.JSON(http.StatusOK, gin.H{"status": "ok", "message": "Spans captured"})
}

func testParam(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"param": c.Param("param")})
}

func testCerrorSimple(c *gin.Context) {
	err := errors.New("simple error without stack")
	span := trace.SpanFromContext(c.Request.Context())
	span.RecordError(err, trace.WithStackTrace(true))
	span.SetStatus(codes.Error, err.Error())
	c.JSON(http.StatusInternalServerError, gin.H{"error": "simple error"})
}

func testCerrorStacktrace(c *gin.Context) {
	buf := make([]byte, 4096)
	n := runtime.Stack(buf, false)
	stackTrace := string(buf[:n])

	err := fmt.Errorf("error with stack trace\n%s", stackTrace)
	span := trace.SpanFromContext(c.Request.Context())
	span.RecordError(err, trace.WithStackTrace(true))
	span.SetStatus(codes.Error, "error with stack trace")
	c.JSON(http.StatusInternalServerError, gin.H{"error": "stacktrace error"})
}

func testCerrorWrapped(c *gin.Context) {
	base := errors.New("base error")
	wrapped := fmt.Errorf("layer 1: %w", base)
	wrapped2 := fmt.Errorf("layer 2: %w", wrapped)

	span := trace.SpanFromContext(c.Request.Context())
	span.RecordError(wrapped2, trace.WithStackTrace(true))
	span.SetStatus(codes.Error, wrapped2.Error())
	c.JSON(http.StatusInternalServerError, gin.H{"error": "wrapped error"})
}

func testCerrorCustom(c *gin.Context) {
	err := &CustomError{Code: 500, Message: "something went wrong"}
	span := trace.SpanFromContext(c.Request.Context())
	span.RecordError(err, trace.WithStackTrace(true))
	span.SetStatus(codes.Error, err.Error())
	c.JSON(http.StatusInternalServerError, gin.H{"error": "custom error"})
}
