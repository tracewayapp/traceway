package notifications

import (
	"encoding/json"

	"github.com/tracewayapp/traceway/backend/app/secrets"
)

// SecretSentinel is what a masked credential reads as on the wire. A form
// that sends it back is asking to keep the stored value.
const SecretSentinel = secrets.Sentinel

var channelSecretFields = map[string][]string{
	"webhook":  {"secret"},
	"slack":    {"webhookUrl"},
	"github":   {"token"},
	"pushover": {"userKey", "appToken"},
	"telegram": {"botToken"},
}

func SecretFields(channelType string) []string {
	return channelSecretFields[channelType]
}

func EncryptSecretFields(channelType string, cfg json.RawMessage) (json.RawMessage, error) {
	return secrets.EncryptFields(cfg, channelSecretFields[channelType])
}

func DecryptSecretFields(channelType string, cfg json.RawMessage) (json.RawMessage, error) {
	return secrets.DecryptFields(cfg, channelSecretFields[channelType])
}

func MaskSecretFields(channelType string, cfg json.RawMessage) (json.RawMessage, []string, error) {
	return secrets.MaskFields(cfg, channelSecretFields[channelType])
}

func KeepStoredSecrets(channelType string, incoming, stored json.RawMessage) (json.RawMessage, error) {
	return secrets.KeepStoredFields(incoming, stored, channelSecretFields[channelType])
}
