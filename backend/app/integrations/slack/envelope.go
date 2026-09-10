package slack

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/slack-go/slack"
	"github.com/slack-go/slack/slackevents"
	"github.com/slack-go/slack/socketmode"
	"github.com/tracewayapp/traceway/backend/app/models"
)

const maxInboundBytes = 1 << 20

const (
	kindChallenge = "challenge"
	kindMention   = "mention"
	kindMessage   = "message"
	kindAction    = "action"
	kindSlash     = "slash"
	kindIgnored   = "ignored"
)

// envelope is one Slack delivery reduced to what the handlers need, the
// same whether it arrived signed over HTTP or as a Socket Mode frame.
type envelope struct {
	kind        string
	challenge   string
	userId      string
	channel     string
	ts          string
	threadTs    string
	text        string
	fromBot     bool
	actionId    string
	actionValue string
}

var errBadSignature = errors.New("slack: request signature rejected")

// parseRequest verifies the signature with the integration's own secret
// and decodes the payload. The body is put back so every port of the
// provider can read the same request.
func (a *App) parseRequest(in *models.Integration, r *http.Request) (envelope, error) {
	s, err := a.settings(in)
	if err != nil {
		return envelope{}, err
	}
	body, err := io.ReadAll(io.LimitReader(r.Body, maxInboundBytes+1))
	if err != nil {
		return envelope{}, err
	}
	r.Body = io.NopCloser(bytes.NewReader(body))
	if len(body) > maxInboundBytes {
		return envelope{}, errors.New("slack: request body too large")
	}
	verifier, err := slack.NewSecretsVerifier(r.Header, s.SigningSecret)
	if err != nil {
		return envelope{}, fmt.Errorf("%w: %v", errBadSignature, err)
	}
	if _, err := verifier.Write(body); err != nil {
		return envelope{}, err
	}
	if err := verifier.Ensure(); err != nil {
		return envelope{}, fmt.Errorf("%w: %v", errBadSignature, err)
	}
	if strings.HasPrefix(r.Header.Get("Content-Type"), "application/x-www-form-urlencoded") {
		return parseForm(body)
	}
	return parseEventsAPI(body)
}

func parseForm(body []byte) (envelope, error) {
	form, err := url.ParseQuery(string(body))
	if err != nil {
		return envelope{}, err
	}
	if payload := form.Get("payload"); payload != "" {
		var callback slack.InteractionCallback
		if err := json.Unmarshal([]byte(payload), &callback); err != nil {
			return envelope{}, err
		}
		return fromInteraction(callback), nil
	}
	if form.Get("command") != "" {
		return fromSlash(slack.SlashCommand{
			Command:   form.Get("command"),
			Text:      form.Get("text"),
			UserID:    form.Get("user_id"),
			ChannelID: form.Get("channel_id"),
		}), nil
	}
	return envelope{kind: kindIgnored}, nil
}

func parseEventsAPI(body []byte) (envelope, error) {
	event, err := slackevents.ParseEvent(body, slackevents.OptionNoVerifyToken())
	if err != nil {
		return envelope{}, err
	}
	return fromEventsAPI(event), nil
}

func fromEventsAPI(event slackevents.EventsAPIEvent) envelope {
	switch event.Type {
	case slackevents.URLVerification:
		verification, ok := event.Data.(*slackevents.EventsAPIURLVerificationEvent)
		if !ok {
			return envelope{kind: kindIgnored}
		}
		return envelope{kind: kindChallenge, challenge: verification.Challenge}
	case slackevents.CallbackEvent:
		switch inner := event.InnerEvent.Data.(type) {
		case *slackevents.AppMentionEvent:
			return envelope{kind: kindMention, userId: inner.User, channel: inner.Channel, ts: inner.TimeStamp, threadTs: inner.ThreadTimeStamp, text: inner.Text, fromBot: inner.BotID != ""}
		case *slackevents.MessageEvent:
			return envelope{kind: kindMessage, userId: inner.User, channel: inner.Channel, ts: inner.TimeStamp, threadTs: inner.ThreadTimeStamp, text: inner.Text, fromBot: inner.BotID != "" || inner.SubType != ""}
		}
	}
	return envelope{kind: kindIgnored}
}

func fromInteraction(callback slack.InteractionCallback) envelope {
	if callback.Type != slack.InteractionTypeBlockActions || len(callback.ActionCallback.BlockActions) == 0 {
		return envelope{kind: kindIgnored}
	}
	action := callback.ActionCallback.BlockActions[0]
	return envelope{
		kind:        kindAction,
		userId:      callback.User.ID,
		channel:     callback.Channel.ID,
		ts:          callback.Message.Timestamp,
		threadTs:    callback.Message.ThreadTimestamp,
		actionId:    action.ActionID,
		actionValue: action.Value,
	}
}

func fromSlash(command slack.SlashCommand) envelope {
	return envelope{kind: kindSlash, userId: command.UserID, channel: command.ChannelID, text: command.Text}
}

// fromSocket maps a Socket Mode frame onto the envelope the HTTP path
// produces for the same payload.
func fromSocket(event socketmode.Event) (envelope, bool) {
	switch event.Type {
	case socketmode.EventTypeEventsAPI:
		payload, ok := event.Data.(slackevents.EventsAPIEvent)
		if !ok {
			return envelope{}, false
		}
		return fromEventsAPI(payload), true
	case socketmode.EventTypeInteractive:
		callback, ok := event.Data.(slack.InteractionCallback)
		if !ok {
			return envelope{}, false
		}
		return fromInteraction(callback), true
	case socketmode.EventTypeSlashCommand:
		command, ok := event.Data.(slack.SlashCommand)
		if !ok {
			return envelope{}, false
		}
		return fromSlash(command), true
	}
	return envelope{}, false
}
