package notifications

import (
	"github.com/tracewayapp/traceway/backend/app/config"
	"github.com/tracewayapp/traceway/backend/app/netguard"
)

// ValidateOutboundURL rejects a destination on the server's own network, unless
// ALLOW_PRIVATE_NOTIFICATION_TARGETS is set. It guards self-scoped surfaces
// (personal contact methods); project webhook channels are deliberately not
// guarded. The shared implementation lives in netguard, which synthetic
// checks use with their own SYNTHETICS_ALLOW_PRIVATE_TARGETS switch.
func ValidateOutboundURL(raw string) error {
	return netguard.ValidateURL(raw, config.Config.AllowPrivateNotificationTargets == "true")
}
