package notifications

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"sync"

	"github.com/tracewayapp/traceway/backend/app/models"
)

// Message and Severity live in models (as NotificationMessage /
// NotificationSeverity) so the outbox package can persist them without
// importing this package; the aliases keep every call site unchanged.
type Severity = models.NotificationSeverity

const (
	SeverityInfo     = models.NotificationSeverityInfo
	SeverityWarning  = models.NotificationSeverityWarning
	SeverityCritical = models.NotificationSeverityCritical
)

type Message = models.NotificationMessage

type Adapter interface {
	Type() string
	Send(ctx context.Context, msg Message) error
	Validate() error
}

// IntegrationBound is an adapter that delivers through an integration row
// of the project's organization; the channel controller checks that the row
// exists there and is enabled before the config is accepted.
type IntegrationBound interface {
	IntegrationId() int
}

// AdapterFactory builds an adapter from a stored channel config.
type AdapterFactory func(config json.RawMessage) (Adapter, error)

var (
	registeredMu sync.RWMutex
	registered   = map[string]AdapterFactory{}
)

// RegisterAdapter adds a channel type owned by another package (an
// integration provider's app adapter) so NewAdapter, the outbox and the
// channel controller treat it like the built-in ones.
func RegisterAdapter(channelType string, factory AdapterFactory) {
	registeredMu.Lock()
	defer registeredMu.Unlock()
	registered[channelType] = factory
}

// RegisteredAdapterTypes lists the channel types added through RegisterAdapter.
func RegisteredAdapterTypes() []string {
	registeredMu.RLock()
	defer registeredMu.RUnlock()
	out := make([]string, 0, len(registered))
	for name := range registered {
		out = append(out, name)
	}
	sort.Strings(out)
	return out
}

// NewAdapter builds the adapter for a stored config. Credentials rest
// encrypted, so this is the one place they are decrypted: the outbox drain,
// the test buttons and config validation all pass through here.
func NewAdapter(channelType string, configJSON json.RawMessage) (Adapter, error) {
	configJSON, err := DecryptSecretFields(channelType, configJSON)
	if err != nil {
		return nil, fmt.Errorf("invalid %s config: %w", channelType, err)
	}
	registeredMu.RLock()
	factory, ok := registered[channelType]
	registeredMu.RUnlock()
	if ok {
		return factory(configJSON)
	}
	switch channelType {
	case "email":
		var cfg EmailAdapter
		if err := json.Unmarshal(configJSON, &cfg); err != nil {
			return nil, fmt.Errorf("invalid email config: %w", err)
		}
		return &cfg, nil
	case "webhook":
		var cfg WebhookAdapter
		if err := json.Unmarshal(configJSON, &cfg); err != nil {
			return nil, fmt.Errorf("invalid webhook config: %w", err)
		}
		return &cfg, nil
	case "slack":
		var cfg SlackAdapter
		if err := json.Unmarshal(configJSON, &cfg); err != nil {
			return nil, fmt.Errorf("invalid slack config: %w", err)
		}
		return &cfg, nil
	case "github":
		var cfg GitHubAdapter
		if err := json.Unmarshal(configJSON, &cfg); err != nil {
			return nil, fmt.Errorf("invalid github config: %w", err)
		}
		return &cfg, nil
	case "pushover":
		var cfg PushoverAdapter
		if err := json.Unmarshal(configJSON, &cfg); err != nil {
			return nil, fmt.Errorf("invalid pushover config: %w", err)
		}
		return &cfg, nil
	case "telegram":
		var cfg TelegramAdapter
		if err := json.Unmarshal(configJSON, &cfg); err != nil {
			return nil, fmt.Errorf("invalid telegram config: %w", err)
		}
		return &cfg, nil
	case "sms":
		var cfg SmsAdapter
		if err := json.Unmarshal(configJSON, &cfg); err != nil {
			return nil, fmt.Errorf("invalid sms config: %w", err)
		}
		return &cfg, nil
	default:
		return nil, fmt.Errorf("unknown channel type: %s", channelType)
	}
}
