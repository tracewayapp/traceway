package services

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/tracewayapp/traceway/backend/app/config"
)

var jwtSecret []byte

// SHA-256 of signing keys that once shipped in the repo. Stored as digests so
// no secret-shaped literal sits in production code.
var retiredJWTSecretDigests = map[string]bool{
	"87710a3651ef56a2d43bfaafce1ee73dc8228c3c071788c8165bcf313e873127": true,
	"e7e45a50bcc0f78bf9cf470d4600937c1eeb0bda41d7eba67e3951a536f39221": true,
}

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
	digest := sha256.Sum256([]byte(secret))
	if retiredJWTSecretDigests[hex.EncodeToString(digest[:])] {
		return errors.New("JWT_SECRET uses a publicly known default; generate a unique secret")
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
