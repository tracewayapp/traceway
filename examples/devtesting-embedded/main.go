package main

import (
	"context"
	"embed"
	"errors"
	"fmt"
	"io/fs"
	"math/rand/v2"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"go.opentelemetry.io/contrib/instrumentation/github.com/gin-gonic/gin/otelgin"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploghttp"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	otellog "go.opentelemetry.io/otel/log"
	sdklog "go.opentelemetry.io/otel/sdk/log"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
	"go.opentelemetry.io/otel/trace"

	tracewaybackend "github.com/tracewayapp/traceway/backend"
)

//go:embed index.html
var indexHTML []byte

//go:embed cdn.html
var cdnHTML []byte

//go:embed static/*
var staticFS embed.FS

const (
	appPort            = 8080
	backendToken       = "backend-dev-token"
	frontendToken      = "frontend-dev-token"
	monitoringToken    = "monitoring-dev-token"
	flutterToken       = "flutter-dev-token"
	flutterUploadToken = "flutter-upload-token"

	backendServiceName = "backend-service"
	workerServiceName  = "worker-service"

	otlpHost = "localhost:8082"
)

type otelService struct {
	name string
	tp   *sdktrace.TracerProvider
	lp   *sdklog.LoggerProvider
	tr   trace.Tracer
	lg   otellog.Logger
}

func (s *otelService) shutdown(ctx context.Context) {
	_ = s.tp.Shutdown(ctx)
	_ = s.lp.Shutdown(ctx)
}

func initOtelService(ctx context.Context, serviceName, token string, extraResourceAttrs ...attribute.KeyValue) (*otelService, error) {
	headers := map[string]string{"Authorization": "Bearer " + token}

	traceExporter, err := otlptracehttp.New(ctx,
		otlptracehttp.WithEndpoint(otlpHost),
		otlptracehttp.WithInsecure(),
		otlptracehttp.WithURLPath("/api/otel/v1/traces"),
		otlptracehttp.WithHeaders(headers),
	)
	if err != nil {
		return nil, fmt.Errorf("%s trace exporter: %w", serviceName, err)
	}

	logExporter, err := otlploghttp.New(ctx,
		otlploghttp.WithEndpoint(otlpHost),
		otlploghttp.WithInsecure(),
		otlploghttp.WithURLPath("/api/otel/v1/logs"),
		otlploghttp.WithHeaders(headers),
	)
	if err != nil {
		return nil, fmt.Errorf("%s log exporter: %w", serviceName, err)
	}

	res, err := resource.New(ctx,
		resource.WithAttributes(append([]attribute.KeyValue{semconv.ServiceName(serviceName)}, extraResourceAttrs...)...),
	)
	if err != nil {
		return nil, fmt.Errorf("%s resource: %w", serviceName, err)
	}

	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(traceExporter, sdktrace.WithBatchTimeout(2*time.Second)),
		sdktrace.WithResource(res),
	)

	lp := sdklog.NewLoggerProvider(
		sdklog.WithProcessor(sdklog.NewBatchProcessor(logExporter, sdklog.WithExportInterval(2*time.Second))),
		sdklog.WithResource(res),
	)

	return &otelService{
		name: serviceName,
		tp:   tp,
		lp:   lp,
		tr:   tp.Tracer(serviceName),
		lg:   lp.Logger(serviceName),
	}, nil
}

func (s *otelService) log(ctx context.Context, sev otellog.Severity, sevText, body string, attrs ...otellog.KeyValue) {
	rec := otellog.Record{}
	now := time.Now()
	rec.SetTimestamp(now)
	rec.SetObservedTimestamp(now)
	rec.SetSeverity(sev)
	rec.SetSeverityText(sevText)
	rec.SetBody(otellog.StringValue(body))
	if len(attrs) > 0 {
		rec.AddAttributes(attrs...)
	}
	s.lg.Emit(ctx, rec)
}

