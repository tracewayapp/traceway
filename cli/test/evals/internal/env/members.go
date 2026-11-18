package env

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"time"
)

// Member is a non-owner user of the organization.
type Member struct {
	Email  string
	Role   string
	UserID int
	JWT    string
}

// AddMember invites a user with role and accepts the invitation. The accept
// token is not in the invite response; with SMTP off the backend logs the
// invitation email, link included, so it is read from there.
func AddMember(ctx context.Context, b *Backend, a *Admin, email, role string) (*Member, error) {
	before := len(b.InvitationTokens())
	api := b.API().WithBearer(a.JWT)
	if _, err := api.MustOK(ctx, http.MethodPost, fmt.Sprintf("/api/organizations/%d/invitations", a.OrgID), nil,
		map[string]string{"email": email, "role": role}); err != nil {
		return nil, err
	}
	var token string
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		if toks := b.InvitationTokens(); len(toks) > before {
			token = toks[len(toks)-1]
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	if token == "" {
		return nil, fmt.Errorf("invitation for %s: no accept token in the backend log", email)
	}
	res, err := b.API().MustOK(ctx, http.MethodPost, "/api/invitations/"+url.PathEscape(token)+"/accept", nil,
		map[string]string{"name": "Eval " + role, "password": randomHex(12)})
	if err != nil {
		return nil, err
	}
	var out struct {
		Token string `json:"token"`
		User  struct {
			ID int `json:"id"`
		} `json:"user"`
	}
	if err := res.JSON(&out); err != nil {
		return nil, err
	}
	return &Member{Email: email, Role: role, UserID: out.User.ID, JWT: out.Token}, nil
}

// SetAgentAccess turns the organization's agent write opt-in on or off.
func SetAgentAccess(ctx context.Context, b *Backend, a *Admin, enabled bool) error {
	_, err := b.API().WithBearer(a.JWT).MustOK(ctx, http.MethodPut,
		fmt.Sprintf("/api/organizations/%d/agent-access", a.OrgID), nil, map[string]bool{"agentWriteEnabled": enabled})
	return err
}

// SetupToken mints an organization setup token (tws_...).
func SetupToken(ctx context.Context, b *Backend, a *Admin) (string, error) {
	res, err := b.API().WithBearer(a.JWT).MustOK(ctx, http.MethodPost, "/api/setup-tokens", nil, map[string]int{"organizationId": a.OrgID})
	if err != nil {
		return "", err
	}
	var out struct {
		Token string `json:"token"`
	}
	if err := res.JSON(&out); err != nil {
		return "", err
	}
	return out.Token, nil
}

// ApprovePendingDraft approves the organization's pending setup draft, if
// any, and reports whether one was approved.
func ApprovePendingDraft(ctx context.Context, b *Backend, a *Admin) (bool, error) {
	api := b.API().WithBearer(a.JWT)
	res, err := api.MustOK(ctx, http.MethodGet, "/api/setup/drafts", url.Values{"organizationId": {fmt.Sprint(a.OrgID)}}, nil)
	if err != nil {
		return false, err
	}
	var out struct {
		Draft *struct {
			ID     any    `json:"id"`
			Status string `json:"status"`
		} `json:"draft"`
	}
	if err := res.JSON(&out); err != nil {
		return false, err
	}
	if out.Draft == nil || out.Draft.Status != "pending" {
		return false, nil
	}
	_, err = api.MustOK(ctx, http.MethodPost, fmt.Sprintf("/api/setup/drafts/%v/approve", out.Draft.ID), nil, nil)
	return err == nil, err
}
