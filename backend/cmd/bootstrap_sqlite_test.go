//go:build !telemetry_ch && !transactional_pg && !telemetry_duckdb

package cmd

import (
	"database/sql"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/tracewayapp/traceway/backend/v2/app/controllers"
	"github.com/tracewayapp/traceway/backend/v2/app/db"
	"github.com/tracewayapp/traceway/backend/v2/app/dbtest"
	"github.com/tracewayapp/traceway/backend/v2/app/models"
	"github.com/tracewayapp/traceway/backend/v2/app/repositories/transactional"
	"github.com/tracewayapp/traceway/backend/v2/app/services"

	"golang.org/x/crypto/bcrypt"
)

func writeBootstrapFile(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "bootstrap.yaml")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func operatorHash(t *testing.T) string {
	t.Helper()
	hash, err := bcrypt.GenerateFromPassword([]byte("operator-password"), bcrypt.MinCost)
	if err != nil {
		t.Fatal(err)
	}
	return string(hash)
}

func TestBootstrapFileConfiguresAFreshInstance(t *testing.T) {
	dbtest.SetupSQLite(t)
	hash := operatorHash(t)
	path := writeBootstrapFile(t, `
organization:
  name: Plant 7
  timezone: Europe/Zurich
users:
  - email: admin@plant7.example
    name: Plant Admin
    role: owner
    password: correct-horse-battery
  - email: operator@plant7.example
    role: readonly
    passwordHash: `+hash+`
projects:
  - name: Line Controllers
    token: plant7-line-controllers-ingest
  - name: HMI Panels
    framework: react
    token: plant7-hmi-panels-ingest
  - name: Generated
`)

	applied, err := applyBootstrapFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !applied {
		t.Fatal("the file was not applied to an empty database")
	}

	type check struct {
		org                     *models.Organization
		admin                   *models.User
		operator                *models.User
		adminRole, operatorRole string
		projects                []*models.Project
	}
	got, err := db.ExecuteTransaction(func(tx *sql.Tx) (check, error) {
		var c check
		var err error
		if c.org, err = transactional.OrganizationRepository.FindFirst(tx); err != nil {
			return c, err
		}
		if c.admin, err = transactional.UserRepository.FindByEmail(tx, "admin@plant7.example"); err != nil {
			return c, err
		}
		if c.operator, err = transactional.UserRepository.FindByEmail(tx, "operator@plant7.example"); err != nil {
			return c, err
		}
		if c.adminRole, err = transactional.OrganizationRepository.GetUserRole(tx, c.org.Id, c.admin.Id); err != nil {
			return c, err
		}
		if c.operatorRole, err = transactional.OrganizationRepository.GetUserRole(tx, c.org.Id, c.operator.Id); err != nil {
			return c, err
		}
		c.projects, err = transactional.ProjectRepository.FindByOrganizationId(tx, c.org.Id)
		return c, err
	})
	if err != nil {
		t.Fatal(err)
	}

	if got.org.Name != "Plant 7" || got.org.Timezone != "Europe/Zurich" {
		t.Errorf("organization = %q in %q", got.org.Name, got.org.Timezone)
	}
	if got.admin.Name != "Plant Admin" || got.adminRole != "owner" || !services.CheckPassword("correct-horse-battery", got.admin.Password) {
		t.Errorf("admin = %q, role %q, password accepted %v", got.admin.Name, got.adminRole, services.CheckPassword("correct-horse-battery", got.admin.Password))
	}
	if got.operator.Name != "operator" || got.operatorRole != "readonly" || got.operator.Password != hash {
		t.Errorf("operator = %q, role %q, hash kept %v", got.operator.Name, got.operatorRole, got.operator.Password == hash)
	}

	byName := map[string]*models.Project{}
	for _, p := range got.projects {
		byName[p.Name] = p
	}
	if len(byName) != 3 {
		t.Fatalf("created %d projects, want 3", len(byName))
	}
	if p := byName["Line Controllers"]; p.Token != "plant7-line-controllers-ingest" || p.Framework != "opentelemetry" || !p.DropHealthyHealthchecks {
		t.Errorf("Line Controllers = token %q, framework %q, drop healthy healthchecks %v", p.Token, p.Framework, p.DropHealthyHealthchecks)
	}
	if p := byName["Generated"]; len(p.Token) != 32 {
		t.Errorf("a project without a token got %q, want a generated one", p.Token)
	}

	for name, template := range map[string]string{"Line Controllers": "traceway-otel-agent", "HMI Panels": "browser-performance"} {
		var count int
		err := db.DB.QueryRow(`SELECT COUNT(*) FROM project_dashboards pd JOIN dashboards d ON d.id = pd.dashboard_id
			WHERE pd.project_id = ? AND d.template_key = ?`, byName[name].Id, template).Scan(&count)
		if err != nil {
			t.Fatal(err)
		}
		if count != 1 {
			t.Errorf("%s has %d %s dashboards, want the one the dashboard would install", name, count, template)
		}
	}
}

