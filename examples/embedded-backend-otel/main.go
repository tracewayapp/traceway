package main

import (
	"context"
	"errors"
	"fmt"
	"math/rand/v2"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	tracewaybackend "github.com/tracewayapp/traceway/backend"
	"go.opentelemetry.io/contrib/instrumentation/github.com/gin-gonic/gin/otelgin"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
	"go.opentelemetry.io/otel/trace"
)

const (
	appPort       = 8080
	tracewayToken = "backend-dev-token"
	serviceName   = "embedded-example"
)

func main() {
	// 1: initialize traceway backend
	go tracewaybackend.Run(
		tracewaybackend.WithPort(8082),
		tracewaybackend.WithDefaultUser("admin@localhost.com", "admin"),
		tracewaybackend.WithDefaultProject("Example App", "opentelemetry", tracewayToken),
		tracewaybackend.DisableLogging(),
	)

	// 2: initialize otel
	shutdown := initTracer()
	defer shutdown()

	router := gin.Default()
	router.Use(otelgin.Middleware(serviceName))

	// 3: create a simple test endpoint
	router.GET("/hello/:name", func(c *gin.Context) {
		name := c.Param("name")
		tracer := otel.Tracer(serviceName)

		_, span := tracer.Start(c.Request.Context(), "db.lookup")
		time.Sleep(time.Duration(20+rand.IntN(80)) * time.Millisecond)
		span.End()

		if name == "error" {
			err := errors.New("something went wrong")
			rootSpan := trace.SpanFromContext(c.Request.Context())
			rootSpan.RecordError(err, trace.WithStackTrace(true))
			rootSpan.SetStatus(codes.Error, err.Error())
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

func initTracer() func() {
	exporter, err := otlptracehttp.New(
		context.Background(),
		otlptracehttp.WithEndpointURL("http://localhost:8082/api/otel/v1/traces"),
		otlptracehttp.WithHeaders(map[string]string{
			"Authorization": "Bearer " + tracewayToken,
		}),
	)
	if err != nil {
		panic(fmt.Sprintf("failed to create OTLP exporter: %v", err))
	}

	res, err := resource.New(
		context.Background(),
		resource.WithAttributes(
			semconv.ServiceName(serviceName),
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
