package secrets

import (
	"encoding/base64"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tracewayapp/traceway/backend/app/config"
)

func testKey(t *testing.T) *Key {
	t.Helper()
	key, _, err := GenerateKey()
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	return key
}

func TestEncryptDecryptRoundTrip(t *testing.T) {
	key := testKey(t)
	ciphertext, err := key.Encrypt([]byte("ghp_secret"))
	if err != nil {
		t.Fatalf("encrypt: %v", err)
	}
	if !strings.HasPrefix(ciphertext, "v1:"+key.ID()+":") {
		t.Fatalf("ciphertext %q lacks the version and key id header", ciphertext)
	}
	if !IsEncrypted(ciphertext) {
		t.Fatalf("IsEncrypted(%q) = false", ciphertext)
	}
	plaintext, err := key.Decrypt(ciphertext)
	if err != nil {
		t.Fatalf("decrypt: %v", err)
	}
	if string(plaintext) != "ghp_secret" {
		t.Fatalf("decrypt returned %q", plaintext)
	}

	again, _ := key.Encrypt([]byte("ghp_secret"))
	if again == ciphertext {
		t.Fatal("two encryptions of the same value produced the same ciphertext")
	}
}

func TestDecryptWithWrongKey(t *testing.T) {
	ciphertext, _ := testKey(t).Encrypt([]byte("x"))
	if _, err := testKey(t).Decrypt(ciphertext); !errors.Is(err, ErrWrongKey) {
		t.Fatalf("expected ErrWrongKey, got %v", err)
	}
}

func TestDecryptTampered(t *testing.T) {
	key := testKey(t)
	ciphertext, _ := key.Encrypt([]byte("payload"))
	parts := strings.SplitN(ciphertext, ":", 3)
	raw, err := base64.RawStdEncoding.DecodeString(parts[2])
	if err != nil {
		t.Fatal(err)
	}
	raw[len(raw)-1] ^= 1
	parts[2] = base64.RawStdEncoding.EncodeToString(raw)
	if _, err := key.Decrypt(strings.Join(parts, ":")); !errors.Is(err, ErrTampered) {
		t.Fatalf("expected ErrTampered, got %v", err)
	}

	other := testKey(t)
	swappedHeader := "v1:" + key.ID() + ":" + strings.SplitN(mustEncrypt(t, other, "payload"), ":", 3)[2]
	if _, err := key.Decrypt(swappedHeader); !errors.Is(err, ErrTampered) {
		t.Fatalf("expected ErrTampered for a swapped key id, got %v", err)
	}
}

func TestDecryptMalformed(t *testing.T) {
	key := testKey(t)
	for _, value := range []string{"", "plain", "v1:", "v1:abc:xyz", "v2:" + key.ID() + ":abcd", "v1:" + key.ID() + ":", "v1:" + key.ID() + ":!!"} {
		if IsEncrypted(value) && value != "v1:"+key.ID()+":!!" {
			t.Errorf("IsEncrypted(%q) = true", value)
		}
		if _, err := key.Decrypt(value); !errors.Is(err, ErrMalformed) {
			t.Errorf("Decrypt(%q): expected ErrMalformed, got %v", value, err)
		}
	}
}

func TestRotate(t *testing.T) {
	oldKey, newKey := testKey(t), testKey(t)
	ciphertext := mustEncrypt(t, oldKey, "rotate me")
	rotated, err := Rotate(oldKey, newKey, ciphertext)
	if err != nil {
		t.Fatalf("rotate: %v", err)
	}
	if !strings.HasPrefix(rotated, "v1:"+newKey.ID()+":") {
		t.Fatalf("rotated ciphertext %q is not under the new key id", rotated)
	}
	if plaintext, err := newKey.Decrypt(rotated); err != nil || string(plaintext) != "rotate me" {
		t.Fatalf("new key decrypt = %q, %v", plaintext, err)
	}
	if _, err := oldKey.Decrypt(rotated); !errors.Is(err, ErrWrongKey) {
		t.Fatalf("old key still decrypts the rotated value: %v", err)
	}
}

func TestParseKeyAcceptsPaddedAndRawBase64(t *testing.T) {
	_, encoded, err := GenerateKey()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ParseKey(encoded); err != nil {
		t.Fatalf("padded: %v", err)
	}
	if _, err := ParseKey(strings.TrimRight(encoded, "=")); err != nil {
		t.Fatalf("raw: %v", err)
	}
	if _, err := ParseKey("dG9vc2hvcnQ="); err == nil {
		t.Fatal("short key accepted")
	}
}

func TestPackageLevelRequiresInit(t *testing.T) {
	Init(nil)
	if _, err := Encrypt([]byte("x")); !errors.Is(err, ErrNotConfigured) {
		t.Fatalf("expected ErrNotConfigured, got %v", err)
	}
	Init(testKey(t))
	t.Cleanup(func() { Init(nil) })
	ciphertext, err := Encrypt([]byte("x"))
	if err != nil {
		t.Fatal(err)
	}
	if plaintext, err := Decrypt(ciphertext); err != nil || string(plaintext) != "x" {
		t.Fatalf("Decrypt = %q, %v", plaintext, err)
	}
}

func TestLoadKeyPrefersEnv(t *testing.T) {
	_, encoded, _ := GenerateKey()
	key, err := LoadKey(&config.Cfg{SecretsKey: encoded, DBType: "postgres"})
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	parsed, _ := ParseKey(encoded)
	if key.ID() != parsed.ID() {
		t.Fatal("LoadKey did not use SECRETS_KEY")
	}
	if _, err := LoadKey(&config.Cfg{SecretsKey: "nope", DBType: "sqlite"}); err == nil || !strings.Contains(err.Error(), "SECRETS_KEY") {
		t.Fatalf("invalid key: expected an error naming SECRETS_KEY, got %v", err)
	}
}

func TestLoadKeyRequiresEnvOutsideSQLite(t *testing.T) {
	_, err := LoadKey(&config.Cfg{DBType: "postgres"})
	if err == nil || !strings.Contains(err.Error(), "SECRETS_KEY") {
		t.Fatalf("expected an error naming SECRETS_KEY, got %v", err)
	}
}

func TestLoadKeyCreatesAndReusesKeyFile(t *testing.T) {
	config.LoggingEnabled = false
	t.Cleanup(func() { config.LoggingEnabled = true })
	dir := t.TempDir()
	cfg := &config.Cfg{DBType: "sqlite", SQLitePath: filepath.Join(dir, "traceway.db")}

	first, err := LoadKey(cfg)
	if err != nil {
		t.Fatalf("first load: %v", err)
	}
	info, err := os.Stat(filepath.Join(dir, KeyFileName))
	if err != nil {
		t.Fatalf("key file missing: %v", err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("key file mode = %o, want 600", info.Mode().Perm())
	}
	second, err := LoadKey(cfg)
	if err != nil {
		t.Fatalf("second load: %v", err)
	}
	if first.ID() != second.ID() {
		t.Fatal("second boot generated a different key")
	}

	memory, err := LoadKey(&config.Cfg{DBType: "sqlite", SQLitePath: ":memory:"})
	if err != nil || memory == nil {
		t.Fatalf("in-memory database: %v", err)
	}
	if _, err := os.Stat(KeyFileName); err == nil {
		t.Fatal("an in-memory database wrote a key file into the working directory")
	}
}

func mustEncrypt(t *testing.T, key *Key, plaintext string) string {
	t.Helper()
	ciphertext, err := key.Encrypt([]byte(plaintext))
	if err != nil {
		t.Fatal(err)
	}
	return ciphertext
}
