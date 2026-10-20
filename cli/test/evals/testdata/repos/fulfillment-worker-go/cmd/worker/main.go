package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"

	"example.com/fulfillment/internal/labels"
	"example.com/fulfillment/internal/queue"
	"example.com/fulfillment/internal/telemetry"
	"example.com/fulfillment/internal/worker"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	shutdown, err := telemetry.Setup(ctx)
	if err != nil {
		slog.Error("telemetry setup", "err", err)
		os.Exit(1)
	}
	defer shutdown(context.Background())
	deps, err := connect(ctx)
	if err != nil {
		slog.Error("connect", "err", err)
		os.Exit(1)
	}
	src := queue.NewConsumer(os.Getenv("QUEUE_GATEWAY_URL"), "label-printer", "fulfillment.orders")
	w := worker.New(src, labels.NewService(deps.orders, deps.carrier, deps.store))
	slog.Info("consuming", "topic", "fulfillment.orders")
	if err := w.Run(ctx); err != nil && ctx.Err() == nil {
		slog.Error("worker stopped", "err", err)
		os.Exit(1)
	}
}
