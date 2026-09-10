//go:build !telemetry_ch && !transactional_pg && !telemetry_duckdb

package controllers

import (
	"crypto/hmac"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strconv"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/tracewayapp/traceway/backend/app/db"
	"github.com/tracewayapp/traceway/backend/app/middleware"
	"github.com/tracewayapp/traceway/backend/app/models"
	"github.com/tracewayapp/traceway/backend/app/notifications"
	"github.com/tracewayapp/traceway/backend/app/repositories/transactional"
	"github.com/tracewayapp/traceway/backend/app/secrets"
)

func setupChannelControllerDB(t *testing.T) {
	t.Helper()
	setupSetupControllerDB(t)
	key, _, err := secrets.GenerateKey()
	if err != nil {
		t.Fatal(err)
	}
	secrets.Init(key)
	t.Cleanup(func() { secrets.Init(nil) })
}

func createChannelTestProject(t *testing.T, tx *sql.Tx, orgId int) uuid.UUID {
	t.Helper()
	project, err := transactional.ProjectRepository.CreateWithOrganization(tx, "Project", "opentelemetry", orgId)
	if err != nil {
		t.Fatalf("create project: %v", err)
	}
	return project.Id
}

func channelRequest(t *testing.T, tx *sql.Tx, projectId uuid.UUID, userId int, method, path, body string) (*gin.Context, *httptest.ResponseRecorder) {
	t.Helper()
	c, recorder := newControllerTestContext(t, tx, userId, method, path, body)
	c.Set(middleware.ProjectIdContextKey, projectId)
	return c, recorder
}

func decodeChannel(t *testing.T, body string) map[string]any {
	t.Helper()
	var response map[string]any
	if err := json.Unmarshal([]byte(body), &response); err != nil {
		t.Fatalf("decode %s: %v", body, err)
	}
	return response
}

func storedChannelConfig(t *testing.T, tx *sql.Tx, id int) map[string]any {
	t.Helper()
	channel, err := transactional.NotificationChannelRepository.FindById(tx, id)
	if err != nil || channel == nil {
		t.Fatalf("find channel: %v", err)
	}
	var config map[string]any
	if err := json.Unmarshal(channel.Config, &config); err != nil {
		t.Fatalf("decode stored config: %v", err)
	}
	return config
}

