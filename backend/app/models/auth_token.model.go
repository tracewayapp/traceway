package models

import "time"

type DeviceAuthorization struct {
	DeviceCodeHash  string
	UserCode        string
	ClientId        string
	Status          string
	UserId          *int
	IntervalSeconds int
	LastPolledAt    *time.Time
	ExpiresAt       time.Time
	CreatedAt       time.Time
}

type RefreshToken struct {
	TokenHash string
	FamilyId  string
	UserId    int
	ClientId  string
	Revoked   bool
	Used      bool
	UsedAt    *time.Time
	ExpiresAt time.Time
	CreatedAt time.Time
}

type PersonalAccessToken struct {
	Id         string     `json:"id"`
	Prefix     string     `json:"prefix"`
	Name       string     `json:"name"`
	LastUsedAt *time.Time `json:"lastUsedAt"`
	ExpiresAt  *time.Time `json:"expiresAt"`
	CreatedAt  time.Time  `json:"createdAt"`
}

type DeviceAuthorizeRequest struct {
	ClientId string `json:"client_id" form:"client_id"`
}

type DeviceAuthorizeResponse struct {
	DeviceCode              string `json:"device_code"`
	UserCode                string `json:"user_code"`
	VerificationURI         string `json:"verification_uri"`
	VerificationURIComplete string `json:"verification_uri_complete"`
	ExpiresIn               int    `json:"expires_in"`
	Interval                int    `json:"interval"`
}

type TokenRequest struct {
	GrantType    string `json:"grant_type" form:"grant_type"`
	DeviceCode   string `json:"device_code" form:"device_code"`
	RefreshToken string `json:"refresh_token" form:"refresh_token"`
	ClientId     string `json:"client_id" form:"client_id"`
	Code         string `json:"code" form:"code"`
	CodeVerifier string `json:"code_verifier" form:"code_verifier"`
	RedirectUri  string `json:"redirect_uri" form:"redirect_uri"`
	Resource     string `json:"resource" form:"resource"`
}

type TokenSetResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int    `json:"expires_in"`
	Email        string `json:"email,omitempty"`
}

type DeviceApproveRequest struct {
	UserCode string `json:"user_code"`
}

type DeviceLookupResponse struct {
	UserCode    string    `json:"user_code"`
	Status      string    `json:"status"`
	ClientName  string    `json:"clientName"`
	RequestedAt time.Time `json:"requestedAt"`
	ExpiresAt   time.Time `json:"expiresAt"`
}

type OauthClient struct {
	Id           string
	Name         string
	RedirectUris []string
	CreatedAt    time.Time
}

type AuthorizationCode struct {
	CodeHash      string
	ClientId      string
	UserId        int
	RedirectUri   string
	CodeChallenge string
	ExpiresAt     time.Time
	CreatedAt     time.Time
}

type RegisterClientRequest struct {
	ClientName   string   `json:"client_name"`
	RedirectUris []string `json:"redirect_uris"`
}

type RegisterClientResponse struct {
	ClientId                string   `json:"client_id"`
	ClientName              string   `json:"client_name"`
	RedirectUris            []string `json:"redirect_uris"`
	GrantTypes              []string `json:"grant_types"`
	ResponseTypes           []string `json:"response_types"`
	TokenEndpointAuthMethod string   `json:"token_endpoint_auth_method"`
	ClientIdIssuedAt        int64    `json:"client_id_issued_at"`
}

type AuthorizeApproveRequest struct {
	ClientId            string `json:"client_id"`
	RedirectUri         string `json:"redirect_uri"`
	State               string `json:"state"`
	CodeChallenge       string `json:"code_challenge"`
	CodeChallengeMethod string `json:"code_challenge_method"`
	Resource            string `json:"resource"`
}

type AuthorizeDenyRequest struct {
	ClientId    string `json:"client_id"`
	RedirectUri string `json:"redirect_uri"`
	State       string `json:"state"`
}

type AuthorizeRedirectResponse struct {
	RedirectTo string `json:"redirectTo"`
}

type OauthClientLookupResponse struct {
	ClientName string `json:"clientName"`
}

type CreatePATRequest struct {
	Name          string `json:"name"`
	ExpiresInDays *int   `json:"expiresInDays"`
}

type CreatePATResponse struct {
	Id        string     `json:"id"`
	Name      string     `json:"name"`
	Prefix    string     `json:"prefix"`
	Token     string     `json:"token"`
	CreatedAt time.Time  `json:"createdAt"`
	ExpiresAt *time.Time `json:"expiresAt"`
}
