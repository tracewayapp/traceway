package notifications

import (
	"context"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/tracewayapp/traceway/backend/app/config"
)

func TestSmsValidateE164(t *testing.T) {
	cases := []struct {
		number string
		valid  bool
	}{
		{"+12025550123", true},
		{"+381641234567", true},
		{"+4915112345678", true},
		{"12025550123", false},
		{"+0123456", false},
		{"+1", false},
		{"+1202555012345678901", false},
		{"+1 202 555 0123", false},
		{"", false},
	}
	for _, tc := range cases {
		adapter := &SmsAdapter{PhoneNumber: tc.number}
		err := adapter.Validate()
		if tc.valid && err != nil {
			t.Errorf("%q should validate, got %v", tc.number, err)
		}
		if !tc.valid && err == nil {
			t.Errorf("%q should be rejected", tc.number)
		}
	}
}

func TestSmsTextComposition(t *testing.T) {
	msg := Message{Subject: "Error rate breached on checkout-api", Severity: SeverityCritical, URL: "https://x.example/ack/twk_abc"}
	text := smsText(msg)
	if !strings.HasPrefix(text, "[Traceway CRITICAL] ") {
		t.Errorf("missing severity tag: %q", text)
	}
	if !strings.Contains(text, "Ack: https://x.example/ack/twk_abc") {
		t.Errorf("missing ack link: %q", text)
	}

	long := Message{Subject: strings.Repeat("x", 300), Severity: SeverityInfo, URL: "https://x.example/ack/twk_abc"}
	longText := smsText(long)
	if !strings.Contains(longText, "…") {
		t.Errorf("long subject should be truncated: %d chars", len(longText))
	}
	if !strings.Contains(longText, "twk_abc") {
		t.Error("truncation must never eat the ack link")
	}

	multibyte := Message{Subject: strings.Repeat("ж", 120), Severity: SeverityInfo}
	multibyteText := smsText(multibyte)
	if !utf8.ValidString(multibyteText) {
		t.Errorf("truncation split a multi-byte rune: %q", multibyteText)
	}
	if !strings.Contains(multibyteText, "…") {
		t.Error("multi-byte subject should be truncated")
	}
}

func TestMaskPhoneNumber(t *testing.T) {
	cases := map[string]string{
		"+12025550123":  "***0123",
		"+381641234567": "***4567",
		"+1":            "***",
		"":              "***",
	}
	for number, want := range cases {
		if got := MaskPhoneNumber(number); got != want {
			t.Errorf("MaskPhoneNumber(%q) = %q, want %q", number, got, want)
		}
	}
}

// Without Twilio credentials SMS is never offered, so a delivery can only
// reach Send if it was queued before the credentials were removed. It must
// fail rather than report a success the phone never saw, and it must not leak
// the full number into the error.
func TestSmsSendFailsWithoutTwilio(t *testing.T) {
	if config.Config == nil {
		config.Init(config.LoadFromEnv())
	}
	if config.Config.TwilioEnabled() {
		t.Skip("Twilio configured in this environment")
	}
	adapter := &SmsAdapter{PhoneNumber: "+12025550123"}
	err := adapter.Send(context.Background(), Message{Subject: "s", Severity: SeverityInfo})
	if err == nil {
		t.Fatal("send without Twilio credentials must fail, not silently succeed")
	}
	if strings.Contains(err.Error(), "+12025550123") {
		t.Errorf("error must not contain the full phone number: %v", err)
	}
}

func TestTwilioEnabledRequiresSender(t *testing.T) {
	cases := []struct {
		name string
		cfg  config.Cfg
		want bool
	}{
		{"nothing set", config.Cfg{}, false},
		{"credentials without a sender", config.Cfg{TwilioAccountSID: "AC", TwilioAuthToken: "t"}, false},
		{"sender without credentials", config.Cfg{TwilioFromNumber: "+12025550123"}, false},
		{"credentials with from number", config.Cfg{TwilioAccountSID: "AC", TwilioAuthToken: "t", TwilioFromNumber: "+12025550123"}, true},
		{"credentials with messaging service", config.Cfg{TwilioAccountSID: "AC", TwilioAuthToken: "t", TwilioMessagingServiceSID: "MG"}, true},
	}
	for _, tc := range cases {
		if got := tc.cfg.TwilioEnabled(); got != tc.want {
			t.Errorf("%s: TwilioEnabled() = %v, want %v", tc.name, got, tc.want)
		}
	}
}
