package synthetics

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net"
	"strings"
	"time"

	"github.com/tracewayapp/traceway/backend/app/models"
	"github.com/tracewayapp/traceway/backend/app/netguard"
)

const maxTcpReadBytes = 64 << 10

func probeTcp(ctx context.Context, check *models.SyntheticCheck) Outcome {
	outcome := Outcome{Status: models.CheckResultDown, TlsDaysRemaining: -1}

	var cfg models.TcpCheckConfig
	if err := json.Unmarshal(check.Config, &cfg); err != nil {
		outcome.ErrorMsg = "invalid check config: " + err.Error()
		return outcome
	}
	if cfg.Host == "" || cfg.Port < 1 || cfg.Port > 65535 {
		outcome.ErrorMsg = "invalid host or port"
		return outcome
	}

	address := net.JoinHostPort(cfg.Host, fmt.Sprintf("%d", cfg.Port))
	timeout := EffectiveTimeout(check)
	dial := (&net.Dialer{Timeout: timeout}).DialContext
	if !AllowPrivateTargets() {
		// Vet the resolved address and dial exactly that address, like the
		// http probe: a validate-then-redial-by-hostname sequence would leave
		// a DNS-rebinding window between the check and the connection.
		dial = netguard.GuardedDialContext(timeout)
	}

	started := time.Now()
	conn, err := dial(ctx, "tcp", address)
	if err != nil {
		outcome.LatencyMs = float64(time.Since(started).Microseconds()) / 1000
		outcome.ErrorMsg = err.Error()
		return outcome
	}
	defer conn.Close()

	if deadline, ok := ctx.Deadline(); ok {
		_ = conn.SetDeadline(deadline)
	}

	if cfg.UseTls {
		tlsConn := tls.Client(conn, &tls.Config{ServerName: cfg.Host})
		if err := tlsConn.HandshakeContext(ctx); err != nil {
			outcome.LatencyMs = float64(time.Since(started).Microseconds()) / 1000
			outcome.ErrorMsg = "TLS handshake failed: " + err.Error()
			return outcome
		}
		state := tlsConn.ConnectionState()
		if len(state.PeerCertificates) > 0 {
			outcome.TlsDaysRemaining = daysUntil(state.PeerCertificates[0].NotAfter)
		}
		conn = tlsConn
	}

	if cfg.SendPayload != "" {
		if _, err := conn.Write([]byte(cfg.SendPayload)); err != nil {
			outcome.LatencyMs = float64(time.Since(started).Microseconds()) / 1000
			outcome.ErrorMsg = "write failed: " + err.Error()
			return outcome
		}
	}
	if cfg.ExpectContains != "" {
		received, matched := readUntilMatch(conn, cfg.ExpectContains)
		if !matched {
			outcome.LatencyMs = float64(time.Since(started).Microseconds()) / 1000
			preview := received
			if len(preview) > 200 {
				preview = preview[:200]
			}
			outcome.ErrorMsg = fmt.Sprintf("response does not contain %q (got %q)", cfg.ExpectContains, preview)
			return outcome
		}
	}

	outcome.LatencyMs = float64(time.Since(started).Microseconds()) / 1000

	if cfg.TlsMinDaysRemaining > 0 {
		if outcome.TlsDaysRemaining < 0 {
			outcome.ErrorMsg = "TLS expiry assertion set but the check does not use TLS"
			return outcome
		}
		if outcome.TlsDaysRemaining < cfg.TlsMinDaysRemaining {
			outcome.ErrorMsg = fmt.Sprintf("TLS certificate expires in %d days (minimum %d)", outcome.TlsDaysRemaining, cfg.TlsMinDaysRemaining)
			return outcome
		}
	}

	outcome.Status = models.CheckResultUp
	return outcome
}

// readUntilMatch reads until the buffer contains the expected substring, the
// connection closes, the deadline passes, or the read cap is hit.
func readUntilMatch(conn net.Conn, expect string) (string, bool) {
	var builder strings.Builder
	buf := make([]byte, 4096)
	for builder.Len() < maxTcpReadBytes {
		n, err := conn.Read(buf)
		if n > 0 {
			builder.Write(buf[:n])
			if strings.Contains(builder.String(), expect) {
				return builder.String(), true
			}
		}
		if err != nil {
			break
		}
	}
	return builder.String(), strings.Contains(builder.String(), expect)
}
