package cmd

import (
	"context"
	"fmt"
	"net"
	"time"

	"github.com/coreos/go-systemd/v22/daemon"
	"github.com/tracewayapp/traceway/backend/v2/app/config"
	"github.com/tracewayapp/traceway/backend/v2/app/traceway"
)

const (
	migrationHeartbeatInterval = 10 * time.Second
	migrationExtensionWindow   = time.Minute
	migrationExtensionBudget   = 30 * time.Minute
)

// A stalled migration must eventually fall back to the unit's startup timeout.
// Cap even the last extension at the budget remaining when we send it.
func startMigrationHeartbeat(extend func(time.Duration) error) func() {
	deadline := time.Now().Add(migrationExtensionBudget)
	stop := make(chan struct{})
	done := make(chan struct{})
	go func() {
		defer close(done)
		defer traceway.Recover()
		ticker := time.NewTicker(migrationHeartbeatInterval)
		defer ticker.Stop()
		failed := false
		for {
			remaining := time.Until(deadline)
			if remaining < time.Microsecond {
				return
			}
			err := extend(min(remaining, migrationExtensionWindow))
			if err != nil && !failed {
				reportSystemdError(fmt.Errorf("extend systemd migration timeout: %w", err))
			}
			failed = err != nil
			select {
			case <-stop:
				return
			case <-ticker.C:
			}
		}
	}()
	return func() {
		close(stop)
		<-done
	}
}

// SdNotify has no write deadline. A full notification socket must not hold up
// the end of migrations while we join the heartbeat before reporting ready.
func extendSystemdStartup(socket string, window time.Duration) error {
	deadline := time.Now().Add(250 * time.Millisecond)
	dialer := net.Dialer{Deadline: deadline}
	conn, err := dialer.Dial("unixgram", socket)
	if err != nil {
		return err
	}
	defer conn.Close()
	if err := conn.SetWriteDeadline(deadline); err != nil {
		return err
	}
	_, err = fmt.Fprintf(conn, "EXTEND_TIMEOUT_USEC=%d", window.Microseconds())
	return err
}

func notifySystemd(ctx context.Context) {
	sent, err := daemon.SdNotify(false, daemon.SdNotifyReady)
	if err != nil {
		reportSystemdError(fmt.Errorf("notify systemd that service is ready: %w", err))
	} else if sent {
		config.Logln("Notified systemd that service is ready")
	}

	interval, err := systemdWatchdogInterval()
	if err != nil {
		reportSystemdError(fmt.Errorf("read systemd watchdog configuration: %w", err))
		return
	}
	if interval == 0 {
		return
	}
	config.Logf("Systemd watchdog heartbeat interval: %s", interval)

	go func() {
		defer traceway.Recover()
		runSystemdWatchdog(ctx, interval, daemon.SdNotify)
	}()
}

func systemdWatchdogInterval() (time.Duration, error) {
	timeout, err := daemon.SdWatchdogEnabled(false)
	if err != nil || timeout == 0 {
		return 0, err
	}
	// Keep headroom for scheduling delays under load, even with a generous watchdog timeout.
	return min(timeout/2, 5*time.Second), nil
}

func runSystemdWatchdog(ctx context.Context, interval time.Duration, notify func(bool, string) (bool, error)) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	failed := false
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			sent, err := notify(false, daemon.SdNotifyWatchdog)
			if err == nil && !sent {
				err = fmt.Errorf("NOTIFY_SOCKET is unset")
			}
			if err != nil {
				if !failed {
					reportSystemdError(fmt.Errorf("send systemd watchdog heartbeat: %w", err))
				}
				failed = true
				continue
			}
			if failed {
				config.Logln("Systemd watchdog heartbeat delivery recovered")
			}
			failed = false
		}
	}
}

func reportSystemdError(err error) {
	config.Logf("Systemd notification failed: %v", err)
	traceway.CaptureException(err)
}
