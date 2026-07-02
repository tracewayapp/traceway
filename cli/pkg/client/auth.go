package client

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
)

type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type loginResponse struct {
	Token string `json:"token"`
}

// Login exchanges an email + password for a JWT. The returned token should be
// stored by the caller and passed to subsequent Client constructions via
// WithJWT.
func (c *Client) Login(ctx context.Context, email, password string) (string, error) {
	var resp loginResponse
	if err := c.do(ctx, http.MethodPost, "/api/login", loginRequest{Email: email, Password: password}, &resp); err != nil {
		return "", err
	}
	if resp.Token == "" {
		return "", errors.New("login response did not include a token")
	}
	return resp.Token, nil
}

// DeviceAuth is the response from starting the device authorization flow.
type DeviceAuth struct {
	DeviceCode              string `json:"device_code"`
	UserCode                string `json:"user_code"`
	VerificationURI         string `json:"verification_uri"`
	VerificationURIComplete string `json:"verification_uri_complete"`
	ExpiresIn               int    `json:"expires_in"`
	Interval                int    `json:"interval"`
}

// TokenSet is the access + refresh token pair issued by the device-code and
// refresh grants.
type TokenSet struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int    `json:"expires_in"`
	Email        string `json:"email"`
}

type deviceAuthorizeRequest struct {
	ClientID string `json:"client_id,omitempty"`
}

// DeviceAuthorize starts the device authorization flow and returns the user
// code, verification URL, and polling parameters.
func (c *Client) DeviceAuthorize(ctx context.Context, clientID string) (*DeviceAuth, error) {
	var resp DeviceAuth
	if err := c.do(ctx, http.MethodPost, "/api/auth/device/authorize", deviceAuthorizeRequest{ClientID: clientID}, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

type deviceTokenRequest struct {
	GrantType  string `json:"grant_type"`
	DeviceCode string `json:"device_code"`
}

// PollDeviceToken polls the token endpoint once. It returns ErrAuthorizationPending,
// ErrSlowDown, ErrAccessDenied, or ErrExpiredToken to drive the polling loop.
func (c *Client) PollDeviceToken(ctx context.Context, deviceCode string) (*TokenSet, error) {
	var resp TokenSet
	if err := c.do(ctx, http.MethodPost, "/api/auth/device/token", deviceTokenRequest{GrantType: "device_code", DeviceCode: deviceCode}, &resp); err != nil {
		return nil, mapOAuthError(err)
	}
	return &resp, nil
}

type refreshRequest struct {
	GrantType    string `json:"grant_type"`
	RefreshToken string `json:"refresh_token"`
}

// Refresh exchanges a refresh token for a new token set (with a rotated refresh
// token). It returns ErrInvalidGrant when the refresh token is expired, revoked,
// or reused.
func (c *Client) Refresh(ctx context.Context, refreshToken string) (*TokenSet, error) {
	var resp TokenSet
	if err := c.do(ctx, http.MethodPost, "/api/auth/token", refreshRequest{GrantType: "refresh_token", RefreshToken: refreshToken}, &resp); err != nil {
		return nil, mapOAuthError(err)
	}
	return &resp, nil
}

type logoutRequest struct {
	RefreshToken string `json:"refresh_token"`
}

// Logout revokes the refresh token's family server-side so the session cannot
// be refreshed after the local credentials are removed. It is idempotent; the
// caller should treat any error as non-fatal (local logout must still succeed).
func (c *Client) Logout(ctx context.Context, refreshToken string) error {
	return c.do(ctx, http.MethodPost, "/api/auth/logout", logoutRequest{RefreshToken: refreshToken}, nil)
}

// mapOAuthError translates an OAuth-style {"error":"..."} 400 body into the
// matching sentinel error.
func mapOAuthError(err error) error {
	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		return err
	}
	var body struct {
		Error string `json:"error"`
	}
	if jsonErr := json.Unmarshal([]byte(apiErr.Body), &body); jsonErr != nil {
		return err
	}
	switch body.Error {
	case "authorization_pending":
		return ErrAuthorizationPending
	case "slow_down":
		return ErrSlowDown
	case "access_denied":
		return ErrAccessDenied
	case "expired_token":
		return ErrExpiredToken
	case "invalid_grant":
		return ErrInvalidGrant
	}
	return err
}
