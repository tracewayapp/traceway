package cmd

import (
	"context"
	"errors"
	"os"
	"strconv"
	"sync/atomic"
	"testing"
	"testing/synctest"
	"time"

	"github.com/coreos/go-systemd/v22/daemon"
)

func TestSystemdWatchdogInterval(t *testing.T) {
	for _, tt := range []struct {
		name    string
		usec    string
		pid     string
		want    time.Duration
		wantErr bool
	}{
		{name: "disabled"},
		{name: "production timeout", usec: "20000000", want: 5 * time.Second},
		{name: "short timeout", usec: "4000000", want: 2 * time.Second},
		{name: "long timeout", usec: "120000000", want: 5 * time.Second},
		{name: "matching pid", usec: "20000000", pid: strconv.Itoa(os.Getpid()), want: 5 * time.Second},
		{name: "other pid", usec: "20000000", pid: strconv.Itoa(os.Getpid() + 1)},
		{name: "invalid timeout", usec: "invalid", wantErr: true},
		{name: "zero timeout", usec: "0", wantErr: true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("WATCHDOG_USEC", tt.usec)
			t.Setenv("WATCHDOG_PID", tt.pid)
			got, err := systemdWatchdogInterval()
			if got != tt.want || (err != nil) != tt.wantErr {
				t.Fatalf("systemdWatchdogInterval() = (%s, %v), want (%s, error=%v)", got, err, tt.want, tt.wantErr)
			}
		})
	}
}

func TestSystemdWatchdogRetriesAndStopsOnCancellation(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		var calls atomic.Int64
		go runSystemdWatchdog(ctx, 5*time.Second, func(unset bool, state string) (bool, error) {
			if unset || state != daemon.SdNotifyWatchdog {
				t.Errorf("notify(%v, %q), want (false, %q)", unset, state, daemon.SdNotifyWatchdog)
			}
			switch calls.Add(1) {
			case 1:
				return false, errors.New("temporary send failure")
			case 2:
				return false, nil
			default:
				return true, nil
			}
		})
		synctest.Wait()

		for want := int64(1); want <= 4; want++ {
			time.Sleep(5 * time.Second)
			synctest.Wait()
			if got := calls.Load(); got != want {
				t.Fatalf("heartbeat calls after %s = %d, want %d", time.Duration(want)*5*time.Second, got, want)
			}
		}
		cancel()
		synctest.Wait()
		time.Sleep(20 * time.Second)
		synctest.Wait()
		if got := calls.Load(); got != 4 {
			t.Fatalf("heartbeats continued after cancellation: %d calls", got)
		}
	})
}
