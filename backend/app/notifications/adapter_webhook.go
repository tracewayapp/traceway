package notifications

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const (
	// Matches the outbox's per-attempt budget: receivers such as Google Apps Script answer only after running.
	webhookTimeout      = 30 * time.Second
	webhookMaxRedirects = 3
)

var webhookClient = &http.Client{
	CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
}

type WebhookAdapter struct {
	URL     string            `json:"url"`
	Method  string            `json:"method,omitempty"`
	Headers map[string]string `json:"headers,omitempty"`
	Secret  string            `json:"secret,omitempty"`
}

func (a *WebhookAdapter) Type() string { return "webhook" }

func (a *WebhookAdapter) Validate() error {
	if a.URL == "" {
		return fmt.Errorf("webhook URL is required")
	}
	if a.Method == "" {
		a.Method = "POST"
	}
	return nil
}

func (a *WebhookAdapter) Send(ctx context.Context, msg Message) error {
	payload := map[string]string{
		"subject":   msg.Subject,
		"body":      msg.Body,
		"severity":  string(msg.Severity),
		"ruleType":  msg.RuleType,
		"ruleName":  msg.RuleName,
		"url":       msg.URL,
		"timestamp": time.Now().UTC().Format(time.RFC3339),
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal webhook payload: %w", err)
	}

	method := a.Method
	if method == "" {
		method = "POST"
	}

	ctx, cancel := context.WithTimeout(ctx, webhookTimeout)
	defer cancel()

	target := a.URL
	for redirects := 0; ; redirects++ {
		resp, err := a.deliver(ctx, method, target, body)
		if err != nil {
			return err
		}
		if resp.StatusCode >= 400 {
			return fmt.Errorf("webhook returned status %d", resp.StatusCode)
		}
		next := resendTarget(resp)
		if next == nil {
			return nil
		}
		if redirects == webhookMaxRedirects {
			return fmt.Errorf("webhook redirected more than %d times", webhookMaxRedirects)
		}
		target = next.String()
	}
}

func (a *WebhookAdapter) deliver(ctx context.Context, method, target string, body []byte) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, method, target, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create webhook request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	if origin, err := url.Parse(a.URL); err == nil && sameSite(req.URL.Hostname(), origin.Hostname()) {
		for k, v := range a.Headers {
			req.Header.Set(k, v)
		}

		if a.Secret != "" {
			mac := hmac.New(sha256.New, []byte(a.Secret))
			mac.Write(body)
			sig := hex.EncodeToString(mac.Sum(nil))
			req.Header.Set("X-Traceway-Signature", "sha256="+sig)
		}
	}

	resp, err := webhookClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("webhook request failed: %w", err)
	}
	defer resp.Body.Close()
	io.Copy(io.Discard, resp.Body)

	return resp, nil
}

func resendTarget(resp *http.Response) *url.URL {
	next, err := resp.Location()
	if err != nil {
		return nil
	}
	switch resp.StatusCode {
	case http.StatusMovedPermanently, http.StatusTemporaryRedirect, http.StatusPermanentRedirect:
		return next
	case http.StatusFound, http.StatusSeeOther:
		// Anything but an https upgrade is the receiver pointing at its response (Apps Script): the payload arrived.
		from := resp.Request.URL
		if from.Scheme == "http" && next.Scheme == "https" && sameSite(next.Hostname(), from.Hostname()) {
			return next
		}
	}
	return nil
}

func sameSite(host, origin string) bool {
	host, origin = strings.ToLower(host), strings.ToLower(origin)
	return host == origin || strings.HasSuffix(host, "."+origin)
}
