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

	"github.com/tracewayapp/traceway/backend/app/config"
)

var e164Pattern = regexp.MustCompile(`^\+[1-9]\d{1,14}$`)

// SmsAdapter delivers via the Twilio Messages API. Without Twilio credentials
// SMS is not offered at all (contact-method creation is rejected and the
// escalator skips SMS methods), so Send only has to fail loudly for deliveries
// already queued when the credentials were removed.
type SmsAdapter struct {
	PhoneNumber string `json:"phoneNumber"`
}

func (a *SmsAdapter) Type() string { return "sms" }

func (a *SmsAdapter) Validate() error {
	if !e164Pattern.MatchString(a.PhoneNumber) {
		return fmt.Errorf("The phone number must be in international E.164 format (e.g. +12025550123).")
	}
	return nil
}

func (a *SmsAdapter) Send(ctx context.Context, msg Message) error {
	cfg := config.Config
	if !cfg.TwilioEnabled() {
		// Never report success here: the outbox would mark the row sent and
		// the page would look delivered to a phone that got nothing. The
		// number is masked because errors reach logs and the issues feed.
		return fmt.Errorf("sms delivery to %s is not configured: no Twilio credentials", MaskPhoneNumber(a.PhoneNumber))
	}

	form := url.Values{"To": {a.PhoneNumber}, "Body": {smsText(msg)}}
	if cfg.TwilioMessagingServiceSID != "" {
		form.Set("MessagingServiceSid", cfg.TwilioMessagingServiceSID)
	} else {
		form.Set("From", cfg.TwilioFromNumber)
	}

	endpoint := "https://api.twilio.com/2010-04-01/Accounts/" + url.PathEscape(cfg.TwilioAccountSID) + "/Messages.json"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return err
	}
	req.SetBasicAuth(cfg.TwilioAccountSID, cfg.TwilioAuthToken)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("twilio request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusCreated {
		return nil
	}
	payload, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
	var twilioError struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	}
	if json.Unmarshal(payload, &twilioError) == nil && twilioError.Message != "" {
		return fmt.Errorf("twilio returned %d (code %d): %s", resp.StatusCode, twilioError.Code, twilioError.Message)
	}
	return fmt.Errorf("twilio returned %d", resp.StatusCode)
}

// MaskPhoneNumber keeps only the last 4 digits, so a number can be shown in
// logs, errors and delivery records without disclosing it.
func MaskPhoneNumber(number string) string {
	if len(number) <= 4 {
		return "***"
	}
	return "***" + number[len(number)-4:]
}

const smsSubjectLimit = 110

// smsText builds the compact SMS body: severity tag, truncated subject, and
// the ack link. Kept within roughly two GSM segments.
func smsText(msg Message) string {
	tag := "[Traceway]"
	switch msg.Severity {
	case SeverityCritical:
		tag = "[Traceway CRITICAL]"
	case SeverityWarning:
		tag = "[Traceway WARNING]"
	}
	subject := msg.Subject
	if len(subject) > smsSubjectLimit {
		// Truncate on a rune boundary: a byte slice can split a multi-byte
		// character and produce invalid UTF-8.
		cut := smsSubjectLimit - 1
		for cut > 0 && !utf8.RuneStart(subject[cut]) {
			cut--
		}
		subject = subject[:cut] + "…"
	}
	text := tag + " " + subject
	if msg.URL != "" {
		text += " Ack: " + msg.URL
	}
	return text
}
