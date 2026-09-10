//go:build !transactional_pg && !telemetry_ch && !telemetry_duckdb

package backfill

import (
	"database/sql"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/tracewayapp/traceway/backend/app/db"
	"github.com/tracewayapp/traceway/backend/app/dbtest"
	"github.com/tracewayapp/traceway/backend/app/models"
	"github.com/tracewayapp/traceway/backend/app/repositories/transactional"
	"github.com/tracewayapp/traceway/backend/app/secrets"
)

func TestRunSecretsEncryptionEncryptsPlaintextRowsOnce(t *testing.T) {
	dbtest.SetupSQLite(t)
	key, _, err := secrets.GenerateKey()
	if err != nil {
		t.Fatal(err)
	}
	secrets.Init(key)
	t.Cleanup(func() { secrets.Init(nil) })

	projectId := createBackfillProject(t)
	plaintextId := insertChannel(t, projectId, "github", `{"token":"ghp_plain","owner":"o","repo":"r"}`)
	emailId := insertChannel(t, projectId, "email", `{"recipients":["a@example.com"]}`)

	if err := RunSecretsEncryption(); err != nil {
		t.Fatalf("first run: %v", err)
	}
	encrypted := channelConfig(t, plaintextId)
	if strings.Contains(encrypted, "ghp_plain") || !strings.Contains(encrypted, `"token":"v1:`) {
		t.Fatalf("github config after backfill = %s", encrypted)
	}
	if got := channelConfig(t, emailId); got != `{"recipients":["a@example.com"]}` {
		t.Fatalf("email config changed to %s", got)
	}

	if err := RunSecretsEncryption(); err != nil {
		t.Fatalf("second run: %v", err)
	}
	if again := channelConfig(t, plaintextId); again != encrypted {
		t.Fatalf("second run re-encrypted the row: %s != %s", again, encrypted)
	}
}

func createBackfillProject(t *testing.T) uuid.UUID {
	t.Helper()
	projectId, err := db.ExecuteTransaction(func(tx *sql.Tx) (uuid.UUID, error) {
		org, err := transactional.OrganizationRepository.Create(tx, "Org", "UTC")
		if err != nil {
			return uuid.Nil, err
		}
		project, err := transactional.ProjectRepository.CreateWithOrganization(tx, "Project", "opentelemetry", org.Id)
		if err != nil {
			return uuid.Nil, err
		}
		return project.Id, nil
	})
	if err != nil {
		t.Fatalf("create project: %v", err)
	}
	return projectId
}

func insertChannel(t *testing.T, projectId uuid.UUID, channelType, config string) int {
	t.Helper()
	id, err := db.ExecuteTransaction(func(tx *sql.Tx) (int, error) {
		now := time.Now().UTC()
		return transactional.NotificationChannelRepository.Create(tx, &models.NotificationChannel{
			ProjectId:   projectId,
			Name:        channelType,
			ChannelType: channelType,
			Config:      json.RawMessage(config),
			Enabled:     true,
			CreatedAt:   now,
			UpdatedAt:   now,
		})
	})
	if err != nil {
		t.Fatalf("insert channel: %v", err)
	}
	return id
}

func channelConfig(t *testing.T, id int) string {
	t.Helper()
	channel, err := db.ExecuteTransaction(func(tx *sql.Tx) (*models.NotificationChannel, error) {
		return transactional.NotificationChannelRepository.FindById(tx, id)
	})
	if err != nil || channel == nil {
		t.Fatalf("find channel %d: %v", id, err)
	}
	return string(channel.Config)
}
