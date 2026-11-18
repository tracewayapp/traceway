package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"
	"time"

	tracewaybackend "github.com/tracewayapp/traceway/backend/v2"
	"github.com/tracewayapp/traceway/backend/v2/app/services"
	"github.com/tracewayapp/traceway/backend/v2/app/traceway"
)

func main() {
	go flushOnSignal()
	tracewaybackend.Run()
	// tracewaybackend.Run(
	// 	tracewaybackend.WithPort(8082),
	// 	tracewaybackend.WithDefaultUser("admin@localhost.com", "admin"),
	// 	tracewaybackend.WithDefaultProject("Backend", "gin", "backend-dev-token"),
	// 	tracewaybackend.WithDefaultProject("Frontend", "svelte", "frontend-dev-token"),
	// )
}

// The server has no graceful shutdown, so a stop signal ends it at once. The
// client metrics still in memory and the last few seconds of self-monitoring are flushed first, then the signal is
// raised again so the process ends exactly as it did before. This lives in
// the binary, not in Run: an embedding app owns its own signals.
func flushOnSignal() {
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, syscall.SIGTERM, os.Interrupt)
	sig := <-signals

	flushCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	// The cells collected since the last flush live only in memory.
	_ = services.ClientMetrics().Flush(flushCtx)
	_ = traceway.Shutdown(flushCtx)
	cancel()

	signal.Reset(sig)
	if self, err := os.FindProcess(os.Getpid()); err == nil && self.Signal(sig) == nil {
		time.Sleep(time.Second)
	}
	os.Exit(1)
}
