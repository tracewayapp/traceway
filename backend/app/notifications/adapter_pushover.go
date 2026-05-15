package notifications

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

type PushoverAdapter struct {
	UserKey  string `json:"userKey"`
	AppToken string `json:"appToken"`
	Device   string `json:"device,omitempty"`
	Priority int    `json:"priority,omitempty"`
	Retry    int    `json:"retry,omitempty"`
	Expire   int    `json:"expire,omitempty"`
	Sound    string `json:"sound,omitempty"`
	TTL      int    `json:"ttl,omitempty"`
	HTML     bool   `json:"html,omitempty"`
	Callback string `json:"callback,omitempty"`
}

func (a *PushoverAdapter) Type() string { return "pushover" }

func (a *PushoverAdapter) Validate() error {
	if a.UserKey == "" {
		return fmt.Errorf("Pushover user key is required")
	}
	if a.AppToken == "" {
		return fmt.Errorf("Pushover app token is required")
	}
	if a.Priority == 2 {
		if a.Retry < 30 {
			return fmt.Errorf("Pushover retry must be at least 30 seconds")
		}
		if a.Expire < 1 || a.Expire > 10800 {
			return fmt.Errorf("Pushover expire must be between 1 and 10800 seconds")
		}
		if a.Retry >= a.Expire {
			return fmt.Errorf("Pushover retry (%ds) must be less than expire (%ds)", a.Retry, a.Expire)
		}
	}
	return nil
}

func (a *PushoverAdapter) Send(ctx context.Context, msg Message) error {
	form := url.Values{
		"token":   {a.AppToken},
		"user":    {a.UserKey},
		"message": {msg.Body},
	}
	if msg.Subject != "" {
		form.Set("title", msg.Subject)
	}
	if a.Device != "" {
		form.Set("device", a.Device)
	}
	if a.Sound != "" {
		form.Set("sound", a.Sound)
	}
	if a.HTML {
		form.Set("html", "1")
	}
	if a.TTL > 0 {
		form.Set("ttl", strconv.Itoa(a.TTL))
	}
	if msg.URL != "" {
		form.Set("url", msg.URL)
	}
	form.Set("timestamp", strconv.FormatInt(time.Now().Unix(), 10))

	priority := a.Priority
	form.Set("priority", strconv.Itoa(priority))

	if priority == 2 {
		retry := a.Retry
		if retry < 30 {
			retry = 30
		}
		expire := a.Expire
		if expire < 1 {
			expire = 3600
		} else if expire > 10800 {
			expire = 10800
		}
		form.Set("retry", strconv.Itoa(retry))
		form.Set("expire", strconv.Itoa(expire))
		if a.Callback != "" {
			form.Set("callback", a.Callback)
		}
	}

	body := form.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		"https://api.pushover.net/1/messages.json",
		strings.NewReader(body),
	)
	if err != nil {
		return fmt.Errorf("failed to create Pushover request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("User-Agent", "Traceway/1.0")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("Pushover request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)

	if resp.StatusCode == http.StatusOK {
		var result struct {
			Status  int    `json:"status"`
			Request string `json:"request"`
		}
		if err := json.Unmarshal(respBody, &result); err == nil && result.Status == 1 {
			return nil
		}
		return fmt.Errorf("Pushover returned unexpected response: %s", string(respBody))
	}

	if resp.StatusCode == http.StatusTooManyRequests {
		return fmt.Errorf("Pushover rate limit exceeded (monthly quota exhausted)")
	}

	var errResult struct {
		Errors []string `json:"errors"`
	}
	if err := json.Unmarshal(respBody, &errResult); err == nil && len(errResult.Errors) > 0 {
		return fmt.Errorf("Pushover returned %d: %s", resp.StatusCode, strings.Join(errResult.Errors, "; "))
	}

	return fmt.Errorf("Pushover returned status %d", resp.StatusCode)
}