func TestChannelCredentialsAreEncryptedAndMasked(t *testing.T) {
	setupChannelControllerDB(t)
	tx, err := db.DB.Begin()
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	t.Cleanup(func() { tx.Rollback() })

	ownerId, orgId := createSetupTestAccount(t, tx, "owner@example.com", "owner")
	readonlyId := addOrgMember(t, tx, orgId, "reader@example.com", "readonly")
	projectId := createChannelTestProject(t, tx, orgId)

	c, rec := channelRequest(t, tx, projectId, ownerId, "POST", "/notification-channels", `{"name":"GitHub","channelType":"github","config":{"token":"ghp_first","owner":"acme","repo":"app","labels":["bug"]}}`)
	NotificationChannelController.Create(c)
	if rec.Code != 201 {
		t.Fatalf("create: expected 201, got %d %s", rec.Code, rec.Body.String())
	}
	created := decodeChannel(t, rec.Body.String())
	if strings.Contains(rec.Body.String(), "ghp_first") {
		t.Fatalf("create response echoed the token: %s", rec.Body.String())
	}
	channelId := int(created["id"].(float64))

	stored := storedChannelConfig(t, tx, channelId)
	token, _ := stored["token"].(string)
	if !secrets.IsEncrypted(token) {
		t.Fatalf("stored token = %q, want ciphertext", token)
	}
	if stored["owner"] != "acme" {
		t.Fatalf("stored config lost non-secret fields: %v", stored)
	}
	firstCiphertext := token

	c, rec = channelRequest(t, tx, projectId, readonlyId, "GET", "/notification-channels", "")
	NotificationChannelController.List(c)
	if rec.Code != 200 {
		t.Fatalf("list: expected 200, got %d %s", rec.Code, rec.Body.String())
	}
	if strings.Contains(rec.Body.String(), "ghp_first") || strings.Contains(rec.Body.String(), "v1:") {
		t.Fatalf("list leaked the credential: %s", rec.Body.String())
	}
	listed := decodeChannel(t, rec.Body.String())["channels"].([]any)[0].(map[string]any)
	if listed["config"].(map[string]any)["token"] != notifications.SecretSentinel {
		t.Fatalf("listed token = %v, want the sentinel", listed["config"])
	}
	if !reflect.DeepEqual(listed["hasSecrets"], []any{"token"}) {
		t.Fatalf("hasSecrets = %v", listed["hasSecrets"])
	}

	path := "/notification-channels/" + strconv.Itoa(channelId)
	c, rec = channelRequest(t, tx, projectId, ownerId, "PUT", path, `{"name":"GitHub renamed","channelType":"github","config":{"token":"********","owner":"acme","repo":"app"}}`)
	c.Params = gin.Params{{Key: "id", Value: strconv.Itoa(channelId)}}
	NotificationChannelController.Update(c)
	if rec.Code != 200 {
		t.Fatalf("update with sentinel: expected 200, got %d %s", rec.Code, rec.Body.String())
	}
	if got := storedChannelConfig(t, tx, channelId)["token"]; got != firstCiphertext {
		t.Fatalf("update with the sentinel replaced the stored token: %v", got)
	}
	if plaintext, _ := secrets.Decrypt(firstCiphertext); string(plaintext) != "ghp_first" {
		t.Fatalf("stored token decrypts to %q", plaintext)
	}

	c, rec = channelRequest(t, tx, projectId, ownerId, "PUT", path, `{"name":"GitHub","channelType":"github","config":{"token":"ghp_second","owner":"acme","repo":"app"}}`)
	c.Params = gin.Params{{Key: "id", Value: strconv.Itoa(channelId)}}
	NotificationChannelController.Update(c)
	if rec.Code != 200 {
		t.Fatalf("update with a new token: expected 200, got %d %s", rec.Code, rec.Body.String())
	}
	replaced, _ := storedChannelConfig(t, tx, channelId)["token"].(string)
	if replaced == firstCiphertext || !secrets.IsEncrypted(replaced) {
		t.Fatalf("new token not stored encrypted: %q", replaced)
	}
	if plaintext, _ := secrets.Decrypt(replaced); string(plaintext) != "ghp_second" {
		t.Fatalf("replaced token decrypts to %q", plaintext)
	}

	c, rec = channelRequest(t, tx, projectId, ownerId, "PUT", path, `{"name":"Telegram","channelType":"telegram","config":{"botToken":"********","chatId":"1"}}`)
	c.Params = gin.Params{{Key: "id", Value: strconv.Itoa(channelId)}}
	NotificationChannelController.Update(c)
	if rec.Code != 422 {
		t.Fatalf("sentinel across a type change: expected 422, got %d %s", rec.Code, rec.Body.String())
	}
}

func TestChannelTestSendsWithDecryptedCredential(t *testing.T) {
	setupChannelControllerDB(t)
	var gotSignature string
	var gotBody []byte
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotSignature = r.Header.Get("X-Traceway-Signature")
		gotBody, _ = io.ReadAll(r.Body)
	}))
	defer server.Close()

	tx, err := db.DB.Begin()
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	ownerId, orgId := createSetupTestAccount(t, tx, "owner@example.com", "owner")
	projectId := createChannelTestProject(t, tx, orgId)
	c, rec := channelRequest(t, tx, projectId, ownerId, "POST", "/notification-channels", `{"name":"Hook","channelType":"webhook","config":{"url":"`+server.URL+`","secret":"hmac-secret"}}`)
	NotificationChannelController.Create(c)
	if rec.Code != 201 {
		t.Fatalf("create: expected 201, got %d %s", rec.Code, rec.Body.String())
	}
	channelId := int(decodeChannel(t, rec.Body.String())["id"].(float64))
	stored := storedChannelConfig(t, tx, channelId)
	if secret, _ := stored["secret"].(string); !secrets.IsEncrypted(secret) {
		t.Fatalf("stored secret = %q, want ciphertext", secret)
	}
	if err := tx.Commit(); err != nil {
		t.Fatalf("commit: %v", err)
	}

	c, rec = channelRequest(t, nil, projectId, ownerId, "POST", "/notification-channels/"+strconv.Itoa(channelId)+"/test", "")
	c.Params = gin.Params{{Key: "id", Value: strconv.Itoa(channelId)}}
	NotificationChannelController.Test(c)
	if rec.Code != 200 {
		t.Fatalf("test: expected 200, got %d %s", rec.Code, rec.Body.String())
	}
	mac := hmac.New(sha256.New, []byte("hmac-secret"))
	mac.Write(gotBody)
	if want := "sha256=" + hex.EncodeToString(mac.Sum(nil)); gotSignature != want {
		t.Fatalf("signature = %q, want %q", gotSignature, want)
	}
	var payload models.NotificationMessage
	if err := json.Unmarshal(gotBody, &payload); err != nil || payload.Subject == "" {
		t.Fatalf("delivered payload = %s (%v)", gotBody, err)
	}
}
