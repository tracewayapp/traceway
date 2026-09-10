package secrets

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/tracewayapp/traceway/backend/app/config"
)

const KeyFileName = "traceway.secrets.key"

// LoadKey resolves the encryption key for this process. SECRETS_KEY wins when
// set. Without it a SQLite deployment falls back to a key file next to the
// main database, generated on first boot, so a single-binary install works
// out of the box; every other main database requires the variable, since the
// database and the key would otherwise live on different machines.
func LoadKey(cfg *config.Cfg) (*Key, error) {
	if cfg.SecretsKey != "" {
		key, err := ParseKey(cfg.SecretsKey)
		if err != nil {
			return nil, fmt.Errorf("SECRETS_KEY is invalid: %w (generate one with: openssl rand -base64 32)", err)
		}
		return key, nil
	}
	if cfg.DBType != "sqlite" {
		return nil, errors.New("SECRETS_KEY is not set; it is required when the main database is PostgreSQL (generate one with: openssl rand -base64 32)")
	}
	if cfg.SQLitePath == ":memory:" {
		key, _, err := GenerateKey()
		return key, err
	}
	return loadOrCreateKeyFile(keyFilePath(cfg.SQLitePath))
}

func keyFilePath(sqlitePath string) string {
	if sqlitePath == "" {
		sqlitePath = "./traceway.db"
	}
	return filepath.Join(filepath.Dir(sqlitePath), KeyFileName)
}

func loadOrCreateKeyFile(path string) (*Key, error) {
	if key, err := readKeyFile(path); err == nil {
		config.Logf("SECRETS_KEY is not set; using the key file at %s. Set SECRETS_KEY to the file's contents and back it up: stored credentials cannot be decrypted without it", path)
		return key, nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}

	key, encoded, err := GenerateKey()
	if err != nil {
		return nil, err
	}
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if errors.Is(err, os.ErrExist) {
		return readKeyFile(path)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to create secrets key file %s: %w", path, err)
	}
	if _, err := file.WriteString(encoded + "\n"); err != nil {
		file.Close()
		return nil, fmt.Errorf("failed to write secrets key file %s: %w", path, err)
	}
	if err := file.Close(); err != nil {
		return nil, fmt.Errorf("failed to write secrets key file %s: %w", path, err)
	}
	config.Logf("SECRETS_KEY is not set; generated a new key file at %s. Set SECRETS_KEY to the file's contents and back it up: stored credentials cannot be decrypted without it", path)
	return key, nil
}

func readKeyFile(path string) (*Key, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	key, err := ParseKey(string(content))
	if err != nil {
		return nil, fmt.Errorf("secrets key file %s is invalid: %w", path, err)
	}
	return key, nil
}
