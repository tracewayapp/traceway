// Package secrets encrypts credentials that rest in the main database
// (channel tokens, integration configs, repository credentials) with
// AES-256-GCM under one operator-owned key.
//
// A ciphertext is the string "v1:<keyid>:<base64(nonce || sealed)>". The key
// id is the first eight hex characters of SHA-256 over the raw key, so a value
// encrypted under a different key is reported as such instead of failing
// authentication, and Rotate can tell rows apart during a key change.
package secrets

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
)

const (
	KeySize = 32

	version     = "v1"
	keyIdLength = 8
)

var (
	ErrNotConfigured = errors.New("secrets: no encryption key is configured (set SECRETS_KEY)")
	ErrWrongKey      = errors.New("secrets: value was encrypted with a different key than the configured SECRETS_KEY")
	ErrTampered      = errors.New("secrets: value failed authentication; it was altered or corrupted")
	ErrMalformed     = errors.New("secrets: value is not an encrypted secret")
)

type Key struct {
	id   string
	aead cipher.AEAD
}

func NewKey(raw []byte) (*Key, error) {
	if len(raw) != KeySize {
		return nil, fmt.Errorf("secrets: key must be %d bytes, got %d", KeySize, len(raw))
	}
	block, err := aes.NewCipher(raw)
	if err != nil {
		return nil, err
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	sum := sha256.Sum256(raw)
	return &Key{id: hex.EncodeToString(sum[:keyIdLength/2]), aead: aead}, nil
}

// ParseKey decodes a base64 key as SECRETS_KEY carries it, with or without
// padding.
func ParseKey(encoded string) (*Key, error) {
	encoded = strings.TrimSpace(encoded)
	raw, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		raw, err = base64.RawStdEncoding.DecodeString(encoded)
	}
	if err != nil {
		return nil, fmt.Errorf("secrets: key is not valid base64: %w", err)
	}
	return NewKey(raw)
}

// GenerateKey returns a fresh random key and its base64 form for storage.
func GenerateKey() (*Key, string, error) {
	raw := make([]byte, KeySize)
	if _, err := rand.Read(raw); err != nil {
		return nil, "", err
	}
	key, err := NewKey(raw)
	if err != nil {
		return nil, "", err
	}
	return key, base64.StdEncoding.EncodeToString(raw), nil
}

func (k *Key) ID() string { return k.id }

func (k *Key) Encrypt(plaintext []byte) (string, error) {
	nonce := make([]byte, k.aead.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return "", err
	}
	header := version + ":" + k.id
	sealed := k.aead.Seal(nil, nonce, plaintext, []byte(header))
	return header + ":" + base64.RawStdEncoding.EncodeToString(append(nonce, sealed...)), nil
}

func (k *Key) Decrypt(ciphertext string) ([]byte, error) {
	keyId, payload, ok := splitCiphertext(ciphertext)
	if !ok {
		return nil, ErrMalformed
	}
	if keyId != k.id {
		return nil, ErrWrongKey
	}
	raw, err := base64.RawStdEncoding.DecodeString(payload)
	if err != nil || len(raw) < k.aead.NonceSize() {
		return nil, ErrMalformed
	}
	nonceSize := k.aead.NonceSize()
	plaintext, err := k.aead.Open(nil, raw[:nonceSize], raw[nonceSize:], []byte(version+":"+keyId))
	if err != nil {
		return nil, ErrTampered
	}
	return plaintext, nil
}

// Rotate re-encrypts a value from one key to another.
func Rotate(from, to *Key, ciphertext string) (string, error) {
	plaintext, err := from.Decrypt(ciphertext)
	if err != nil {
		return "", err
	}
	return to.Encrypt(plaintext)
}

// IsEncrypted reports whether a stored value carries the ciphertext header,
// which is how callers tell an already-encrypted field from a plaintext one.
func IsEncrypted(value string) bool {
	_, _, ok := splitCiphertext(value)
	return ok
}

func splitCiphertext(value string) (keyId, payload string, ok bool) {
	parts := strings.SplitN(value, ":", 3)
	if len(parts) != 3 || parts[0] != version || len(parts[1]) != keyIdLength || parts[2] == "" {
		return "", "", false
	}
	if _, err := hex.DecodeString(parts[1]); err != nil {
		return "", "", false
	}
	return parts[1], parts[2], true
}

var active *Key

// Init installs the process-wide key used by Encrypt and Decrypt.
func Init(key *Key) { active = key }

func Configured() bool { return active != nil }

func Encrypt(plaintext []byte) (string, error) {
	if active == nil {
		return "", ErrNotConfigured
	}
	return active.Encrypt(plaintext)
}

func Decrypt(ciphertext string) ([]byte, error) {
	if active == nil {
		return nil, ErrNotConfigured
	}
	return active.Decrypt(ciphertext)
}
