package notifications

import (
	"context"
	"encoding/json"
	"fmt"
)

type Severity string

const (
	SeverityInfo     Severity = "info"
	SeverityWarning  Severity = "warning"
	SeverityCritical Severity = "critical"
)

type Message struct {
	Subject  string
	Body     string
	HTMLBody string
	Severity Severity
	RuleType string
	RuleName string
	URL      string
	Endpoint string
}

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
	default:
		return nil, fmt.Errorf("unknown channel type: %s", channelType)
	}
}
