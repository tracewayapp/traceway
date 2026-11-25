package client

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"unicode/utf8"
)

// PersonalAccessTokenMaxDays is the longest lifetime a token can be given.
// Zero means the token never expires.
const PersonalAccessTokenMaxDays = 3650

// ValidatePersonalAccessToken mirrors the server's 422s for a new read token.
func ValidatePersonalAccessToken(name string, expiresInDays int) error {
	name = strings.TrimSpace(name)
	switch {
	case name == "":
		return errors.New("a token name is required")
	case utf8.RuneCountInString(name) > 100:
		return errors.New("the token name must be 100 characters or fewer")
	case expiresInDays < 0 || expiresInDays > PersonalAccessTokenMaxDays:
		return fmt.Errorf("the expiry must be between 1 and %d days, or 0 for none", PersonalAccessTokenMaxDays)
	}
	return nil
}

// CreateReadToken creates a read-only personal access token (twp_) for the
// signed-in user. The response carries the token once: treat it as a
// credential. Agent credentials can only create read tokens, and the request
// is an agent write, so projectID selects the organization whose Agent Access
// setting and activity log apply. expiresInDays 0 means no expiry.
func (c *Client) CreateReadToken(ctx context.Context, projectID, name string, expiresInDays int) (json.RawMessage, error) {
	body := map[string]any{"name": strings.TrimSpace(name), "scope": "read"}
	if expiresInDays > 0 {
		body["expiresInDays"] = expiresInDays
	}
	var out json.RawMessage
	err := c.do(ctx, http.MethodPost, "/api/personal-access-tokens?"+url.Values{"projectId": {projectID}}.Encode(), body, &out)
	return out, err
}

// ListPersonalAccessTokens lists the signed-in user's tokens: name, prefix,
// scope, last use and expiry, never the token itself.
func (c *Client) ListPersonalAccessTokens(ctx context.Context) (json.RawMessage, error) {
	var out json.RawMessage
	err := c.do(ctx, http.MethodGet, "/api/personal-access-tokens", nil, &out)
	return out, err
}
