//go:build !telemetry_ch && !transactional_pg && !telemetry_duckdb

package controllers

import (
	"database/sql"
	"encoding/json"
	"strings"
	"testing"

	"github.com/tracewayapp/traceway/backend/app/config"
	"github.com/tracewayapp/traceway/backend/app/db"
	"github.com/tracewayapp/traceway/backend/app/models"
	"github.com/tracewayapp/traceway/backend/app/repositories/transactional"
)

// newOrgTestUser gives each case a fresh database holding exactly one user and
// no organization. createSetupTestAccount is not reusable here: it creates an
// organization, which is the thing these tests need to be absent.
func newOrgTestUser(t *testing.T, email string) (*sql.Tx, int) {
	t.Helper()
	setupSetupControllerDB(t)

	tx, err := db.DB.Begin()
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	t.Cleanup(func() { tx.Rollback() })

	user, err := transactional.UserRepository.Create(tx, email, "Test User", "hashed-password")
	if err != nil {
		t.Fatalf("create user: %v", err)
	}
	return tx, user.Id
}

// createOrganization runs the handler against a real transaction and returns
// the recorder, so each case asserts on the wire response the form sees.
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

func setCloudMode(t *testing.T, cloud bool) {
	t.Helper()
	prev := config.Config
	mode := ""
	if cloud {
		mode = "true"
	}
	config.Init(&config.Cfg{CloudMode: mode})
	t.Cleanup(func() { config.Init(prev) })
}

func TestCreateOrganizationMakesCallerOwner(t *testing.T) {
	tx, userId := newOrgTestUser(t, "solo@example.com")

	status, payload := createOrganization(t, tx, userId, `{"name":"Recovered Org","timezone":"Europe/Belgrade"}`)
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
	role, err := transactional.OrganizationRepository.GetUserRole(tx, orgId, userId)
	if err != nil {
		t.Fatalf("get role: %v", err)
	}
	if role != "owner" {
		t.Errorf("expected persisted role owner, got %q", role)
	}
}

func TestCreateOrganizationDefaultsTimezoneToUTC(t *testing.T) {
	tx, userId := newOrgTestUser(t, "notz@example.com")

	status, payload := createOrganization(t, tx, userId, `{"name":"No Timezone"}`)
	if status != 201 {
		t.Fatalf("expected 201, got %d (%v)", status, payload)
	}
	if payload["timezone"] != "UTC" {
		t.Errorf("expected UTC default, got %v", payload["timezone"])
	}
}

func TestCreateOrganizationValidation(t *testing.T) {
	tx, userId := newOrgTestUser(t, "invalid@example.com")

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
			status, payload := createOrganization(t, tx, userId, tc.body)
			if status != tc.wantStatus {
				t.Fatalf("expected %d, got %d (%v)", tc.wantStatus, status, payload)
			}
			if payload["error"] == nil || payload["error"] == "" {
				t.Errorf("expected an error message for the form, got %v", payload)
			}
		})
	}

	// A 100-rune name is the boundary and must be accepted.
	status, payload := createOrganization(t, tx, userId, `{"name":"`+strings.Repeat("a", 100)+`"}`)
	if status != 201 {
		t.Fatalf("expected 100-char name to be accepted, got %d (%v)", status, payload)
	}
}

// Self-hosted instances allow exactly one organization. Without this the route,
// which carries no role guard, would let any authenticated user own a new one.
func TestCreateOrganizationSelfHostedAllowsOnlyOne(t *testing.T) {
	tx, userId := newOrgTestUser(t, "selfhosted@example.com")
	setCloudMode(t, false)

	status, payload := createOrganization(t, tx, userId, `{"name":"First Org"}`)
	if status != 201 {
		t.Fatalf("first organization should be allowed, got %d (%v)", status, payload)
	}

	status, payload = createOrganization(t, tx, userId, `{"name":"Second Org"}`)
	if status != 422 {
		t.Fatalf("expected 422 for a second self-hosted organization, got %d (%v)", status, payload)
	}
	if payload["error"] == nil {
		t.Error("expected an actionable message pointing at an administrator")
	}
}

func TestCreateOrganizationCloudAllowsMoreThanOne(t *testing.T) {
	tx, userId := newOrgTestUser(t, "cloud@example.com")
	setCloudMode(t, true)

	if status, payload := createOrganization(t, tx, userId, `{"name":"First Org"}`); status != 201 {
		t.Fatalf("first organization: expected 201, got %d (%v)", status, payload)
	}
	if status, payload := createOrganization(t, tx, userId, `{"name":"Second Org"}`); status != 201 {
		t.Fatalf("cloud must allow a second organization, got %d (%v)", status, payload)
	}
}

// Register and FinishSetup run these for every new org+owner pair; cloud wires
// provisioning there, so an organization created here must not skip them.
func TestCreateOrganizationRunsPostRegistrationHooks(t *testing.T) {
	tx, userId := newOrgTestUser(t, "hooks@example.com")

	var gotOrg *models.Organization
	var gotUser *models.User
	prev := PostRegistrationHooks
	PostRegistrationHooks = []func(*sql.Tx, *models.Organization, *models.User) error{
		func(_ *sql.Tx, org *models.Organization, user *models.User) error {
			gotOrg, gotUser = org, user
			return nil
		},
	}
	t.Cleanup(func() { PostRegistrationHooks = prev })

	status, payload := createOrganization(t, tx, userId, `{"name":"Hooked Org"}`)
	if status != 201 {
		t.Fatalf("expected 201, got %d (%v)", status, payload)
	}
	if gotOrg == nil || gotUser == nil {
		t.Fatal("PostRegistrationHooks did not run for an organization created here")
	}
	if gotOrg.Name != "Hooked Org" {
		t.Errorf("hook got org %q", gotOrg.Name)
	}
	if gotUser.Id != userId {
		t.Errorf("hook got user %d, want %d", gotUser.Id, userId)
	}
}

func TestCreateOrganizationLimitHookBlocks(t *testing.T) {
	tx, userId := newOrgTestUser(t, "capped@example.com")

	prevHook := OrganizationLimitHook
	gotUserId := 0
	OrganizationLimitHook = func(_ *sql.Tx, id int) error {
		gotUserId = id
		return &LimitExceededError{Message: "Your plan allows 1 organization"}
	}
	t.Cleanup(func() { OrganizationLimitHook = prevHook })

	status, payload := createOrganization(t, tx, userId, `{"name":"Second Org"}`)
	if status != 422 {
		t.Fatalf("expected 422 from the limit hook, got %d (%v)", status, payload)
	}
	if payload["error"] != "Your plan allows 1 organization" {
		t.Errorf("expected the hook's message to reach the form, got %v", payload["error"])
	}
	if gotUserId != userId {
		t.Errorf("hook should be keyed on the creating user, got %d want %d", gotUserId, userId)
	}

	orgs, err := transactional.OrganizationRepository.FindByUserId(tx, userId)
	if err != nil {
		t.Fatalf("find orgs: %v", err)
	}
	if len(orgs) != 0 {
		t.Errorf("blocked request must not create an organization, found %d", len(orgs))
	}
}
