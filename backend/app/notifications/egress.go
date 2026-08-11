package notifications

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/url"
	"strings"
	"time"

	"github.com/tracewayapp/traceway/backend/app/config"
)

// ValidateOutboundURL rejects a destination on the server's own network, unless
// ALLOW_PRIVATE_NOTIFICATION_TARGETS is set. It guards self-scoped surfaces
// (personal contact methods); project webhook channels are deliberately not
// guarded.
func ValidateOutboundURL(raw string) error {
	if config.Config.AllowPrivateNotificationTargets == "true" {
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

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	addrs, err := net.DefaultResolver.LookupIPAddr(ctx, host)
	if err != nil || len(addrs) == 0 {
		return fmt.Errorf("The host %q could not be resolved.", host)
	}
	for _, addr := range addrs {
		if isPrivateIP(addr.IP) {
			return errors.New("The URL must point at a public address.")
		}
	}
	return nil
}

func isPrivateIP(ip net.IP) bool {
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
