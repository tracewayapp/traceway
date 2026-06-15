package services

import (
	"encoding/json"
	"errors"
	"github.com/tracewayapp/traceway/backend/app/config"
	"net/http"
	"net/url"
	"time"
)

type turnstileService struct {
	secretKey string
	client    *http.Client
}

type turnstileResponse struct {
	Success     bool     `json:"success"`
	ErrorCodes  []string `json:"error-codes"`
	ChallengeTs string   `json:"challenge_ts"`
	Hostname    string   `json:"hostname"`
}

var TurnstileService *turnstileService

func InitTurnstile() {
	TurnstileService = &turnstileService{
		secretKey: config.Config.TurnstileSecretKey,
		client:    &http.Client{Timeout: 10 * time.Second},
	}
}

func (s *turnstileService) IsEnabled() bool {
	return s.secretKey != ""
}

func (s *turnstileService) Verify(token string, remoteIP string) error {
	if token == "" {
		return errors.New("captcha verification required")
	}

	data := url.Values{}
	data.Set("secret", s.secretKey)
	data.Set("response", token)
	if remoteIP != "" {
		data.Set("remoteip", remoteIP)
	}

	resp, err := s.client.PostForm("https://challenges.cloudflare.com/turnstile/v0/siteverify", data)
	if err != nil {
		return errors.New("captcha verification failed")
	}
	defer resp.Body.Close()

	var result turnstileResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return errors.New("captcha verification failed")
	}

	if !result.Success {
		return errors.New("captcha verification failed")
	}

	return nil
}
