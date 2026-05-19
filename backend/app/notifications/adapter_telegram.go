package notifications

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"
)

type TelegramAdapter struct {
	BotToken string `json:"botToken"`
	ChatId   string `json:"chatId"`
}

var telegramBotTokenRe = regexp.MustCompile(`^\d+:[A-Za-z0-9_-]+$`)

func (a *TelegramAdapter) Type() string { return "telegram" }

func (a *TelegramAdapter) Validate() error {
	token := strings.TrimSpace(a.BotToken)
	if token == "" {
		return fmt.Errorf("Telegram bot token is required")
	}
	if !telegramBotTokenRe.MatchString(token) {
		return fmt.Errorf("Telegram bot token format looks invalid (expected '<id>:<secret>')")
	}
	if strings.TrimSpace(a.ChatId) == "" {
		return fmt.Errorf("Telegram chat ID is required")
	}
	return nil
}

func (a *TelegramAdapter) Send(ctx context.Context, msg Message) error {
	var text strings.Builder
	if msg.Subject != "" {
		text.WriteString(msg.Subject)
		text.WriteString("\n\n")
	}
	text.WriteString(msg.Body)
	if msg.URL != "" {
		text.WriteString("\n\n")
		text.WriteString(msg.URL)
	}

	form := url.Values{
		"chat_id":                  {a.ChatId},
		"text":                     {text.String()},
		"disable_web_page_preview": {"true"},
	}

	endpoint := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", url.PathEscape(strings.TrimSpace(a.BotToken)))
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return fmt.Errorf("failed to create Telegram request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("Telegram request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)

	if resp.StatusCode == http.StatusOK {
		var result struct {
			Ok bool `json:"ok"`
		}
		if err := json.Unmarshal(respBody, &result); err == nil && result.Ok {
			return nil
		}
		return fmt.Errorf("Telegram returned unexpected response: %s", string(respBody))
	}

	var errResult struct {
		Ok          bool   `json:"ok"`
		ErrorCode   int    `json:"error_code"`
		Description string `json:"description"`
	}
	if err := json.Unmarshal(respBody, &errResult); err == nil && errResult.Description != "" {
		return fmt.Errorf("Telegram returned %d: %s", resp.StatusCode, errResult.Description)
	}

	return fmt.Errorf("Telegram returned status %d", resp.StatusCode)
}
