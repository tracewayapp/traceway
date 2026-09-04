package cmd

import (
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/coreos/go-systemd/v22/daemon"
	"github.com/tracewayapp/traceway/backend/app/config"
	traceway "go.tracewayapp.com"
)

// Boot can outlast systemd's default TimeoutStartSec=90s -- an index-building
// migration is the usual reason, which is why the Helm chart budgets 300s for
// its startup probe. Rather than ask operators to guess a TimeoutStartSec for
// their largest migration, hold the deadline open while boot runs: every
// EXTEND_TIMEOUT_USEC message pushes it out from the moment systemd receives
// it.
//
// The window is several times the interval so that one late heartbeat -- a
// stalled disk, a stop-the-world pause -- cannot let the deadline lapse.
//
// The total is bounded because these messages go out on a timer rather than on
// boot progress: without a budget, a boot wedged against an unreachable
// database would hold the deadline open forever, which deletes
// TimeoutStartSec instead of deferring it. Once the budget is spent, systemd's
// own timeout applies again.
const (
	bootExtendInterval = 10 * time.Second
	bootExtendWindow   = 60 * time.Second
	bootExtendBudget   = 30 * time.Minute
)

// beginBoot holds the service manager's startup timeout open, and returns the
// function that ends the boot phase by reporting the service ready.
//
// Reporting ready is the same call so that no caller can leave a heartbeat
// running past it: once the service is running, EXTEND_TIMEOUT_USEC extends
// the *watchdog* deadline rather than the startup one, where a stray message
// would paper over a hung process.
//
// All of it is a no-op when the process was not started by a service manager
// (NOTIFY_SOCKET unset), which is every other way Traceway runs: Docker,
// Kubernetes, go run.
func beginBoot(interval, window, budget time.Duration) (ready func()) {
	stop := startBootHeartbeat(interval, window, budget)

	return sync.OnceFunc(func() {
		stop()
		notifyReady()
	})
}

func startBootHeartbeat(interval, window, budget time.Duration) (stop func()) {
	// Ask the environment rather than infer it from the first send: SdNotify
	// also reports "not sent" for a transient socket error, and treating that
	// as "no service manager" would abandon the heartbeat for the whole boot
	// and hand the process back to the 90s default this exists to survive.
	if os.Getenv("NOTIFY_SOCKET") == "" {
		return func() {}
	}

	extendStartTimeout(window)

	stopped := make(chan struct{})
	done := make(chan struct{})

	go func() {
		defer traceway.Recover()
		defer close(done)

		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		spent := time.After(budget)

		for {
			select {
			case <-ticker.C:
				extendStartTimeout(window)
			case <-spent:
				config.Logf("No longer extending the systemd startup timeout: boot has taken %s", budget)
				return
			case <-stopped:
				return
			}
		}
	}()

	return sync.OnceFunc(func() {
		close(stopped)
		<-done
	})
}

func extendStartTimeout(window time.Duration) {
	if _, err := daemon.SdNotify(false, fmt.Sprintf("EXTEND_TIMEOUT_USEC=%d", window.Microseconds())); err != nil {
		config.Logf("Failed to extend the systemd startup timeout: %v", err)
	}
}

func notifyReady() {
	sent, err := daemon.SdNotify(false, daemon.SdNotifyReady)
	if err != nil {
		config.Logf("Failed to notify systemd: %v", err)
	} else if sent {
		config.Logln("Notified systemd that service is ready")
	}

	startWatchdog()
}

// startWatchdog pings the watchdog at the interval systemd asked for rather
// than a fixed one: WatchdogSec=10s expects a ping every 5s, and the hardcoded
// 15s this replaces would have had systemd kill a perfectly healthy process.
func startWatchdog() {
	ping, err := watchdogPingInterval()
	if err != nil {
		config.Logf("Failed to read the systemd watchdog interval: %v", err)
		return
	}
	if ping <= 0 {
		return
	}

	config.Logf("Pinging the systemd watchdog every %s", ping)

	go func() {
		defer traceway.Recover()

		ticker := time.NewTicker(ping)
		defer ticker.Stop()
		for range ticker.C {
			daemon.SdNotify(false, daemon.SdNotifyWatchdog)
		}
	}()
}

// watchdogPingInterval is half of WATCHDOG_USEC, the rate sd_notify(3) asks
// for. It is 0 when the watchdog is disabled or another process is the watched
// PID, in which case nothing should be sent at all.
func watchdogPingInterval() (time.Duration, error) {
	interval, err := daemon.SdWatchdogEnabled(false)
	return interval / 2, err
}
