package cmd

import (
	"context"
	"errors"
	"net"
	"os"
	"runtime"
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

func TestSystemdMigrationExtensionDatagram(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("systemd uses Linux datagram sockets")
	}
	t.Chdir(t.TempDir())
	socket := "notify.sock"
	listener, err := net.ListenUnixgram("unixgram", &net.UnixAddr{Net: "unixgram", Name: socket})
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	if err := extendSystemdStartup(socket, 3*time.Second); err != nil {
		t.Fatal(err)
	}
	if err := listener.SetReadDeadline(time.Now().Add(time.Second)); err != nil {
		t.Fatal(err)
	}
	var buf [64]byte
	n, err := listener.Read(buf[:])
	if err != nil || string(buf[:n]) != "EXTEND_TIMEOUT_USEC=3000000" {
		t.Fatalf("received %q, %v", buf[:n], err)
	}
}

func TestMigrationHeartbeatStopsAtBudget(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		start := time.Now()
		var windows []time.Duration
		stop := startMigrationHeartbeat(func(window time.Duration) error {
			if time.Since(start)+window > migrationExtensionBudget {
				t.Errorf("extension at %s grants %s past the budget", time.Since(start), window)
			}
			windows = append(windows, window)
			if len(windows) == 1 {
				if !time.Now().Equal(start) || window != migrationExtensionWindow {
					t.Errorf("first extension at %s grants %s", time.Since(start), window)
				}
				return errors.New("temporary send failure")
			}
			return nil
		})
		time.Sleep(migrationExtensionBudget + migrationExtensionWindow)
		stop()
		if want := int(migrationExtensionBudget / migrationHeartbeatInterval); len(windows) != want {
			t.Fatalf("sent %d extensions, want %d", len(windows), want)
		}
		if got := windows[len(windows)-1]; got != migrationHeartbeatInterval {
			t.Fatalf("last extension = %s, want %s", got, migrationHeartbeatInterval)
		}
	})
}

func TestMigrationHeartbeatJoinsBeforeReturning(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		var calls int
		stop := startMigrationHeartbeat(func(window time.Duration) error {
			time.Sleep(time.Second)
			calls++
			return nil
		})
		synctest.Wait()
		stop()
		if calls != 1 {
			t.Fatalf("stop returned before the in-flight send completed")
		}
		time.Sleep(2 * migrationHeartbeatInterval)
		synctest.Wait()
		if calls != 1 {
			t.Fatalf("sent %d extensions after stopping", calls-1)
		}
	})
}
