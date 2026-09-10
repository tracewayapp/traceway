package services

import (
	"errors"
	"fmt"
	"time"

	"github.com/tracewayapp/traceway/backend/app/config"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

var jwtSecret []byte

type JWTClaims struct {
	UserId int    `json:"userId"`
	Email  string `json:"email"`
	jwt.RegisteredClaims
}

func InitJWT() error {
	secret := config.Config.JWTSecret
	if secret == "" {
		return errors.New("JWT_SECRET environment variable is not set")
	}
	if len(secret) < 32 {
		return errors.New("JWT_SECRET must be at least 32 characters")
	}
	jwtSecret = []byte(secret)
	return nil
}

func GenerateToken(userId int, email string) (string, error) {
	return generateToken(userId, email, 7*24*time.Hour)
}

func GenerateAccessToken(userId int, email string, ttl time.Duration) (string, error) {
	return generateToken(userId, email, ttl)
}

func generateToken(userId int, email string, ttl time.Duration) (string, error) {
	claims := JWTClaims{
		UserId: userId,
		Email:  email,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(ttl)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			NotBefore: jwt.NewNumericDate(time.Now()),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(jwtSecret)
}

func ValidateToken(tokenString string) (*JWTClaims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &JWTClaims{}, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return jwtSecret, nil
	})

	if err != nil {
		return nil, err
	}

	if claims, ok := token.Claims.(*JWTClaims); ok && token.Valid {
		return claims, nil
	}

	return nil, errors.New("invalid token")
}

const runTokenKind = "run"

// RunTokenClaims is the credential an executor hands the agent process: it
// reads one project, as readonly, for a short while, and carries no user.
type RunTokenClaims struct {
	Kind      string `json:"kind"`
	AttemptId string `json:"attemptId"`
	ProjectId string `json:"projectId"`
	Role      string `json:"role"`
	jwt.RegisteredClaims
}

func GenerateRunToken(attemptId uuid.UUID, projectId uuid.UUID, ttl time.Duration) (string, time.Time, error) {
	now := time.Now()
	expires := now.Add(ttl)
	claims := RunTokenClaims{
		Kind:      runTokenKind,
		AttemptId: attemptId.String(),
		ProjectId: projectId.String(),
		Role:      "readonly",
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(expires),
			IssuedAt:  jwt.NewNumericDate(now),
			NotBefore: jwt.NewNumericDate(now),
		},
	}
	signed, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(jwtSecret)
	return signed, expires, err
}

var ErrNotRunToken = errors.New("not a run token")

// ValidateRunToken accepts only tokens minted by GenerateRunToken; a
// dashboard session token fails with ErrNotRunToken so callers fall through
// to the user shapes.
func ValidateRunToken(tokenString string) (*RunTokenClaims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &RunTokenClaims{}, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return jwtSecret, nil
	})
	if err != nil {
		return nil, err
	}
	claims, ok := token.Claims.(*RunTokenClaims)
	if !ok || !token.Valid {
		return nil, errors.New("invalid token")
	}
	if claims.Kind != runTokenKind || claims.Role != "readonly" {
		return nil, ErrNotRunToken
	}
	if _, err := uuid.Parse(claims.AttemptId); err != nil {
		return nil, ErrNotRunToken
	}
	if _, err := uuid.Parse(claims.ProjectId); err != nil {
		return nil, ErrNotRunToken
	}
	return claims, nil
}

// stateClaims is a short-lived signed bag of strings for browser round
// trips that leave and re-enter the instance (an App manifest flow): the
// kind keeps one flow's state from being replayed into another.
type stateClaims struct {
	Kind   string            `json:"kind"`
	Values map[string]string `json:"values"`
	jwt.RegisteredClaims
}

// SignState signs values for a round trip of at most ttl.
func SignState(kind string, values map[string]string, ttl time.Duration) (string, error) {
	if len(jwtSecret) == 0 {
		return "", errors.New("JWT secret is not initialized")
	}
	claims := stateClaims{Kind: kind, Values: values, RegisteredClaims: jwt.RegisteredClaims{
		IssuedAt:  jwt.NewNumericDate(time.Now()),
		ExpiresAt: jwt.NewNumericDate(time.Now().Add(ttl)),
	}}
	return jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(jwtSecret)
}

// ParseState verifies a state token of the given kind and returns its values.
func ParseState(kind string, token string) (map[string]string, error) {
	if len(jwtSecret) == 0 {
		return nil, errors.New("JWT secret is not initialized")
	}
	var claims stateClaims
	parsed, err := jwt.ParseWithClaims(token, &claims, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, errors.New("unexpected signing method")
		}
		return jwtSecret, nil
	})
	if err != nil {
		return nil, err
	}
	if !parsed.Valid || claims.Kind != kind {
		return nil, errors.New("state is not valid for this flow")
	}
	return claims.Values, nil
}
