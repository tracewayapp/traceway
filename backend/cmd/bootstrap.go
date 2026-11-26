package cmd

import (
	"bytes"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"net/mail"
	"os"
	"regexp"
	"strings"
	"unicode/utf8"

	"github.com/tracewayapp/traceway/backend/v2/app/config"
	"github.com/tracewayapp/traceway/backend/v2/app/controllers"
	"github.com/tracewayapp/traceway/backend/v2/app/db"
	"github.com/tracewayapp/traceway/backend/v2/app/models"
	"github.com/tracewayapp/traceway/backend/v2/app/oncall"
	"github.com/tracewayapp/traceway/backend/v2/app/repositories/transactional"
	"github.com/tracewayapp/traceway/backend/v2/app/services"

	"go.yaml.in/yaml/v3"
	"golang.org/x/crypto/bcrypt"
)

const bootstrapAdvisoryLockId = 824737010 // metric rollup repair: 824737009, backfills: 824737001-824737008

const maxBootstrapFileBytes = 1 << 20

// Tokens travel in an Authorization header and before the @ of a connection
// string, so the alphabet stays URL- and header-safe.
var bootstrapTokenPattern = regexp.MustCompile(`^[A-Za-z0-9._~-]{16,128}$`)

var yamlTypeSuffix = regexp.MustCompile(` in type \S+`)

type bootstrapFile struct {
	Organization bootstrapOrganization `yaml:"organization"`
	Users        []bootstrapUser       `yaml:"users"`
	Projects     []bootstrapProject    `yaml:"projects"`
}

type bootstrapOrganization struct {
	Name     string `yaml:"name"`
	Timezone string `yaml:"timezone"`
}

type bootstrapUser struct {
	Email        string `yaml:"email"`
	Name         string `yaml:"name"`
	Role         string `yaml:"role"`
	Password     string `yaml:"password"`
	PasswordHash string `yaml:"passwordHash"`
}

type bootstrapProject struct {
	Name           string `yaml:"name"`
	Framework      string `yaml:"framework"`
	Token          string `yaml:"token"`
	SourceMapToken string `yaml:"sourceMapToken"`
}

type bootstrapFileError struct {
	path string
	err  error
}

func (e *bootstrapFileError) Error() string {
	return fmt.Sprintf("cannot use bootstrap file %s:\n  %s", e.path, strings.ReplaceAll(e.err.Error(), "\n", "\n  "))
}

// runBootstrap applies BOOTSTRAP_FILE when the database is still empty. A
// broken file stops startup: devices configured from the same file would
// otherwise send to a project that was never created.
func runBootstrap(path string) {
	if path == "" {
		return
	}
	_, err := applyBootstrapFile(path)
	var fileErr *bootstrapFileError
	if errors.As(err, &fileErr) {
		exitOnMisconfiguration("FATAL: "+fileErr.Error(), "Nothing was created. Fix the file and start Traceway again.")
	}
	if err != nil {
		panic(fmt.Errorf("applying bootstrap file %s failed: %w", path, err))
	}
}

func applyBootstrapFile(path string) (bool, error) {
	fresh, err := db.ExecuteTransaction(isFreshInstance)
	if err != nil {
		return false, err
	}
	if !fresh {
		config.Logf("Bootstrap: %s was not applied, the database already has an organization or a user", path)
		return false, nil
	}

	file, err := loadBootstrapFile(path)
	if err != nil {
		return false, &bootstrapFileError{path: path, err: err}
	}
	hashes, err := file.passwordHashes()
	if err != nil {
		return false, err
	}

	applied, err := db.ExecuteTransaction(func(tx *sql.Tx) (bool, error) {
		if !db.IsSQLite() {
			if _, err := tx.Exec(fmt.Sprintf("SELECT pg_advisory_xact_lock(%d)", bootstrapAdvisoryLockId)); err != nil {
				return false, err
			}
		}
		// Replicas starting together all saw an empty database above; the lock lets one of them apply the file.
		fresh, err := isFreshInstance(tx)
		if err != nil || !fresh {
			return false, err
		}
		return true, file.apply(tx, hashes)
	})
	if err != nil || !applied {
		return false, err
	}
	config.Logf("Bootstrap: created organization %q with %d users and %d projects from %s", file.Organization.Name, len(file.Users), len(file.Projects), path)
	return true, nil
}

func isFreshInstance(tx *sql.Tx) (bool, error) {
	hasOrganizations, err := transactional.OrganizationRepository.HasOrganizations(tx)
	if err != nil || hasOrganizations {
		return false, err
	}
	hasUsers, err := transactional.UserRepository.HasUsers(tx)
	return !hasUsers, err
}

func loadBootstrapFile(path string) (*bootstrapFile, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	data, err := io.ReadAll(io.LimitReader(f, maxBootstrapFileBytes+1))
	if err != nil {
		return nil, err
	}
	if len(data) > maxBootstrapFileBytes {
		return nil, errors.New("the file is larger than 1 MB")
	}
	return parseBootstrapFile(data)
}

func parseBootstrapFile(data []byte) (*bootstrapFile, error) {
	decoder := yaml.NewDecoder(bytes.NewReader(data))
	decoder.KnownFields(true)
	var file bootstrapFile
	if err := decoder.Decode(&file); err != nil {
		if errors.Is(err, io.EOF) {
			return nil, errors.New("the file is empty")
		}
		return nil, errors.New(yamlTypeSuffix.ReplaceAllString(err.Error(), ""))
	}
	if err := file.validate(); err != nil {
		return nil, err
	}
	return &file, nil
}

