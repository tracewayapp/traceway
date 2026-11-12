package config

import (
	"context"
	"fmt"
	"log"
	"strings"

	"github.com/tracewayapp/traceway/backend/v2/app/traceway"
	otellog "go.opentelemetry.io/otel/log"
)

var LoggingEnabled = true

func Logf(format string, v ...any) {
	LogfContext(context.Background(), format, v...)
}

// LoggingEnabled only quiets stdout: self-monitoring still receives the line.
func Logln(v ...any) {
	if !LoggingEnabled && !traceway.Enabled() {
		return
	}
	msg := strings.TrimSuffix(fmt.Sprintln(v...), "\n")
	if LoggingEnabled {
		log.Println("[tracewaybackend] " + msg)
	}
	traceway.EmitLog(context.Background(), severityOf(msg), msg)
}

func LogfContext(ctx context.Context, format string, v ...any) {
	if !LoggingEnabled && !traceway.Enabled() {
		return
	}
	msg := fmt.Sprintf(format, v...)
	if LoggingEnabled {
		log.Print("[tracewaybackend] " + msg)
	}
	traceway.EmitLog(ctx, severityOf(msg), msg)
}

// LogfLocal writes to stdout only and is never exported to self-monitoring,
// whose logs every member of the monitoring project's organization can read.
// A line that carries a secret (a token, a password reset or invitation
// link) goes through here.
func LogfLocal(format string, v ...any) {
	if LoggingEnabled {
		log.Print("[tracewaybackend] " + fmt.Sprintf(format, v...))
	}
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
