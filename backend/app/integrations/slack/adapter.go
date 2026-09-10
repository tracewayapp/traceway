package slack

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/slack-go/slack"
	"github.com/tracewayapp/traceway/backend/app/db"
	"github.com/tracewayapp/traceway/backend/app/models"
	"github.com/tracewayapp/traceway/backend/app/notifications"
	"github.com/tracewayapp/traceway/backend/app/repositories/transactional"
)

// ChannelType is the notification channel that posts alerts through a Slack
// integration as Block Kit messages with View, Fix it and Archive buttons.
// The incoming-webhook channel type "slack" stays as it is.
const ChannelType = "slack_app"

const (
	maxHeaderChars = 150
	maxBodyChars   = 2900
)

type adapter struct {
	app         *App
	Integration int    `json:"integrationId"`
	Channel     string `json:"channel"`
}

func (a *App) newAdapter(config json.RawMessage) (notifications.Adapter, error) {
	ad := &adapter{app: a}
	if err := json.Unmarshal(config, ad); err != nil {
		return nil, fmt.Errorf("invalid %s config: %w", ChannelType, err)
	}
	return ad, nil
}

func (ad *adapter) Type() string       { return ChannelType }
func (ad *adapter) IntegrationId() int { return ad.Integration }

func (ad *adapter) Validate() error {
	if ad.Integration <= 0 {
		return errors.New("Pick a Slack integration.")
	}
	if !channelIdRe.MatchString(strings.TrimSpace(ad.Channel)) {
		return errors.New("The channel must be a channel ID such as C0123456789.")
	}
	return nil
}

func (ad *adapter) Send(ctx context.Context, msg notifications.Message) error {
	in, err := db.ExecuteTransaction(func(tx *sql.Tx) (*models.Integration, error) {
		return transactional.IntegrationRepository.FindById(tx, ad.Integration)
	})
	if err != nil {
		return err
	}
	if in == nil || in.Provider != Provider {
		return fmt.Errorf("slack integration %d no longer exists", ad.Integration)
	}
	if !in.Enabled {
		return fmt.Errorf("slack integration %d is disabled", ad.Integration)
	}
	s, err := ad.app.settings(in)
	if err != nil {
		return err
	}
	_, _, err = ad.app.api(s).PostMessageContext(ctx, ad.Channel, slack.MsgOptionText(msg.Subject, false), slack.MsgOptionBlocks(alertBlocks(msg)...))
	if err != nil {
		return fmt.Errorf("slack chat.postMessage to %s: %w", ad.Channel, err)
	}
	return nil
}

// alertBlocks lays out a notification: header, body, a context line with
// severity and rule, and the buttons. Fix it and Archive exist only when
// the message names an issue in a project.
func alertBlocks(msg notifications.Message) []slack.Block {
	blocks := []slack.Block{
		slack.NewHeaderBlock(slack.NewTextBlockObject(slack.PlainTextType, truncate(msg.Subject, maxHeaderChars), false, false)),
	}
	if body := strings.TrimSpace(msg.Body); body != "" {
		blocks = append(blocks, slack.NewSectionBlock(slack.NewTextBlockObject(slack.MarkdownType, truncate(escape(body), maxBodyChars), false, false), nil, nil))
	}
	context := fmt.Sprintf("Severity: *%s*", msg.Severity)
	if msg.RuleName != "" {
		context += " · Rule: " + escape(msg.RuleName)
	}
	blocks = append(blocks, slack.NewContextBlock("", slack.NewTextBlockObject(slack.MarkdownType, context, false, false)))

	var buttons []slack.BlockElement
	if strings.HasPrefix(msg.URL, "http://") || strings.HasPrefix(msg.URL, "https://") {
		buttons = append(buttons, slack.NewButtonBlockElement(actionView, "", slack.NewTextBlockObject(slack.PlainTextType, "View", false, false)).WithURL(msg.URL))
	}
	if target := issueTargetOf(msg); target != nil {
		value, _ := json.Marshal(target)
		buttons = append(buttons,
			slack.NewButtonBlockElement(actionFixIt, string(value), slack.NewTextBlockObject(slack.PlainTextType, "Fix it", false, false)).WithStyle(slack.StylePrimary),
			slack.NewButtonBlockElement(actionArchive, string(value), slack.NewTextBlockObject(slack.PlainTextType, "Archive", false, false)),
		)
	}
	if len(buttons) > 0 {
		blocks = append(blocks, slack.NewActionBlock("traceway_actions", buttons...))
	}
	return blocks
}

func issueTargetOf(msg notifications.Message) *issueTarget {
	match := issuePathRe.FindStringSubmatch(msg.URL)
	if match == nil || msg.ProjectId == "" {
		return nil
	}
	return &issueTarget{ProjectId: msg.ProjectId, Hash: match[1]}
}
