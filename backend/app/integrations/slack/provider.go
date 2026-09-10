// Package slack is Slack as a surface of the fix agent: alerts with a Fix
// it button, threads mirroring an attempt's conversation, replies resuming
// it, and @mentions or /traceway starting attempts. One integration row is
// one Slack app in one workspace. Socket Mode carries the same payloads as
// the signed HTTP route, so a self-hosted instance needs no public URL.
package slack

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/slack-go/slack"
	"github.com/tracewayapp/traceway/backend/app/agent"
	"github.com/tracewayapp/traceway/backend/app/config"
	"github.com/tracewayapp/traceway/backend/app/models"
	"github.com/tracewayapp/traceway/backend/app/notifications"
	"github.com/tracewayapp/traceway/backend/app/secrets"
)

const (
	Provider       = "slack"
	defaultAPIBase = "https://slack.com/api/"
	requestTimeout = 15 * time.Second
)

var secretFields = []string{"botToken", "signingSecret", "appToken"}

var channelIdRe = regexp.MustCompile(`^[CGD][A-Z0-9]{6,}$`)

// App is the provider over one Slack API endpoint. APIBase and Client are
// overridden by tests; DashboardURL resolves the instance origin for the
// links Slack messages carry.
type App struct {
	APIBase      string
	Client       *http.Client
	DashboardURL func() string
}

func New() *App {
	return &App{
		APIBase:      defaultAPIBase,
		Client:       &http.Client{Timeout: requestTimeout},
		DashboardURL: func() string { return config.Config.PublicBaseURL() },
	}
}

// Register wires Slack into the agent registries and adds the slack_app
// notification channel type.
func Register(a *App) {
	agent.RegisterTrigger(triggerView{a})
	agent.RegisterChannel(a)
	notifications.RegisterAdapter(ChannelType, a.newAdapter)
}

func (a *App) Provider() string { return Provider }
func (a *App) Kinds() []string  { return []string{agent.KindChat, agent.KindTrigger} }

func (a *App) Fields() []agent.Field {
	return []agent.Field{
		{Key: "botToken", Label: "Bot token", Kind: agent.FieldSecret, Required: true,
			Help: "The xoxb- token from the app's OAuth & Permissions page after installing it from the manifest."},
		{Key: "signingSecret", Label: "Signing secret", Kind: agent.FieldSecret, Required: true,
			Help: "From Basic Information; every request Slack sends is checked against it."},
		{Key: "appToken", Label: "App-level token", Kind: agent.FieldSecret, Required: false,
			Help: "An xapp- token with connections:write turns on Socket Mode, so this instance needs no public URL. Leave empty to receive events on the inbound route instead."},
		{Key: "channel", Label: "Default channel ID", Kind: agent.FieldText, Required: true,
			Help: "Where attempts that did not start from an alert message get their thread (the C... id from the channel's details, not its name). Invite the app to it."},
	}
}

func (a *App) SetupFlow() *agent.SetupFlow { return nil }

func (a *App) Validate(cfg map[string]string) error {
	if !strings.HasPrefix(strings.TrimSpace(cfg["botToken"]), "xoxb-") {
		return errors.New("The bot token must start with xoxb-.")
	}
	if strings.TrimSpace(cfg["signingSecret"]) == "" {
		return errors.New("The signing secret is required.")
	}
	if appToken := strings.TrimSpace(cfg["appToken"]); appToken != "" && !strings.HasPrefix(appToken, "xapp-") {
		return errors.New("The app-level token must start with xapp-.")
	}
	if !channelIdRe.MatchString(strings.TrimSpace(cfg["channel"])) {
		return errors.New("The default channel must be a channel ID such as C0123456789.")
	}
	return nil
}

// settings is an integration's decrypted config.
type settings struct {
	BotToken      string `json:"botToken"`
	SigningSecret string `json:"signingSecret"`
	AppToken      string `json:"appToken"`
	Channel       string `json:"channel"`
}

func (a *App) settings(in *models.Integration) (settings, error) {
	decrypted, err := secrets.DecryptFields(json.RawMessage(in.Config), secretFields)
	if err != nil {
		return settings{}, err
	}
	var s settings
	if err := json.Unmarshal(decrypted, &s); err != nil {
		return settings{}, err
	}
	if s.BotToken == "" {
		return settings{}, errors.New("slack integration has no bot token")
	}
	return s, nil
}

func (a *App) api(s settings) *slack.Client {
	options := []slack.Option{slack.OptionAPIURL(a.APIBase), slack.OptionHTTPClient(a.Client)}
	if s.AppToken != "" {
		options = append(options, slack.OptionAppLevelToken(s.AppToken))
	}
	return slack.New(s.BotToken, options...)
}

func (a *App) dashboardURL() string {
	if a.DashboardURL == nil {
		return ""
	}
	return a.DashboardURL()
}

// Respond answers Slack's URL verification handshake for the HTTP route.
func (a *App) Respond(in *models.Integration, r *http.Request) ([]byte, bool, error) {
	env, err := a.parseRequest(in, r)
	if err != nil {
		return nil, false, err
	}
	if env.kind != kindChallenge {
		return nil, false, nil
	}
	response, err := json.Marshal(map[string]string{"challenge": env.challenge})
	return response, true, err
}

// Inbound is the Channel side of the HTTP route: replies in threads the app
// opened.
func (a *App) Inbound(ctx context.Context, in *models.Integration, r *http.Request) ([]agent.InboundMessage, error) {
	env, err := a.parseRequest(in, r)
	if err != nil {
		return nil, err
	}
	if env.kind != kindMessage {
		return nil, nil
	}
	return a.replies(ctx, in, env)
}

// triggerView is App as a TriggerSource: the two ports both declare Inbound
// with different results, so the trigger side is served by a view type.
type triggerView struct{ *App }

func (v triggerView) Inbound(ctx context.Context, in *models.Integration, r *http.Request) ([]agent.Request, error) {
	env, err := v.parseRequest(in, r)
	if err != nil {
		return nil, err
	}
	switch env.kind {
	case kindMention, kindSlash, kindAction:
		return v.requests(ctx, in, env)
	}
	return nil, nil
}

// dispatch runs one decoded delivery through both ports; Socket Mode uses
// it since its envelopes never pass through the HTTP route.
func (a *App) dispatch(ctx context.Context, in *models.Integration, env envelope) (agent.InboundResult, error) {
	var inbound agent.Inbound
	var err error
	switch env.kind {
	case kindMention, kindSlash, kindAction:
		inbound.Requests, err = a.requests(ctx, in, env)
	case kindMessage:
		inbound.Messages, err = a.replies(ctx, in, env)
	}
	if err != nil {
		return agent.InboundResult{}, err
	}
	return agent.Dispatch(ctx, Provider, in, inbound)
}

var (
	_ agent.Channel          = (*App)(nil)
	_ agent.TriggerSource    = triggerView{}
	_ agent.InboundResponder = (*App)(nil)
)
