// Package logging builds the service logger: slog through the OpenTelemetry
// bridge, so every record carries the trace and span of the request that
// wrote it, plus a JSON copy on stdout for kubectl logs.
package logging

import (
	"context"
	"log/slog"
	"os"
	"strings"

	"go.opentelemetry.io/contrib/bridges/otelslog"
)

// New returns the logger for the level named by LOG_LEVEL (debug, info,
// warn, error; default info).
func New(level string) *slog.Logger {
	var lvl slog.Level
	switch strings.ToLower(level) {
	case "debug":
		lvl = slog.LevelDebug
	case "warn":
		lvl = slog.LevelWarn
	case "error":
		lvl = slog.LevelError
	default:
		lvl = slog.LevelInfo
	}
	stdout := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: lvl, AddSource: lvl == slog.LevelDebug})
	return slog.New(fanout{min: lvl, hs: []slog.Handler{stdout, otelslog.NewHandler("listings-api")}})
}

// fanout writes every record to each handler.
type fanout struct {
	min slog.Level
	hs  []slog.Handler
}

func (f fanout) Enabled(_ context.Context, l slog.Level) bool { return l >= f.min }

func (f fanout) Handle(ctx context.Context, r slog.Record) error {
	for _, h := range f.hs {
		if err := h.Handle(ctx, r.Clone()); err != nil {
			return err
		}
	}
	return nil
}

func (f fanout) WithAttrs(as []slog.Attr) slog.Handler {
	out := fanout{min: f.min}
	for _, h := range f.hs {
		out.hs = append(out.hs, h.WithAttrs(as))
	}
	return out
}

func (f fanout) WithGroup(name string) slog.Handler {
	out := fanout{min: f.min}
	for _, h := range f.hs {
		out.hs = append(out.hs, h.WithGroup(name))
	}
	return out
}
