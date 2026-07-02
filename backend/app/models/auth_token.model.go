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
