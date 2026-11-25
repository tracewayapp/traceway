package cmd

import (
	"context"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/tracewayapp/traceway/backend/v2/app/services"
	"github.com/tracewayapp/traceway/backend/v2/app/slo"
	"github.com/tracewayapp/traceway/backend/v2/app/traceway"
)

// Shutdown is the work a stopping process must not skip. The server has no
// graceful shutdown of its own: it hands its SLO shards to the instances that
// remain, writes the client metric cells that live only in memory, and flushes
// self-monitoring last so the telemetry of the first two is exported too.
func Shutdown(ctx context.Context) {
	slo.Stop(ctx)
	// The cells collected since the last flush live only in memory.
	_ = services.ClientMetrics().Flush(ctx)
	_ = traceway.Shutdown(ctx)
}

// ShutdownOnSignal waits for SIGTERM or an interrupt, runs Shutdown with a
// five second budget, then raises the signal again so the process ends exactly
// as it would have without it. Run does not install it, since an embedding app
// owns its own signals: a binary opts in with `go ShutdownOnSignal()`.
func ShutdownOnSignal() {
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, syscall.SIGTERM, os.Interrupt)
	sig := <-signals

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	Shutdown(ctx)
	cancel()

	signal.Reset(sig)
	if self, err := os.FindProcess(os.Getpid()); err == nil && self.Signal(sig) == nil {
		time.Sleep(time.Second)
	}
	os.Exit(1)
}
