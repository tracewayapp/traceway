package notifications

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"github.com/tracewayapp/traceway/backend/app/models"
	"github.com/tracewayapp/traceway/backend/app/secrets"
)

func initTestSecrets(t *testing.T) {
	t.Helper()
	key, _, err := secrets.GenerateKey()
	if err != nil {
		t.Fatal(err)
	}
	secrets.Init(key)
	t.Cleanup(func() { secrets.Init(nil) })
}

func configMap(t *testing.T, cfg json.RawMessage) map[string]any {
	t.Helper()
	var values map[string]any
	if err := json.Unmarshal(cfg, &values); err != nil {
		t.Fatalf("unmarshal %s: %v", cfg, err)
	}
	return values
}

func TestEncryptSecretFieldsEncryptsOnlyCredentials(t *testing.T) {
	initTestSecrets(t)
	cfg := json.RawMessage(`{"userKey":"user-1","appToken":"app-1","device":"phone","priority":2}`)

	encrypted, err := EncryptSecretFields("pushover", cfg)
	if err != nil {
		t.Fatal(err)
	}
	values := configMap(t, encrypted)
	for _, field := range []string{"userKey", "appToken"} {
		if !secrets.IsEncrypted(values[field].(string)) {
			t.Errorf("%s = %v, want ciphertext", field, values[field])
		}
	}
	if values["device"] != "phone" || values["priority"] != float64(2) {
		t.Errorf("non-secret fields changed: %v", values)
	}

	again, err := EncryptSecretFields("pushover", encrypted)
	if err != nil {
		t.Fatal(err)
	}
	if string(again) != string(encrypted) {
		t.Fatal("encrypting an encrypted config changed it")
	}

	decrypted, err := DecryptSecretFields("pushover", encrypted)
	if err != nil {
		t.Fatal(err)
	}
	if got := configMap(t, decrypted); got["userKey"] != "user-1" || got["appToken"] != "app-1" {
		t.Errorf("decrypt returned %v", got)
	}
}

func TestSecretFieldsLeaveOtherShapesAlone(t *testing.T) {
	initTestSecrets(t)
	for _, tc := range []struct {
		channelType string
		cfg         string
	}{
		{"email", `{"recipients":["a@example.com"]}`},
		{"escalation", `{"policyId":3}`},
		{"github", `{"token":"","owner":"o","repo":"r"}`},
		{"github", `{"owner":"o","repo":"r"}`},
		{"github", `{"token":42}`},
		{"github", ``},
	} {
		out, err := EncryptSecretFields(tc.channelType, json.RawMessage(tc.cfg))
		if err != nil {
			t.Errorf("%s %s: %v", tc.channelType, tc.cfg, err)
		}
		if string(out) != tc.cfg {
			t.Errorf("%s %s: changed to %s", tc.channelType, tc.cfg, out)
		}
	}
	if _, err := EncryptSecretFields("github", json.RawMessage(`not json`)); err == nil {
		t.Error("malformed config accepted")
	}
}

func TestMaskSecretFields(t *testing.T) {
	initTestSecrets(t)
	encrypted, _ := EncryptSecretFields("github", json.RawMessage(`{"token":"ghp_x","owner":"o","repo":"r"}`))
	masked, fields, err := MaskSecretFields("github", encrypted)
	if err != nil {
		t.Fatal(err)
	}
	values := configMap(t, masked)
	if values["token"] != SecretSentinel || values["owner"] != "o" {
		t.Errorf("masked = %v", values)
	}
	if !reflect.DeepEqual(fields, []string{"token"}) {
		t.Errorf("fields = %v", fields)
	}
	if strings.Contains(string(masked), "ghp_x") {
		t.Error("plaintext leaked into the masked config")
	}

	_, fields, _ = MaskSecretFields("github", json.RawMessage(`{"owner":"o"}`))
	if len(fields) != 0 {
		t.Errorf("fields for a config without a token = %v", fields)
	}
}

func TestKeepStoredSecrets(t *testing.T) {
	initTestSecrets(t)
	stored, _ := EncryptSecretFields("pushover", json.RawMessage(`{"userKey":"user-1","appToken":"app-1"}`))
	storedValues := configMap(t, stored)

	merged, err := KeepStoredSecrets("pushover", json.RawMessage(`{"userKey":"********","appToken":"app-2","device":"d"}`), stored)
	if err != nil {
		t.Fatal(err)
	}
	values := configMap(t, merged)
	if values["userKey"] != storedValues["userKey"] {
		t.Errorf("userKey = %v, want the stored ciphertext", values["userKey"])
	}
	if values["appToken"] != "app-2" || values["device"] != "d" {
		t.Errorf("merged = %v", values)
	}

	orphan, err := KeepStoredSecrets("pushover", json.RawMessage(`{"userKey":"********","appToken":"a"}`), nil)
	if err != nil {
		t.Fatal(err)
	}
	if configMap(t, orphan)["userKey"] != "" {
		t.Errorf("sentinel without a stored value = %v", configMap(t, orphan)["userKey"])
	}
}

func TestNewAdapterDecryptsStoredCredentials(t *testing.T) {
	initTestSecrets(t)
	encrypted, _ := EncryptSecretFields("github", json.RawMessage(`{"token":"ghp_x","owner":"o","repo":"r"}`))
	adapter, err := NewAdapter("github", encrypted)
	if err != nil {
		t.Fatal(err)
	}
	if adapter.(*GitHubAdapter).Token != "ghp_x" {
		t.Errorf("token = %q", adapter.(*GitHubAdapter).Token)
	}

	other, _, _ := secrets.GenerateKey()
	secrets.Init(other)
	if _, err := NewAdapter("github", encrypted); !errors.Is(err, secrets.ErrWrongKey) {
		t.Errorf("wrong key: got %v", err)
	}
}

func TestAdapterSendDeliversWithDecryptedSnapshot(t *testing.T) {
	initTestSecrets(t)
	var gotSignature string
	var gotBody []byte
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotSignature = r.Header.Get("X-Traceway-Signature")
		gotBody, _ = io.ReadAll(r.Body)
	}))
	defer server.Close()

	snapshot, err := EncryptSecretFields("webhook", json.RawMessage(`{"url":"`+server.URL+`","secret":"hmac-secret"}`))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(snapshot), "v1:") || strings.Contains(string(snapshot), "hmac-secret") {
		t.Fatalf("snapshot is not encrypted: %s", snapshot)
	}
	if err := AdapterSend(context.Background(), "webhook", snapshot, models.NotificationMessage{Subject: "s", Body: "b"}); err != nil {
		t.Fatal(err)
	}
	mac := hmac.New(sha256.New, []byte("hmac-secret"))
	mac.Write(gotBody)
	if want := "sha256=" + hex.EncodeToString(mac.Sum(nil)); gotSignature != want {
		t.Errorf("signature = %q, want %q (the send did not use the decrypted secret)", gotSignature, want)
	}
}
