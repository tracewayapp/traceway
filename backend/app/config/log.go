package config

import (
	"context"
	"fmt"
	"log"
	"strings"

	"github.com/tracewayapp/traceway/backend/app/traceway"
	otellog "go.opentelemetry.io/otel/log"
)

var LoggingEnabled = true

func Logf(format string, v ...any) {
	LogfContext(context.Background(), format, v...)
}

func Logln(v ...any) {
	if !LoggingEnabled {
		return
	}
	msg := strings.TrimSuffix(fmt.Sprintln(v...), "\n")
	log.Println("[tracewaybackend] " + msg)
	traceway.EmitLog(context.Background(), severityOf(msg), msg)
}

func LogfContext(ctx context.Context, format string, v ...any) {
	if !LoggingEnabled {
		return
	}
	msg := fmt.Sprintf(format, v...)
	log.Print("[tracewaybackend] " + msg)
	traceway.EmitLog(ctx, severityOf(msg), msg)
}

func severityOf(msg string) otellog.Severity {
	upper := strings.ToUpper(msg)
	switch {
	case strings.HasPrefix(upper, "ERROR"), strings.HasPrefix(upper, "FATAL"), strings.HasPrefix(upper, "FAILED"):
		return otellog.SeverityError
	case strings.HasPrefix(upper, "WARNING"), strings.HasPrefix(upper, "WARN"):
		return otellog.SeverityWarn
	}
	return otellog.SeverityInfo
}
