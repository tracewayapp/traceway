package backfill

import (
	"bytes"
	"database/sql"
	"fmt"

	"github.com/tracewayapp/traceway/backend/app/config"
	"github.com/tracewayapp/traceway/backend/app/db"
	"github.com/tracewayapp/traceway/backend/app/notifications"
	"github.com/tracewayapp/traceway/backend/app/repositories/transactional"
)

// RunSecretsEncryption encrypts the credentials of every notification channel
// still stored in plaintext. Encrypted rows are untouched, so it runs on every
// boot: that also catches rows a binary from before encryption wrote after a
// rollback.
func RunSecretsEncryption() error {
	encrypted, err := db.ExecuteTransaction(func(tx *sql.Tx) (int, error) {
		if !db.IsSQLite() {
			if _, err := tx.Exec("SELECT pg_advisory_xact_lock(824737005)"); err != nil {
				return 0, fmt.Errorf("failed to take backfill lock: %w", err)
			}
		}
		channels, err := transactional.NotificationChannelRepository.FindAll(tx)
		if err != nil {
			return 0, fmt.Errorf("failed to list notification channels: %w", err)
		}
		encrypted := 0
		for _, channel := range channels {
			next, err := notifications.EncryptSecretFields(channel.ChannelType, channel.Config)
			if err != nil {
				return 0, fmt.Errorf("failed to encrypt notification channel %d: %w", channel.Id, err)
			}
			if bytes.Equal(next, channel.Config) {
				continue
			}
			channel.Config = next
			if err := transactional.NotificationChannelRepository.Update(tx, channel); err != nil {
				return 0, fmt.Errorf("failed to update notification channel %d: %w", channel.Id, err)
			}
			encrypted++
		}
		return encrypted, nil
	})
	if err != nil {
		return err
	}
	if encrypted > 0 {
		config.Logf("secrets backfill: encrypted the credentials of %d notification channel(s)", encrypted)
	}
	return nil
}
