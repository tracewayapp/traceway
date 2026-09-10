package slack

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/slack-go/slack"
	"github.com/tracewayapp/traceway/backend/app/agent"
	"github.com/tracewayapp/traceway/backend/app/models"
)

const maxMirrorChars = 3500

// threadRef names a Slack message as "<channel>/<ts>"; a bare channel is a
// place to post without a parent message.
func threadRef(channel string, ts string) string {
	if ts == "" {
		return channel
	}
	return channel + "/" + ts
}

func splitRef(ref string) (channel string, ts string) {
	channel, ts, _ = strings.Cut(ref, "/")
	return channel, ts
}

// Open posts the attempt's first message: under the alert or mention it
// started from when the origin is a Slack message, otherwise as a new
// message in the origin's channel or the integration's default channel.
func (a *App) CanOpenDefaultThread() bool { return true }

func (a *App) Open(ctx context.Context, in *models.Integration, attempt *models.AgentAttempt, origin *agent.Link) (agent.Link, error) {
	s, err := a.settings(in)
	if err != nil {
		return agent.Link{}, err
	}
	api := a.api(s)
	channel, parentTs := s.Channel, ""
	if origin != nil && origin.Provider == Provider {
		if originChannel, originTs := splitRef(origin.ExternalRef); originChannel != "" {
			channel, parentTs = originChannel, originTs
		}
	}
	text := fmt.Sprintf("*Attempt %d* started for %s.\n<%s|Open the run page>", attempt.Number, subjectLabel(attempt), a.runURL(attempt))
	options := []slack.MsgOption{slack.MsgOptionText(text, false)}
	if parentTs != "" {
		options = append(options, slack.MsgOptionTS(parentTs))
	}
	postedChannel, ts, err := api.PostMessageContext(ctx, channel, options...)
	if err != nil {
		return agent.Link{}, fmt.Errorf("slack: open thread in %s: %w", channel, err)
	}
	threadTs := parentTs
	if threadTs == "" {
		threadTs = ts
	}
	return agent.Link{
		Provider:      Provider,
		Kind:          models.LinkKindThread,
		ExternalRef:   threadRef(postedChannel, threadTs),
		URL:           a.permalink(ctx, api, postedChannel, threadTs),
		IntegrationId: in.Id,
	}, nil
}

// Post mirrors one thread entry into the Slack thread.
func (a *App) Post(ctx context.Context, in *models.Integration, thread agent.Link, m agent.Message) (agent.Link, error) {
	s, err := a.settings(in)
	if err != nil {
		return agent.Link{}, err
	}
	channel, ts := splitRef(thread.ExternalRef)
	if channel == "" || ts == "" {
		return agent.Link{}, errors.New("slack: thread link has no message")
	}
	_, _, err = a.api(s).PostMessageContext(ctx, channel, slack.MsgOptionTS(ts), slack.MsgOptionText(mirrorText(m), false))
	if err != nil {
		return agent.Link{}, fmt.Errorf("slack: post in %s: %w", thread.ExternalRef, err)
	}
	return thread, nil
}

// mirrorText renders a thread entry for Slack: the agent's own entries by
// kind, everyone else's with the surface they came from.
func mirrorText(m agent.Message) string {
	body := truncate(escape(m.Body), maxMirrorChars)
	if m.Provider == agent.ProviderAgent {
		switch m.Kind {
		case models.MessageKindQuestion:
			return ":question: *The agent needs an answer.* Reply in this thread.\n" + body
		case models.MessageKindFinding:
			return ":mag: *Finding*\n" + body
		case models.MessageKindPR:
			return ":rocket: " + body
		}
		return body
	}
	who := m.Provider
	if m.Author != nil && m.Author.Display != "" {
		who = m.Author.Display + " via " + m.Provider
	}
	return fmt.Sprintf("_%s_\n%s", escape(who), body)
}

func (a *App) runURL(attempt *models.AgentAttempt) string {
	path := fmt.Sprintf("/agent/%s?projectId=%s", attempt.Id, attempt.ProjectId)
	if base := a.dashboardURL(); base != "" {
		return base + path
	}
	return path
}

func subjectLabel(attempt *models.AgentAttempt) string {
	if attempt.SubjectKind == models.SubjectKindTracewayException {
		return "issue `" + attempt.SubjectRef + "`"
	}
	return escape(attempt.SubjectRef)
}

var escaper = strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;")

func escape(text string) string {
	return escaper.Replace(text)
}

func truncate(text string, limit int) string {
	if len(text) <= limit {
		return text
	}
	cut := limit
	for cut > 0 && cut < len(text) && text[cut]&0xC0 == 0x80 {
		cut--
	}
	return text[:cut] + "…"
}

// RequestApproval posts the pending attempt to the default channel with
// Approve and Dismiss buttons; the message is the attempt's Slack thread,
// so the run's progress lands under it once someone approves.
func (a *App) RequestApproval(ctx context.Context, in *models.Integration, attempt *models.AgentAttempt) (agent.Link, error) {
	s, err := a.settings(in)
	if err != nil {
		return agent.Link{}, err
	}
	api := a.api(s)
	value, _ := json.Marshal(approvalTarget{AttemptId: attempt.Id.String()})
	text := fmt.Sprintf("*Attempt %d* on %s is waiting for approval.\n<%s|Open the run page>", attempt.Number, subjectLabel(attempt), a.runURL(attempt))
	blocks := []slack.Block{
		slack.NewSectionBlock(slack.NewTextBlockObject(slack.MarkdownType, text, false, false), nil, nil),
		slack.NewActionBlock("traceway_approval",
			slack.NewButtonBlockElement(actionApprove, string(value), slack.NewTextBlockObject(slack.PlainTextType, "Approve", false, false)).WithStyle(slack.StylePrimary),
			slack.NewButtonBlockElement(actionDismiss, string(value), slack.NewTextBlockObject(slack.PlainTextType, "Dismiss", false, false)).WithStyle(slack.StyleDanger),
		),
	}
	channel, ts, err := api.PostMessageContext(ctx, s.Channel, slack.MsgOptionText(text, false), slack.MsgOptionBlocks(blocks...))
	if err != nil {
		return agent.Link{}, fmt.Errorf("slack: request approval in %s: %w", s.Channel, err)
	}
	return agent.Link{Provider: Provider, Kind: models.LinkKindThread, ExternalRef: threadRef(channel, ts), URL: a.permalink(ctx, api, channel, ts), IntegrationId: in.Id}, nil
}

var _ agent.ApprovalRequester = (*App)(nil)