func main() {
	go tracewaybackend.Run(
		tracewaybackend.WithSQLitePath("./storage/traceway.db"),
		tracewaybackend.WithPort(8082),
		tracewaybackend.WithDefaultUser("admin@localhost.com", "admin"),
		tracewaybackend.WithDefaultProject("Backend API", "opentelemetry", backendToken),
		tracewaybackend.WithDefaultProject("jQuery Frontend", "jquery", frontendToken),
		tracewaybackend.WithDefaultProject("Traceway Monitoring", "gin", monitoringToken),
		tracewaybackend.WithDefaultProject("Flutter App", "flutter", flutterToken),
		tracewaybackend.WithDefaultProjectSourceMapToken("Flutter App", flutterUploadToken),
		tracewaybackend.WithMonitoringURL(monitoringToken+"@http://localhost:8082/api/report"),
	)

	time.Sleep(2 * time.Second)

	ctx := context.Background()

	backendSvc, err := initOtelService(ctx, backendServiceName, backendToken)
	if err != nil {
		panic(err)
	}
	defer backendSvc.shutdown(ctx)

	workerSvc, err := initOtelService(ctx, workerServiceName, backendToken)
	if err != nil {
		panic(err)
	}
	defer workerSvc.shutdown(ctx)

	attrTestSvc, err := initOtelService(ctx, "attr-test-service", backendToken,
		attribute.String("os.description", "7.0.11-orbstack-00360-gc9bc4d96ac70 #1 SMP PREEMPT_DYNAMIC Thu Jun 4 16:40:25 UTC 2026 aarch64 GNU/Linux"),
		attribute.String("os.version", "#1 SMP PREEMPT_DYNAMIC Thu Jun 4 16:40:25 UTC 2026"),
		attribute.String("host.id", strings.Repeat("f3b47b65006f", 6)),
		attribute.String("process.command_line", "/usr/local/sbin/php-fpm --nodaemonize --fpm-config /usr/local/etc/php-fpm.d/www.conf --force-stderr"),
	)
	if err != nil {
		panic(err)
	}
	defer attrTestSvc.shutdown(ctx)

	router := gin.Default()

	router.Use(func(c *gin.Context) {
		origin := c.GetHeader("Origin")
		if origin != "" {
			c.Header("Access-Control-Allow-Origin", origin)
		}
		c.Header("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Content-Type, Authorization, traceway-trace-id")
		c.Header("Access-Control-Expose-Headers", "traceway-trace-id")
		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		c.Next()
	})

	router.Use(otelgin.Middleware(backendServiceName, otelgin.WithTracerProvider(backendSvc.tp)))

	router.GET("/", func(c *gin.Context) {
		c.Data(http.StatusOK, "text/html; charset=utf-8", indexHTML)
	})

	router.GET("/cdn", func(c *gin.Context) {
		c.Data(http.StatusOK, "text/html; charset=utf-8", cdnHTML)
	})

	staticSub, _ := fs.Sub(staticFS, "static")
	router.StaticFS("/static", http.FS(staticSub))

	router.GET("/api/test-error", func(c *gin.Context) {
		ctx := c.Request.Context()
		backendSvc.log(ctx, otellog.SeverityInfo, "INFO", "received test-error request")
		time.Sleep(time.Duration(20+rand.IntN(80)) * time.Millisecond)
		err := errors.New("simulated backend error for distributed trace testing")
		backendSvc.log(ctx, otellog.SeverityError, "ERROR", "handler failed: "+err.Error())
		span := trace.SpanFromContext(ctx)
		span.RecordError(err, trace.WithStackTrace(true))
		span.SetStatus(codes.Error, err.Error())
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
	})

	router.GET("/api/test-success", func(c *gin.Context) {
		ctx := c.Request.Context()
		backendSvc.log(ctx, otellog.SeverityDebug, "DEBUG", "test-success entered")
		time.Sleep(time.Duration(20+rand.IntN(80)) * time.Millisecond)
		backendSvc.log(ctx, otellog.SeverityInfo, "INFO", "test-success completed")
		c.JSON(http.StatusOK, gin.H{"message": "success"})
	})

	router.GET("/api/test-log-levels", func(c *gin.Context) {
		ctx := c.Request.Context()
		backendSvc.log(ctx, otellog.SeverityTrace1, "TRACE", "trace-level log for visual testing")
		backendSvc.log(ctx, otellog.SeverityDebug, "DEBUG", "debug: cache miss, falling back to db")
		backendSvc.log(ctx, otellog.SeverityInfo, "INFO", "info: request accepted")
		backendSvc.log(ctx, otellog.SeverityWarn, "WARN", "warn: connection pool at 80% capacity")
		backendSvc.log(ctx, otellog.SeverityError, "ERROR", "error: downstream returned non-2xx")
		backendSvc.log(ctx, otellog.SeverityFatal, "FATAL", "fatal: synthetic fatal for UI testing, nothing is actually broken")
		c.JSON(http.StatusOK, gin.H{"emitted": 6})
	})

	router.GET("/api/test-spans-with-logs", func(c *gin.Context) {
		ctx := c.Request.Context()
		backendSvc.log(ctx, otellog.SeverityInfo, "INFO", "handler: entering /test-spans-with-logs")

		authCtx, authSpan := backendSvc.tr.Start(ctx, "auth.verify", trace.WithAttributes(
			attribute.String("auth.method", "bearer"),
			attribute.Int("user.id", 42),
		))
		backendSvc.log(authCtx, otellog.SeverityInfo, "INFO", "auth: token verified")
		time.Sleep(5 * time.Millisecond)
		authSpan.End()

		dbCtx, dbSpan := backendSvc.tr.Start(ctx, "db.query", trace.WithAttributes(
			attribute.String("db.system", "sqlite"),
			attribute.String("db.operation", "SELECT"),
			attribute.String("db.collection.name", "users"),
		))
		backendSvc.log(dbCtx, otellog.SeverityDebug, "DEBUG", "db: executing SELECT * FROM users WHERE id = ?")
		time.Sleep(20 * time.Millisecond)

		cacheCtx, cacheSpan := backendSvc.tr.Start(dbCtx, "cache.lookup", trace.WithAttributes(
			attribute.String("cache.key", "user:42"),
			attribute.Bool("cache.hit", true),
		))
		backendSvc.log(cacheCtx, otellog.SeverityInfo, "INFO", "cache: key user:42 -> hit")
		time.Sleep(2 * time.Millisecond)
		cacheSpan.End()

		backendSvc.log(dbCtx, otellog.SeverityWarn, "WARN", "db: query took longer than expected",
			otellog.String("threshold_ms", "20"))
		dbSpan.End()

		backendSvc.log(ctx, otellog.SeverityInfo, "INFO", "handler: returning 200")
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	router.GET("/api/test-distributed-logs", func(c *gin.Context) {
		ctx := c.Request.Context()
		dtid := uuid.New().String()

		rootSpan := trace.SpanFromContext(ctx)
		rootSpan.SetAttributes(attribute.String("traceway.distributed_trace_id", dtid))

		backendSvc.log(ctx, otellog.SeverityInfo, "INFO", "backend: received request, about to call worker",
			otellog.String("distributed_trace_id", dtid))

		workerCtx, workerSpan := workerSvc.tr.Start(context.Background(), "worker.process-job",
			trace.WithSpanKind(trace.SpanKindConsumer))
		workerSpan.SetAttributes(attribute.String("traceway.distributed_trace_id", dtid))

		workerSvc.log(workerCtx, otellog.SeverityInfo, "INFO", "worker: starting job",
			otellog.String("distributed_trace_id", dtid))
		time.Sleep(15 * time.Millisecond)
		workerSvc.log(workerCtx, otellog.SeverityDebug, "DEBUG", "worker: step 1 complete")
		time.Sleep(15 * time.Millisecond)
		workerSvc.log(workerCtx, otellog.SeverityWarn, "WARN", "worker: retryable downstream error, will retry once")
		time.Sleep(10 * time.Millisecond)
		workerSvc.log(workerCtx, otellog.SeverityInfo, "INFO", "worker: job complete")
		workerSpan.End()

		backendSvc.log(ctx, otellog.SeverityInfo, "INFO", "backend: worker reported success, returning 200")
		c.JSON(http.StatusOK, gin.H{
			"status":             "ok",
			"distributedTraceId": dtid,
		})
	})

	router.GET("/api/test-long-attributes", func(c *gin.Context) {
		ctx := c.Request.Context()
		longToken := strings.Repeat("0646a849a52752904984ab92b2a39f1c", 12)
		span := trace.SpanFromContext(ctx)
		span.SetAttributes(
			attribute.String("request.signature", longToken),
			attribute.String("http.user_agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/137.0.0.0 Safari/537.36"),
		)
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	router.GET("/api/test-long-log-attributes", func(c *gin.Context) {
		ctx := c.Request.Context()
		longToken := strings.Repeat("0646a849a52752904984ab92b2a39f1c", 12)
		attrTestSvc.log(ctx, otellog.SeverityDebug, "DEBUG",
			"log line with long attributes; unbroken debug context: "+longToken,
			otellog.String("auth.token", longToken),
			otellog.String("stack.preview", "goroutine 1 [running]:\nmain.handler(0x14000123456)\n\t/app/main.go:42 +0x1f4\nmain.process(0x14000123456)\n\t/app/worker.go:88 +0x2bc"),
			otellog.String("payload.json", `{"user":{"id":"0646a849a52752904984ab92b2a39f1c","token":"`+longToken+`"}}`),
		)
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	router.GET("/api/test-sse", func(c *gin.Context) {
		span := trace.SpanFromContext(c.Request.Context())
		span.SetAttributes(attribute.Bool("traceway.is_stream", true))
		writeSSEStream(c, 30*time.Second, time.Second)
	})

	router.GET("/api/test-sse-short", func(c *gin.Context) {
		span := trace.SpanFromContext(c.Request.Context())
		span.SetAttributes(attribute.Bool("traceway.is_stream", true))
		writeSSEStream(c, 5*time.Second, 500*time.Millisecond)
	})

	router.GET("/api/test-long-poll", func(c *gin.Context) {
		span := trace.SpanFromContext(c.Request.Context())
		span.SetAttributes(attribute.Bool("traceway.is_stream", true))
		time.Sleep(10 * time.Second)
		c.JSON(http.StatusOK, gin.H{"status": "ok", "via": "long-poll"})
	})

	fmt.Println()
	fmt.Println("=================================================")
	fmt.Printf("  Node build:       http://localhost:%d\n", appPort)
	fmt.Printf("  CDN (no build):   http://localhost:%d/cdn\n", appPort)
	fmt.Println("  Dashboard:        http://localhost:8082")
	fmt.Println("  Login:            admin@localhost.com / admin")
	fmt.Println()
	fmt.Println("  OTel logs test endpoints (hit with curl or browser):")
	fmt.Printf("    curl http://localhost:%d/api/test-error\n", appPort)
	fmt.Printf("    curl http://localhost:%d/api/test-success\n", appPort)
	fmt.Printf("    curl http://localhost:%d/api/test-log-levels\n", appPort)
	fmt.Printf("    curl http://localhost:%d/api/test-spans-with-logs\n", appPort)
	fmt.Printf("    curl http://localhost:%d/api/test-distributed-logs\n", appPort)
	fmt.Printf("    curl http://localhost:%d/api/test-long-attributes\n", appPort)
	fmt.Printf("    curl http://localhost:%d/api/test-long-log-attributes\n", appPort)
	fmt.Println()
	fmt.Println("  Streaming endpoints (is_stream — expect a 'Stream' badge):")
	fmt.Printf("    curl -N http://localhost:%d/api/test-sse\n", appPort)
	fmt.Printf("    curl -N http://localhost:%d/api/test-sse-short\n", appPort)
	fmt.Printf("    curl http://localhost:%d/api/test-long-poll\n", appPort)
	fmt.Println("=================================================")
	fmt.Println()

	router.Run(fmt.Sprintf(":%d", appPort))
}
func writeSSEStream(c *gin.Context, duration, interval time.Duration) {
	c.Writer.Header().Set("Content-Type", "text/event-stream")
	c.Writer.Header().Set("Cache-Control", "no-cache")
	c.Writer.Header().Set("Connection", "keep-alive")
	c.Writer.Header().Set("X-Accel-Buffering", "no")
	c.Writer.WriteHeader(http.StatusOK)
	c.Writer.Flush()

	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	timeout := time.After(duration)

	clientGone := c.Request.Context().Done()
	tick := 0
	for {
		select {
		case <-clientGone:
			return
		case <-timeout:
			return
		case t := <-ticker.C:
			tick++
			fmt.Fprintf(c.Writer, "event: tick\ndata: {\"n\":%d,\"time\":\"%s\"}\n\n", tick, t.Format(time.RFC3339Nano))
			c.Writer.Flush()
		}
	}
}
