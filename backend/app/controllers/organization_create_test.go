//go:build !telemetry_ch && !transactional_pg && !telemetry_duckdb

package controllers

import (
	"database/sql"
	"encoding/json"
	"strings"
	"testing"

	"github.com/tracewayapp/traceway/backend/app/db"
	"github.com/tracewayapp/traceway/backend/app/repositories/transactional"
)

// createOrganization runs the handler against a real transaction and returns
// the recorder, so each case asserts on the wire response the dialog sees.
func createOrganization(t *testing.T, tx *sql.Tx, userId int, body string) (int, map[string]any) {
	t.Helper()
	c, recorder := newControllerTestContext(t, tx, userId, "POST", "/organizations", body)
	OrganizationController.Create(c)

	var payload map[string]any
	if recorder.Body.Len() > 0 {
		if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
			t.Fatalf("decode response %q: %v", recorder.Body.String(), err)
		}
	}
	return recorder.Code, payload
}

func TestCreateOrganizationMakesCallerOwner(t *testing.T) {
	setupSetupControllerDB(t)

	tx, err := db.DB.Begin()
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	defer tx.Rollback()

	user, err := transactional.UserRepository.Create(tx, "solo@example.com", "Solo User", "hashed-password")
	if err != nil {
		t.Fatalf("create user: %v", err)
	}

	status, payload := createOrganization(t, tx, user.Id, `{"name":"Recovered Org","timezone":"Europe/Belgrade"}`)
	if status != 201 {
		t.Fatalf("expected 201, got %d (%v)", status, payload)
	}
	if payload["role"] != "owner" {
		t.Errorf("expected creator to be owner, got %v", payload["role"])
	}
	if payload["name"] != "Recovered Org" {
		t.Errorf("unexpected name: %v", payload["name"])
	}
	if payload["timezone"] != "Europe/Belgrade" {
		t.Errorf("unexpected timezone: %v", payload["timezone"])
	}

	// The whole point of the endpoint: the user can now reach a usable account.
	orgId := int(payload["id"].(float64))
	role, err := transactional.OrganizationRepository.GetUserRole(tx, orgId, user.Id)
	if err != nil {
		t.Fatalf("get role: %v", err)
	}
	if role != "owner" {
		t.Errorf("expected persisted role owner, got %q", role)
	}
}

func TestCreateOrganizationDefaultsTimezoneToUTC(t *testing.T) {
	setupSetupControllerDB(t)

	tx, err := db.DB.Begin()
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	defer tx.Rollback()

	user, err := transactional.UserRepository.Create(tx, "notz@example.com", "No TZ", "hashed-password")
	if err != nil {
		t.Fatalf("create user: %v", err)
	}

	status, payload := createOrganization(t, tx, user.Id, `{"name":"No Timezone"}`)
	if status != 201 {
		t.Fatalf("expected 201, got %d (%v)", status, payload)
	}
	if payload["timezone"] != "UTC" {
		t.Errorf("expected UTC default, got %v", payload["timezone"])
	}
}

func TestCreateOrganizationValidation(t *testing.T) {
	setupSetupControllerDB(t)

	tx, err := db.DB.Begin()
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	defer tx.Rollback()

	user, err := transactional.UserRepository.Create(tx, "invalid@example.com", "Invalid", "hashed-password")
	if err != nil {
		t.Fatalf("create user: %v", err)
	}

	cases := []struct {
		name       string
		body       string
		wantStatus int
	}{
		{"missing name", `{}`, 400},
		{"blank name", `{"name":"   "}`, 422},
		{"name too long", `{"name":"` + strings.Repeat("a", 101) + `"}`, 422},
		{"unknown timezone", `{"name":"Fine","timezone":"Mars/Olympus"}`, 422},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			status, payload := createOrganization(t, tx, user.Id, tc.body)
			if status != tc.wantStatus {
				t.Fatalf("expected %d, got %d (%v)", tc.wantStatus, status, payload)
			}
			if payload["error"] == nil || payload["error"] == "" {
				t.Errorf("expected an error message for the dialog, got %v", payload)
			}
		})
	}

	// A 100-rune name is the boundary and must be accepted.
	status, payload := createOrganization(t, tx, user.Id, `{"name":"`+strings.Repeat("a", 100)+`"}`)
	if status != 201 {
		t.Fatalf("expected 100-char name to be accepted, got %d (%v)", status, payload)
	}
}

func TestCreateOrganizationLimitHookBlocks(t *testing.T) {
	setupSetupControllerDB(t)

	tx, err := db.DB.Begin()
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	defer tx.Rollback()

	user, err := transactional.UserRepository.Create(tx, "capped@example.com", "Capped", "hashed-password")
	if err != nil {
		t.Fatalf("create user: %v", err)
	}

	prevHook := OrganizationLimitHook
	gotUserId := 0
	OrganizationLimitHook = func(_ *sql.Tx, userId int) error {
		gotUserId = userId
		return &LimitExceededError{Message: "Your plan allows 1 organization"}
	}
	t.Cleanup(func() { OrganizationLimitHook = prevHook })

	status, payload := createOrganization(t, tx, user.Id, `{"name":"Second Org"}`)
	if status != 422 {
		t.Fatalf("expected 422 from the limit hook, got %d (%v)", status, payload)
	}
	if payload["error"] != "Your plan allows 1 organization" {
		t.Errorf("expected the hook's message to reach the dialog, got %v", payload["error"])
	}
	if gotUserId != user.Id {
		t.Errorf("hook should be keyed on the creating user, got %d want %d", gotUserId, user.Id)
	}

	orgs, err := transactional.OrganizationRepository.FindByUserId(tx, user.Id)
	if err != nil {
		t.Fatalf("find orgs: %v", err)
	}
	if len(orgs) != 0 {
		t.Errorf("blocked request must not create an organization, found %d", len(orgs))
	}
}