func (f *bootstrapFile) validate() error {
	var problems []string
	add := func(format string, args ...any) {
		problems = append(problems, fmt.Sprintf(format, args...))
	}

	f.Organization.Name = strings.TrimSpace(f.Organization.Name)
	if n := utf8.RuneCountInString(f.Organization.Name); n < 1 || n > 100 {
		add("organization.name: must be between 1 and 100 characters")
	}
	f.Organization.Timezone = strings.TrimSpace(f.Organization.Timezone)
	if f.Organization.Timezone == "" {
		f.Organization.Timezone = "UTC"
	}
	if _, err := oncall.LoadTimezone(f.Organization.Timezone); err != nil {
		add("organization.timezone: %s", err)
	}

	if len(f.Users) == 0 {
		add("users: at least one user is required")
	}
	emails := map[string]int{}
	owners := 0
	for i := range f.Users {
		u := &f.Users[i]
		at := fmt.Sprintf("users[%d]", i)

		u.Email = strings.TrimSpace(u.Email)
		if address, err := mail.ParseAddress(u.Email); err != nil || address.Address != u.Email {
			add("%s.email: %q is not an email address", at, u.Email)
		} else if first, seen := emails[strings.ToLower(u.Email)]; seen {
			add("%s.email: %s is already used by users[%d]", at, u.Email, first)
		} else {
			emails[strings.ToLower(u.Email)] = i
		}

		u.Name = strings.TrimSpace(u.Name)
		if u.Name == "" {
			u.Name, _, _ = strings.Cut(u.Email, "@")
		}

		switch u.Role {
		case "owner":
			owners++
		case "admin", "user", "readonly":
		default:
			add("%s.role: must be owner, admin, user or readonly", at)
		}

		switch {
		case u.Password != "" && u.PasswordHash != "":
			add("%s: set password or passwordHash, not both", at)
		case u.PasswordHash != "":
			if _, err := bcrypt.Cost([]byte(u.PasswordHash)); err != nil {
				add("%s.passwordHash: is not a bcrypt hash", at)
			}
		case u.Password == "":
			add("%s: password or passwordHash is required", at)
		case len(u.Password) < 8:
			add("%s.password: must be at least 8 characters", at)
		case len(u.Password) > 72:
			add("%s.password: must be at most 72 bytes", at)
		}
	}
	if len(f.Users) > 0 && owners == 0 {
		add("users: at least one user needs the role owner")
	}

	names := map[string]int{}
	tokens := map[string]string{}
	for i := range f.Projects {
		p := &f.Projects[i]
		at := fmt.Sprintf("projects[%d]", i)

		p.Name = strings.TrimSpace(p.Name)
		if p.Framework == "" {
			p.Framework = "opentelemetry"
		}
		if msg := controllers.ValidateNewProject(p.Name, p.Framework); msg != "" {
			add("%s: %s", at, msg)
		} else if first, seen := names[strings.ToLower(p.Name)]; seen {
			add("%s.name: %q is already used by projects[%d]", at, p.Name, first)
		} else {
			names[strings.ToLower(p.Name)] = i
		}

		for _, token := range []struct{ field, value string }{{"token", p.Token}, {"sourceMapToken", p.SourceMapToken}} {
			if token.value == "" {
				continue
			}
			if !bootstrapTokenPattern.MatchString(token.value) {
				add("%s.%s: must be 16 to 128 characters of letters, digits, '.', '_', '~' and '-'", at, token.field)
			} else if first, seen := tokens[token.value]; seen {
				add("%s.%s: is the same as %s", at, token.field, first)
			} else {
				tokens[token.value] = at + "." + token.field
			}
		}
	}

	if len(problems) > 0 {
		return errors.New(strings.Join(problems, "\n"))
	}
	return nil
}

// bcrypt is slow on purpose, so the hashes are computed before the
// transaction instead of holding the main database for each of them.
func (f *bootstrapFile) passwordHashes() ([]string, error) {
	hashes := make([]string, len(f.Users))
	for i, u := range f.Users {
		if u.PasswordHash != "" {
			hashes[i] = u.PasswordHash
			continue
		}
		hash, err := services.HashPassword(u.Password)
		if err != nil {
			return nil, err
		}
		hashes[i] = hash
	}
	return hashes, nil
}

func (f *bootstrapFile) apply(tx *sql.Tx, hashes []string) error {
	org, err := transactional.OrganizationRepository.Create(tx, f.Organization.Name, f.Organization.Timezone)
	if err != nil {
		return err
	}

	var owner *models.User
	for i, u := range f.Users {
		user, err := transactional.UserRepository.Create(tx, u.Email, u.Name, hashes[i])
		if err != nil {
			return err
		}
		if _, err := transactional.OrganizationRepository.AddUser(tx, org.Id, user.Id, u.Role); err != nil {
			return err
		}
		if owner == nil && u.Role == "owner" {
			owner = user
		}
	}

	for _, hook := range controllers.PostRegistrationHooks {
		if err := hook(tx, org, owner); err != nil {
			return err
		}
	}

	for _, p := range f.Projects {
		project, err := transactional.ProjectRepository.CreateWithOrganizationAndTokens(tx, p.Name, p.Framework, org.Id, p.Token, p.SourceMapToken)
		if err != nil {
			return err
		}
		if err := controllers.PopulateDashboardsOnCreate(tx, project, &owner.Id); err != nil {
			return err
		}
		if p.Token == "" {
			config.LogfLocal("Bootstrap: project %q was given the token %s", project.Name, project.Token)
		}
	}
	return nil
}
