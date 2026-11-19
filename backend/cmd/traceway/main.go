package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/tracewayapp/traceway/backend/v2/app/services"
	"github.com/tracewayapp/traceway/backend/v2/app/traceway"
	"github.com/tracewayapp/traceway/backend/v2/cmd"
)

func main() {
	go flushOnSignal()
	cmd.RunStandalone()
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
