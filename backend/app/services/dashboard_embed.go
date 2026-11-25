package services

import (
	"crypto/hmac"
	"crypto/sha256"
	"errors"
	"strconv"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

const dashboardEmbedAudience = "traceway-dashboard-embed"

type DashboardEmbedView struct {
	From       time.Time         `json:"from"`
	To         time.Time         `json:"to"`
	TagFilters map[string]string `json:"tagFilters,omitempty"`
}

type DashboardEmbedClaims struct {
	Rolling     bool      `json:"rolling,omitempty"`
	DashboardId int       `json:"dashboardId"`
	ProjectId   uuid.UUID `json:"projectId"`
	DashboardEmbedView
	jwt.RegisteredClaims
}

// App session validation accepts tokens signed with jwtSecret. Derive a separate
// key so an embed can never authenticate to the ordinary API (or vice versa).
func dashboardEmbedKey() ([]byte, error) {
	if len(jwtSecret) == 0 {
		return nil, errors.New("JWT signing key is unavailable")
	}
	mac := hmac.New(sha256.New, jwtSecret)
	mac.Write([]byte(dashboardEmbedAudience))
	return mac.Sum(nil), nil
}

func GenerateDashboardEmbed(dashboardId int, projectId uuid.UUID, view DashboardEmbedView, expiresAt time.Time, rolling bool) (*DashboardEmbedClaims, string, error) {
	key, err := dashboardEmbedKey()
	if err != nil {
		return nil, "", err
	}
	now := time.Now().UTC()
	claims := &DashboardEmbedClaims{
		DashboardId:        dashboardId,
		Rolling:            rolling,
		ProjectId:          projectId,
		DashboardEmbedView: view,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   uuid.NewSHA1(projectId, []byte(strconv.Itoa(dashboardId))).String(),
			Audience:  jwt.ClaimStrings{dashboardEmbedAudience},
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(expiresAt),
		},
	}
	token, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(key)
	return claims, token, err
}

func ValidateDashboardEmbed(raw, subject string) (*DashboardEmbedClaims, error) {
	if subject == "" {
		return nil, errors.New("missing dashboard UUID")
	}
	claims := new(DashboardEmbedClaims)
	_, err := jwt.ParseWithClaims(raw, claims, func(*jwt.Token) (any, error) { return dashboardEmbedKey() },
		jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}),
		jwt.WithAudience(dashboardEmbedAudience),
		jwt.WithSubject(subject),
		jwt.WithExpirationRequired(),
	)
	if err != nil {
		return nil, err
	}
	return claims, nil
}
