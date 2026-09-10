package notifications

import (
	"encoding/json"

	"github.com/tracewayapp/traceway/backend/app/models"
)

// AgentChannelType is the notification channel that starts a fix-agent
// attempt instead of sending anything: a rule on new errors or regressions
// attached to it turns every fire into an attempt, queued or waiting for
// approval as its config says.
const AgentChannelType = "agent"

// AgentChannelConfig is the stored config of an agent channel.
type AgentChannelConfig struct {
	ProfileId *int   `json:"profileId,omitempty"`
	Approval  string `json:"approval"`
}

const (
	AgentApprovalAsk  = "ask"
	AgentApprovalAuto = "auto"
)

// AttemptStartResult says what the starter did with a fire.
type AttemptStartResult struct {
	Started  bool
	Pending  bool
	Existing bool
}

// AttemptStarter starts an attempt for a rule that targets an agent
// channel. It is implemented by the agent package and registered from
// cmd/run.go, the same indirection as PageOpener.
type AttemptStarter func(channelConfig json.RawMessage, rule *models.NotificationRuleWithChannel, msg Message) (AttemptStartResult, error)

var attemptStarter AttemptStarter

func RegisterAttemptStarter(starter AttemptStarter) {
	attemptStarter = starter
}

// ParseAgentChannelConfig validates an agent channel's config.
func ParseAgentChannelConfig(raw json.RawMessage) (AgentChannelConfig, string) {
	var cfg AgentChannelConfig
	if len(raw) > 0 {
		if err := json.Unmarshal(raw, &cfg); err != nil {
			return cfg, "The fix agent config is not valid JSON."
		}
	}
	if cfg.Approval == "" {
		cfg.Approval = AgentApprovalAsk
	}
	if cfg.Approval != AgentApprovalAsk && cfg.Approval != AgentApprovalAuto {
		return cfg, "Approval must be ask or auto."
	}
	if cfg.ProfileId != nil && *cfg.ProfileId <= 0 {
		cfg.ProfileId = nil
	}
	return cfg, ""
}
