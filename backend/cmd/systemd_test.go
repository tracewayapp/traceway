package cmd

import (
	"fmt"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"
)

const readTimeout = 2 * time.Second

// notifySocket binds a unixgram socket and points NOTIFY_SOCKET at it, the way
// systemd does for a Type=notify unit.
func notifySocket(t *testing.T) *net.UnixConn {
	t.Helper()

	// Keep test names short: the socket path has to fit sun_path's ~107 bytes.
	path := filepath.Join(t.TempDir(), "n")
	conn, err := net.ListenUnixgram("unixgram", &net.UnixAddr{Name: path, Net: "unixgram"})
	if err != nil {
		t.Fatalf("bind %s: %v", path, err)
	}
	t.Cleanup(func() { conn.Close() })

	t.Setenv("NOTIFY_SOCKET", path)
	return conn
}

func readMessage(t *testing.T, conn *net.UnixConn) string {
	t.Helper()

	message, ok := readMessageWithin(t, conn, readTimeout)
	if !ok {
		t.Fatalf("no message arrived within %s", readTimeout)
	}
	return message
}

func expectNoMessage(t *testing.T, conn *net.UnixConn, within time.Duration) {
	t.Helper()

	if message, ok := readMessageWithin(t, conn, within); ok {
		t.Fatalf("unexpected message %q", message)
	}
}

func readMessageWithin(t *testing.T, conn *net.UnixConn, within time.Duration) (string, bool) {
	t.Helper()

	if err := conn.SetReadDeadline(time.Now().Add(within)); err != nil {
		t.Fatalf("set deadline: %v", err)
	}
	buf := make([]byte, 256)
	n, err := conn.Read(buf)
	if err != nil {
		return "", false
	}
	return string(buf[:n]), true
}

func TestBootHeartbeatKeepsExtendingTheStartTimeout(t *testing.T) {
	conn := notifySocket(t)

	stop := startBootHeartbeat(20*time.Millisecond, 5*time.Second, time.Minute)
	defer stop()

	// The first extension goes out before Run() touches the database, so a boot
	// that stalls immediately is still covered.
	want := "EXTEND_TIMEOUT_USEC=5000000"
	for i := range 3 {
		if got := readMessage(t, conn); got != want {
			t.Fatalf("message %d = %q, want %q", i, got, want)
		}
	}
}

func TestNothingExtendsTheStartTimeoutAfterReady(t *testing.T) {
	conn := notifySocket(t)

	ready := beginBoot(20*time.Millisecond, 5*time.Second, time.Minute)
	readMessage(t, conn)
	ready()

	// Anything after READY=1 extends the *watchdog* deadline instead of the
	// startup one, so a heartbeat outliving ready() would hide a hung process.
	for {
		message := readMessage(t, conn)
		if message == "READY=1" {
			break
		}
		if message != "EXTEND_TIMEOUT_USEC=5000000" {
			t.Fatalf("unexpected message before READY=1: %q", message)
		}
	}
	expectNoMessage(t, conn, 100*time.Millisecond)

	// A second call must neither re-report ready nor start a second watchdog.
	ready()
	expectNoMessage(t, conn, 100*time.Millisecond)
}

func TestBootHeartbeatStopsExtendingOnceTheBudgetIsSpent(t *testing.T) {
	conn := notifySocket(t)

	// A boot wedged against an unreachable database must not hold systemd's
	// start deadline open forever.
	stop := startBootHeartbeat(10*time.Millisecond, 5*time.Second, 50*time.Millisecond)
	defer stop()

	readMessage(t, conn)
	deadline := time.Now().Add(readTimeout)
	for {
		if _, ok := readMessageWithin(t, conn, 150*time.Millisecond); !ok {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("heartbeat still extending well past its budget")
		}
	}
}

func TestBootHeartbeatSurvivesAFailedFirstSend(t *testing.T) {
	// NOTIFY_SOCKET names a socket nothing has bound yet, so the first send
	// fails. Reading "no service manager" into that failure would abandon the
	// heartbeat and drop boot back to the default TimeoutStartSec.
	path := filepath.Join(t.TempDir(), "n")
	t.Setenv("NOTIFY_SOCKET", path)

	stop := startBootHeartbeat(20*time.Millisecond, 5*time.Second, time.Minute)
	defer stop()

	conn, err := net.ListenUnixgram("unixgram", &net.UnixAddr{Name: path, Net: "unixgram"})
	if err != nil {
		t.Fatalf("bind %s: %v", path, err)
	}
	defer conn.Close()

	if got := readMessage(t, conn); got != "EXTEND_TIMEOUT_USEC=5000000" {
		t.Fatalf("got %q, want the heartbeat to have retried", got)
	}
}

func TestBootHeartbeatIsANoOpWithoutAServiceManager(t *testing.T) {
	t.Setenv("NOTIFY_SOCKET", "")

	done := make(chan struct{})
	go func() {
		defer close(done)
		beginBoot(bootExtendInterval, bootExtendWindow, bootExtendBudget)()
	}()

	select {
	case <-done:
	case <-time.After(readTimeout):
		t.Fatal("ready() blocked when NOTIFY_SOCKET is unset")
	}
}

func TestWatchdogPingIntervalIsHalfOfWhatSystemdAsksFor(t *testing.T) {
	tests := []struct {
		name    string
		usec    string
		pid     string
		want    time.Duration
		wantErr bool
	}{
		{name: "disabled", usec: "", want: 0},
		// The interval this replaces was hardcoded at 15s, so systemd would
		// have killed a healthy process one ping into a 10s watchdog.
		{name: "shorter than the old hardcoded interval", usec: "10000000", want: 5 * time.Second},
		{name: "longer than the old hardcoded interval", usec: "60000000", want: 30 * time.Second},
		{name: "watched pid is this process", usec: "30000000", pid: fmt.Sprint(os.Getpid()), want: 15 * time.Second},
		{name: "watched pid is another process", usec: "30000000", pid: fmt.Sprint(os.Getpid() + 1), want: 0},
		{name: "malformed", usec: "soon", wantErr: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Setenv("WATCHDOG_USEC", test.usec)
			t.Setenv("WATCHDOG_PID", test.pid)

			got, err := watchdogPingInterval()
			if test.wantErr {
				if err == nil {
					t.Fatalf("got %s, want an error", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != test.want {
				t.Fatalf("got %s, want %s", got, test.want)
			}
		})
	}
}
