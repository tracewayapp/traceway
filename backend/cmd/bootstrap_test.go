package cmd

import (
	"strings"
	"testing"
)

func TestParseBootstrapFileFillsDefaults(t *testing.T) {
	file, err := parseBootstrapFile([]byte(`
organization:
  name: Plant 7
users:
  - email: admin@plant7.example
    role: owner
    password: correct-horse-battery
projects:
  - name: Line Controllers
    token: plant7-line-controllers-ingest
`))
	if err != nil {
		t.Fatal(err)
	}
	if file.Organization.Timezone != "UTC" {
		t.Errorf("timezone = %q, want UTC", file.Organization.Timezone)
	}
	if file.Users[0].Name != "admin" {
		t.Errorf("user name = %q, want the part of the email before @", file.Users[0].Name)
	}
	if file.Projects[0].Framework != "opentelemetry" {
		t.Errorf("framework = %q, want opentelemetry", file.Projects[0].Framework)
	}
}

func TestParseBootstrapFileAcceptsJSON(t *testing.T) {
	file, err := parseBootstrapFile([]byte(`{
		"organization": {"name": "Plant 7", "timezone": "Europe/Zurich"},
		"users": [{"email": "admin@plant7.example", "role": "owner", "password": "correct-horse-battery"}],
		"projects": [{"name": "HMI Panels", "framework": "react", "token": "plant7-hmi-panels-ingest"}]
	}`))
	if err != nil {
		t.Fatal(err)
	}
	if file.Projects[0].Token != "plant7-hmi-panels-ingest" {
		t.Errorf("token = %q", file.Projects[0].Token)
	}
}

func TestParseBootstrapFileKeepsNumericTokensAsWritten(t *testing.T) {
	file, err := parseBootstrapFile([]byte(`
organization: {name: Plant 7}
users: [{email: admin@plant7.example, role: owner, password: correct-horse-battery}]
projects: [{name: Sensors, token: 00012345678901234567}]
`))
	if err != nil {
		t.Fatal(err)
	}
	if file.Projects[0].Token != "00012345678901234567" {
		t.Errorf("token = %q, want the digits as written", file.Projects[0].Token)
	}
}

func TestParseBootstrapFileRejectsUnknownFields(t *testing.T) {
	_, err := parseBootstrapFile([]byte(`
organization:
  name: Plant 7
users:
  - email: admin@plant7.example
    role: owner
    passwrd: correct-horse-battery
`))
	if err == nil {
		t.Fatal("a misspelled field was accepted")
	}
	msg := err.Error()
	if !strings.Contains(msg, "line 7") || !strings.Contains(msg, "passwrd") {
		t.Errorf("error does not point at the misspelled field: %s", msg)
	}
	if strings.Contains(msg, "cmd.") {
		t.Errorf("error leaks a Go type name: %s", msg)
	}
}

func TestParseBootstrapFileRejectsEmptyFile(t *testing.T) {
	_, err := parseBootstrapFile([]byte("  \n"))
	if err == nil || !strings.Contains(err.Error(), "empty") {
		t.Fatalf("err = %v, want an empty-file error", err)
	}
}

func TestParseBootstrapFileReportsEveryProblem(t *testing.T) {
	_, err := parseBootstrapFile([]byte(`
organization:
  timezone: Mars/Olympus
users:
  - email: Plant Admin <admin@plant7.example>
    role: superuser
    password: short
  - email: ops@plant7.example
    role: admin
  - email: OPS@plant7.example
    role: user
    password: correct-horse-battery
    passwordHash: $2a$12$abc
  - email: viewer@plant7.example
    role: readonly
    passwordHash: not-a-hash
projects:
  - name: Line/Controllers
  - name: Sensors
    framework: cobol
  - name: Gateway
    token: too-short
  - name: gateway
    token: plant7-shared-ingest-token
  - name: Edge
    token: plant7-shared-ingest-token
    sourceMapToken: plant7 has spaces in it
`))
	if err == nil {
		t.Fatal("an invalid file was accepted")
	}
	for _, want := range []string{
		"organization.name: must be between 1 and 100 characters",
		`organization.timezone: Unknown timezone "Mars/Olympus".`,
		`users[0].email: "Plant Admin <admin@plant7.example>" is not an email address`,
		"users[0].role: must be owner, admin, user or readonly",
		"users[0].password: must be at least 8 characters",
		"users[1]: password or passwordHash is required",
		"users[2].email: OPS@plant7.example is already used by users[1]",
		"users[2]: set password or passwordHash, not both",
		"users[3].passwordHash: is not a bcrypt hash",
		"users: at least one user needs the role owner",
		"projects[0]: Project name can only contain",
		"projects[1]: Framework must be one of",
		"projects[2].token: must be 16 to 128 characters",
		`projects[3].name: "gateway" is already used by projects[2]`,
		"projects[4].token: is the same as projects[3].token",
		"projects[4].sourceMapToken: must be 16 to 128 characters",
	} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("missing problem %q in:\n%s", want, err)
		}
	}
}

func TestParseBootstrapFileRequiresUsers(t *testing.T) {
	_, err := parseBootstrapFile([]byte("organization:\n  name: Plant 7\n"))
	if err == nil || !strings.Contains(err.Error(), "users: at least one user is required") {
		t.Fatalf("err = %v, want a missing-users error", err)
	}
}

func TestParseBootstrapFileRejectsPasswordsBcryptCannotHash(t *testing.T) {
	_, err := parseBootstrapFile([]byte(`
organization: {name: Plant 7}
users: [{email: admin@plant7.example, role: owner, password: ` + strings.Repeat("x", 73) + `}]
`))
	if err == nil || !strings.Contains(err.Error(), "users[0].password: must be at most 72 bytes") {
		t.Fatalf("err = %v, want a password length error", err)
	}
}
