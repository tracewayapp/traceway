package notifications

import (
	"strings"
	"testing"
	"unicode/utf8"
)

func TestTelegramTextWithinLimitIsUnchanged(t *testing.T) {
	msg := Message{
		Subject: "New error: RuntimeError",
		Body:    "A new error has been detected.",
		URL:     "https://x.example/issues/abc123",
	}
	text := telegramText(msg)
	want := msg.Subject + "\n\n" + msg.Body + "\n\n" + msg.URL
	if text != want {
		t.Errorf("telegramText() = %q, want %q", text, want)
	}
}

func TestTelegramTextTruncatesOversizedBody(t *testing.T) {
	msg := Message{
		Subject: "New error: PgError",
		Body:    "Stack trace:\n" + strings.Repeat("at some.deeply.nested.frame()\n", 500),
		URL:     "https://x.example/issues/abc123",
	}
	text := telegramText(msg)

	if utf8.RuneCountInString(text) > telegramMaxMessageLength {
		t.Errorf("text still exceeds Telegram's limit: %d runes", utf8.RuneCountInString(text))
	}
	if !strings.Contains(text, "[truncated]") {
		t.Error("oversized body should be marked as truncated")
	}
	if !strings.HasSuffix(text, msg.URL) {
		t.Errorf("truncation must never eat the URL: %q", text)
	}
}

func TestTelegramTextTruncationKeepsValidUTF8(t *testing.T) {
	msg := Message{
		Body: strings.Repeat("ж", 5000),
		URL:  "https://x.example/issues/abc123",
	}
	text := telegramText(msg)

	if !utf8.ValidString(text) {
		t.Errorf("truncation split a multi-byte rune: %q", text)
	}
	if utf8.RuneCountInString(text) > telegramMaxMessageLength {
		t.Errorf("text still exceeds Telegram's limit: %d runes", utf8.RuneCountInString(text))
	}
}

func TestTelegramTextWithoutURLStillTruncates(t *testing.T) {
	msg := Message{Body: strings.Repeat("x", 10000)}
	text := telegramText(msg)

	if utf8.RuneCountInString(text) > telegramMaxMessageLength {
		t.Errorf("text still exceeds Telegram's limit: %d runes", utf8.RuneCountInString(text))
	}
	if !strings.Contains(text, "[truncated]") {
		t.Error("oversized body should be marked as truncated")
	}
}
