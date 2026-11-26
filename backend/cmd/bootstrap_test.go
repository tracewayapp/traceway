package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tracewayapp/traceway/backend/v2/app/services"
)

const bootstrapTestHash = "$2b$05$OtYGnd3qinWuQQSZeIoLw.dBPaC66xPW0SJTuIJIRTvymMHT/gldS"

func writeBootstrapFile(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "bootstrap.yaml")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func bootstrapWithUser(user string) []byte {
	return []byte("organization: {name: Plant 7}\nusers: [{" + user + "}]\n")
}

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
		"users[3].passwordHash: is not a complete bcrypt hash",
		"users: at least one user needs the role owner",
		"projects[0]: Project name can only contain",
		"projects[1]: Framework must be one of",
		"projects[2].token: must be 16 to 64 characters",
		`projects[3].name: "gateway" is already used by projects[2]`,
		"projects[4].token: is the same as projects[3].token",
		"projects[4].sourceMapToken: must be 16 to 64 characters",
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

func TestParseBootstrapFileRejectsEmailsTheSignInFormRefuses(t *testing.T) {
	for _, email := range []string{"root@localhost", "admin@plant7", "admin@10.0.0.1"} {
		_, err := parseBootstrapFile(bootstrapWithUser("email: " + email + ", role: owner, password: correct-horse-battery"))
		if err == nil || !strings.Contains(err.Error(), "users[0].email") {
			t.Errorf("%s: err = %v, want it refused, since POST /api/login would never accept it", email, err)
		}
	}
}

func TestParseBootstrapFileRejectsValuesTooLongForPostgres(t *testing.T) {
	long := strings.Repeat("a", 256)
	_, err := parseBootstrapFile(bootstrapWithUser("email: " + long + "@plant7.example, name: " + long + ", role: owner, password: correct-horse-battery"))
	if err == nil {
		t.Fatal("a 256 character email and name were accepted")
	}
	for _, want := range []string{"users[0].email: must be at most 255 characters", "users[0].name: must be at most 255 characters"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("missing problem %q in:\n%s", want, err)
		}
	}

	projects := func(token string) []byte {
		return append(bootstrapWithUser("email: admin@plant7.example, role: owner, password: correct-horse-battery"),
			"projects: [{name: Sensors, token: "+token+", sourceMapToken: "+token+"-maps}]\n"...)
	}
	if _, err := parseBootstrapFile(projects(strings.Repeat("t", 59))); err != nil {
		t.Errorf("tokens of 59 and 64 characters were refused: %v", err)
	}
	_, err = parseBootstrapFile(projects(strings.Repeat("t", 65)))
	for _, want := range []string{"projects[0].token: must be 16 to 64 characters", "projects[0].sourceMapToken: must be 16 to 64 characters"} {
		if err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("missing problem %q in: %v", want, err)
		}
	}
}

func TestParseBootstrapFileAcceptsOnlyCompleteBcryptHashes(t *testing.T) {
	if !services.CheckPassword("operator-password", bootstrapTestHash) {
		t.Fatal("the test hash does not match its password")
	}
	for _, prefix := range []string{"$2a$", "$2b$", "$2y$"} {
		hash := prefix + bootstrapTestHash[4:]
		file, err := parseBootstrapFile(bootstrapWithUser("email: admin@plant7.example, role: owner, passwordHash: '" + hash + "'"))
		if err != nil {
			t.Errorf("%s hash refused: %v", prefix, err)
			continue
		}
		if !services.CheckPassword("operator-password", file.Users[0].PasswordHash) {
			t.Errorf("%s hash was accepted but the password does not sign in with it", prefix)
		}
	}
	for name, hash := range map[string]string{
		"truncated":         bootstrapTestHash[:59],
		"trailing text":     bootstrapTestHash + "x",
		"foreign character": bootstrapTestHash[:30] + "!" + bootstrapTestHash[31:],
		"cost out of range": "$2b$99$" + bootstrapTestHash[7:],
		"yescrypt":          "$y$j9T$F5Jx5fExrKuPp53xLKQ..1$X3DX6M94c7o.9agCG9G317fhZg9SqC.5i5rd.RhAtQ7",
		"htpasswd line":     "admin:" + bootstrapTestHash,
	} {
		_, err := parseBootstrapFile(bootstrapWithUser("email: admin@plant7.example, role: owner, passwordHash: '" + hash + "'"))
		if err == nil || !strings.Contains(err.Error(), "users[0].passwordHash: is not a complete bcrypt hash") {
			t.Errorf("%s: err = %v, want the hash refused", name, err)
		}
	}
}

func TestParseBootstrapFileRejectsASecondDocument(t *testing.T) {
	_, err := parseBootstrapFile(append(bootstrapWithUser("email: admin@plant7.example, role: owner, password: correct-horse-battery"),
		"---\nprojects: [{name: Sensors}]\n"...))
	if err == nil || !strings.Contains(err.Error(), "more than one YAML document") {
		t.Fatalf("err = %v, want the second document refused instead of dropped", err)
	}
}

func TestParseBootstrapFileExplainsAWrongShapeWithoutGoTypesOrValues(t *testing.T) {
	for doc, want := range map[string]string{
		"organization: Plant 7\n":                               "line 1: expected a mapping, found a single value",
		"users: admin@plant7.example\n":                         "line 1: expected a list, found a single value",
		"users:\n  - correct-horse-battery\n":                   "line 2: expected a mapping, found a single value",
		"users: [{password: [correct-horse-battery, other]}]\n": "line 1: expected a single value, found a list",
		"- organization\n":                                      "line 1: expected a mapping, found a list",
	} {
		_, err := parseBootstrapFile([]byte(doc))
		if err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("%q: err = %v, want %q", doc, err, want)
			continue
		}
		for _, leak := range []string{"cmd.", "!!", "Plant", "admin@", "correct"} {
			if strings.Contains(err.Error(), leak) {
				t.Errorf("%q: the error contains %q: %s", doc, leak, err)
			}
		}
	}
}
