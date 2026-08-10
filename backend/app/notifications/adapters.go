package notifications

import (
	"context"
	"encoding/json"
	"fmt"

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

func NewAdapter(channelType string, configJSON json.RawMessage) (Adapter, error) {
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
