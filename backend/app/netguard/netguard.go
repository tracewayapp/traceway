// Package netguard holds the shared private-address egress guard used by
// notification adapters and synthetic checks. Validation happens both at
// save time (friendly errors) and at dial time (DNS-rebinding safe).
package netguard

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/url"
	"strings"
	"time"
)

// ValidateURL rejects an http(s) destination that resolves to the server's
// own network. Callers decide whether the guard applies (allowPrivate=true
// short-circuits).
func ValidateURL(raw string, allowPrivate bool) error {
	if allowPrivate {
		return nil
	}
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return errors.New("The URL is not valid.")
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return errors.New("The URL must start with http:// or https://.")
	}
	host := parsed.Hostname()
	if host == "" {
		return errors.New("The URL is missing a host.")
	}
	return ValidateHost(host)
}

// ValidateHost resolves a hostname and rejects it when any address is
// private/loopback/link-local/CGNAT.
func ValidateHost(host string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	addrs, err := net.DefaultResolver.LookupIPAddr(ctx, host)
	if err != nil || len(addrs) == 0 {
		return fmt.Errorf("The host %q could not be resolved.", host)
	}
	for _, addr := range addrs {
		if IsPrivateIP(addr.IP) {
			return errors.New("The URL must point at a public address.")
		}
	}
	return nil
}

// GuardedDialContext returns a DialContext that re-checks the resolved
// address at connection time, so a hostname cannot pass save-time validation
// and then be re-pointed at a private address (DNS rebinding).
func GuardedDialContext(timeout time.Duration) func(ctx context.Context, network, addr string) (net.Conn, error) {
	dialer := &net.Dialer{Timeout: timeout}
	return func(ctx context.Context, network, addr string) (net.Conn, error) {
		host, port, err := net.SplitHostPort(addr)
		if err != nil {
			return nil, err
		}
		addrs, err := net.DefaultResolver.LookupIPAddr(ctx, host)
		if err != nil || len(addrs) == 0 {
			return nil, fmt.Errorf("could not resolve %q", host)
		}
		for _, resolved := range addrs {
			if IsPrivateIP(resolved.IP) {
				return nil, fmt.Errorf("destination %q resolves to a private address", host)
			}
		}
		// Dial the vetted address directly so the connection cannot be
		// re-resolved to something else.
		return dialer.DialContext(ctx, network, net.JoinHostPort(addrs[0].IP.String(), port))
	}
}

func IsPrivateIP(ip net.IP) bool {
	if ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() ||
		ip.IsUnspecified() || ip.IsInterfaceLocalMulticast() {
		return true
	}
	// 100.64.0.0/10 (CGNAT) and 0.0.0.0/8, which net.IP does not classify.
	if v4 := ip.To4(); v4 != nil {
		return v4[0] == 0 || (v4[0] == 100 && v4[1] >= 64 && v4[1] <= 127)
	}
	return false
}
