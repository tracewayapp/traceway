// Package web is the dashboard as an agent surface: the Fix it button is
// its trigger and the run page's thread is its channel. It has no
// integration row, no fields and no inbound route; the controllers call the
// agent package directly and the thread lives in agent_messages.
package web

import (
	"context"
	"errors"
	"net/http"

	"github.com/tracewayapp/traceway/backend/app/agent"
	"github.com/tracewayapp/traceway/backend/app/models"
)

const Provider = "web"

type Surface struct{}

func (Surface) Provider() string            { return Provider }
func (Surface) Kinds() []string             { return []string{agent.KindChat, agent.KindTrigger} }
func (Surface) Fields() []agent.Field       { return nil }
func (Surface) SetupFlow() *agent.SetupFlow { return nil }
func (Surface) Validate(map[string]string) error {
	return nil
}

var errNoInbound = errors.New("the dashboard has no inbound route; its controllers call the agent package directly")

func (Surface) Inbound(context.Context, *models.Integration, *http.Request) ([]agent.Request, error) {
	return nil, errNoInbound
}

// Open names the run page as the thread. The dashboard reads agent_messages
// directly, so the link only records where the conversation is visible.
func (Surface) Open(_ context.Context, _ *models.Integration, attempt *models.AgentAttempt, _ *agent.Link) (agent.Link, error) {
	return agent.Link{Provider: Provider, Kind: models.LinkKindThread, ExternalRef: attempt.Id.String(), URL: "/agent/" + attempt.Id.String()}, nil
}

// Post has nothing to deliver: the message row is the dashboard's thread.
func (Surface) Post(_ context.Context, _ *models.Integration, thread agent.Link, _ agent.Message) (agent.Link, error) {
	return thread, nil
}

func (Surface) InboundMessages(context.Context, *models.Integration, *http.Request) ([]agent.InboundMessage, error) {
	return nil, errNoInbound
}

var (
	_ agent.TriggerSource = Surface{}
	_ agent.Channel       = channelView{}
)

// channelView is Surface as a Channel: the two ports both declare Inbound
// with different results, so the channel side is served by a view type.
type channelView struct{ Surface }

func (v channelView) Inbound(ctx context.Context, in *models.Integration, r *http.Request) ([]agent.InboundMessage, error) {
	return v.Surface.InboundMessages(ctx, in, r)
}

// Register wires the dashboard into the agent registries.
func Register() {
	agent.RegisterTrigger(Surface{})
	agent.RegisterChannel(channelView{})
}