func TestBootstrapFileIsIgnoredOnceApplied(t *testing.T) {
	dbtest.SetupSQLite(t)
	path := writeBootstrapFile(t, `
organization: {name: Plant 7}
users: [{email: admin@plant7.example, role: owner, password: correct-horse-battery}]
projects: [{name: Line Controllers, token: plant7-line-controllers-ingest}]
`)
	if _, err := applyBootstrapFile(path); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(path, []byte("organization: {name: Plant 8}\nnot valid at all: ["), 0o600); err != nil {
		t.Fatal(err)
	}
	applied, err := applyBootstrapFile(path)
	if err != nil || applied {
		t.Fatalf("second start: applied %v, err %v; want the file ignored", applied, err)
	}
	var count int
	if err := db.DB.QueryRow("SELECT COUNT(*) FROM organizations").Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("%d organizations after the second start, want 1", count)
	}
}

func TestBootstrapFileIsIgnoredAfterManualRegistration(t *testing.T) {
	dbtest.SetupSQLite(t)
	if _, err := db.ExecuteTransaction(func(tx *sql.Tx) (*models.User, error) {
		return transactional.UserRepository.Create(tx, "someone@example.test", "Someone", "")
	}); err != nil {
		t.Fatal(err)
	}
	applied, err := applyBootstrapFile(filepath.Join(t.TempDir(), "missing.yaml"))
	if err != nil || applied {
		t.Fatalf("applied %v, err %v; an instance with a user must not read the file", applied, err)
	}
}

func TestInvalidBootstrapFileCreatesNothing(t *testing.T) {
	dbtest.SetupSQLite(t)
	for name, path := range map[string]string{
		"invalid": writeBootstrapFile(t, `
organization: {name: Plant 7}
users: [{email: admin@plant7.example, role: owner, password: correct-horse-battery}]
projects: [{name: Line Controllers, token: short}]
`),
		"missing": filepath.Join(t.TempDir(), "missing.yaml"),
	} {
		_, err := applyBootstrapFile(path)
		var fileErr *bootstrapFileError
		if !errors.As(err, &fileErr) {
			t.Fatalf("%s file: err = %v, want a bootstrapFileError", name, err)
		}
		fresh, err := db.ExecuteTransaction(isFreshInstance)
		if err != nil {
			t.Fatal(err)
		}
		if !fresh {
			t.Fatalf("%s file left data behind", name)
		}
	}
}

func TestBootstrapFileRunsPostRegistrationHooksForTheOwner(t *testing.T) {
	dbtest.SetupSQLite(t)
	var hookedEmail string
	prev := controllers.PostRegistrationHooks
	controllers.PostRegistrationHooks = []func(*sql.Tx, *models.Organization, *models.User) error{
		func(_ *sql.Tx, _ *models.Organization, user *models.User) error {
			hookedEmail = user.Email
			return nil
		},
	}
	t.Cleanup(func() { controllers.PostRegistrationHooks = prev })

	path := writeBootstrapFile(t, `
organization: {name: Plant 7}
users:
  - {email: viewer@plant7.example, role: readonly, password: correct-horse-battery}
  - {email: admin@plant7.example, role: owner, password: correct-horse-battery}
`)
	if _, err := applyBootstrapFile(path); err != nil {
		t.Fatal(err)
	}
	if hookedEmail != "admin@plant7.example" {
		t.Fatalf("hook ran for %q, want the first owner", hookedEmail)
	}
}
