// Package worker runs the consumer loop.
package worker

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"runtime/debug"
	"time"

	"example.com/fulfillment/internal/labels"
	"example.com/fulfillment/internal/queue"
	"example.com/fulfillment/internal/telemetry"
)

// Source is where messages come from.
type Source interface {
	Group() string
	Fetch(ctx context.Context) (*queue.Message, error)
	Commit(ctx context.Context, m *queue.Message) error
}

// Worker consumes fulfillment.orders and hands each order to the labels service.
type Worker struct {
	src    Source
	labels *labels.Service
}

// New returns a Worker.
func New(src Source, l *labels.Service) *Worker { return &Worker{src: src, labels: l} }

// Run consumes until ctx is done.
func (w *Worker) Run(ctx context.Context) error {
	for ctx.Err() == nil {
		m, err := w.src.Fetch(ctx)
		if err != nil {
			slog.WarnContext(ctx, "fetch failed", "err", err)
			time.Sleep(time.Second)
			continue
		}
		w.handle(ctx, m)
		if err := w.src.Commit(ctx, m); err != nil {
			slog.WarnContext(ctx, "commit failed", "err", err)
		}
	}
	return ctx.Err()
}

func (w *Worker) handle(ctx context.Context, m *queue.Message) {
	ctx, end := telemetry.StartConsumer(ctx, "process "+m.Topic,
		telemetry.Attr{Key: "messaging.system", Value: "kafka"},
		telemetry.Attr{Key: "messaging.operation.type", Value: "process"},
		telemetry.Attr{Key: "messaging.destination.name", Value: m.Topic},
		telemetry.Attr{Key: "messaging.consumer.group.name", Value: w.src.Group()},
		telemetry.Attr{Key: "messaging.kafka.message.key", Value: m.Key},
		telemetry.Attr{Key: "messaging.kafka.offset", Value: m.Offset})
	defer end()
	defer func() {
		if v := recover(); v != nil {
			telemetry.RecordPanic(ctx, v, debug.Stack())
			slog.ErrorContext(ctx, "panic handling message", "topic", m.Topic, "offset", m.Offset, "panic", fmt.Sprint(v))
		}
	}()
	var ev labels.OrderPaid
	if err := json.Unmarshal(m.Value, &ev); err != nil {
		slog.ErrorContext(ctx, "message failed", "topic", m.Topic, "offset", m.Offset, "err", fmt.Errorf("decode: %w", err))
		return
	}
	if err := w.labels.Handle(ctx, ev); err != nil {
		telemetry.RecordError(ctx, err)
		slog.ErrorContext(ctx, "message failed", "topic", m.Topic, "offset", m.Offset, "err", err)
	}
}
