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
	"unicode/utf8"
)

// telegramMaxMessageLength is the Telegram Bot API's sendMessage text limit
// (4096 UTF-16 code units; treating it as a rune budget keeps a comfortable
// safety margin since nearly all runes are a single UTF-16 unit). Stack
// traces routinely blow past this, and Telegram hard-rejects the whole
// message with 400 "message is too long" rather than accepting a prefix.
const telegramMaxMessageLength = 4096

const telegramTruncationMarker = "\n\n[truncated]"

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
	form := url.Values{
		"chat_id":                  {a.ChatId},
		"text":                     {telegramText(msg)},
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

// telegramText composes the message and truncates it to Telegram's length
// limit. The URL is always kept in full and un-truncated: once the body is
// cut off, the link back to the dashboard is the only way to see the rest.
func telegramText(msg Message) string {
	var footer strings.Builder
	if msg.URL != "" {
		footer.WriteString("\n\n")
		footer.WriteString(msg.URL)
	}
	footerStr := footer.String()

	var head strings.Builder
	if msg.Subject != "" {
		head.WriteString(msg.Subject)
		head.WriteString("\n\n")
	}
	head.WriteString(msg.Body)
	headStr := head.String()

	full := headStr + footerStr
	if utf8.RuneCountInString(full) <= telegramMaxMessageLength {
		return full
	}

	budget := telegramMaxMessageLength - utf8.RuneCountInString(footerStr) - utf8.RuneCountInString(telegramTruncationMarker)
	if budget < 0 {
		budget = 0
	}
	return truncateRunes(headStr, budget) + telegramTruncationMarker + footerStr
}

// truncateRunes cuts s to at most n runes. Slicing by byte index can split a
// multi-byte UTF-8 character and produce an invalid string; converting to
// []rune first keeps every cut on a character boundary.
func truncateRunes(s string, n int) string {
	if n <= 0 {
		return ""
	}
	runes := []rune(s)
	if len(runes) <= n {
		return s
	}
	return string(runes[:n])
}
