package cmd

import (
	"context"
	"fmt"
	"time"

	"github.com/coreos/go-systemd/v22/daemon"
	"github.com/tracewayapp/traceway/backend/app/config"
	"github.com/tracewayapp/traceway/backend/app/traceway"
)

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
