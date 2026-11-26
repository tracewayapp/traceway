//go:build transactional_pg

package cmd

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"testing"

	"github.com/google/uuid"
	"github.com/tracewayapp/lit/v2"
	"github.com/tracewayapp/traceway/backend/v2/app/config"
	"github.com/tracewayapp/traceway/backend/v2/app/db"
	"github.com/tracewayapp/traceway/backend/v2/app/models"
)

func setupBootstrapPostgres(t *testing.T) {
	t.Helper()
	password := os.Getenv("TEST_POSTGRES_PASSWORD")
	if password == "" {
		t.Skip("TEST_POSTGRES_PASSWORD is not set")
	}
	host := os.Getenv("TEST_POSTGRES_HOST")
	if host == "" {
		host = "localhost"
	}
	base := fmt.Sprintf("host=%s user=%s password=%s dbname=%s sslmode=disable", host, os.Getenv("TEST_POSTGRES_USERNAME"), password, os.Getenv("TEST_POSTGRES_DATABASE"))
	if port := os.Getenv("TEST_POSTGRES_PORT"); port != "" {
		base += " port=" + port
	}
	admin, err := sql.Open("postgres", base)
	if err != nil {
		t.Fatal(err)
	}
	schema := "bootstrap_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	if _, err := admin.Exec("CREATE SCHEMA " + schema); err != nil {
		t.Fatal(err)
	}
	conn, err := sql.Open("postgres", base+" search_path="+schema)
	if err != nil {
		t.Fatal(err)
	}
	previousDB, previousDriver, previousConfig := db.DB, db.Driver, config.Config
	t.Cleanup(func() {
		conn.Close()
		_, _ = admin.Exec("DROP SCHEMA " + schema + " CASCADE")
		admin.Close()
		db.DB, db.Driver, config.Config = previousDB, previousDriver, previousConfig
	})

	files, err := filepath.Glob("../app/migrations/pg/*.up.sql")
	if err != nil || len(files) == 0 {
		t.Fatalf("no Postgres migrations found: %v", err)
	}
	sort.Strings(files)
	for _, file := range files {
		migration, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := conn.Exec(string(migration)); err != nil {
			t.Fatalf("%s: %v", filepath.Base(file), err)
		}
	}
	db.DB, db.Driver = conn, lit.PostgreSQL
	config.Init(&config.Cfg{})
	models.Init(db.Driver)
}

func TestBootstrapFileIsAppliedOnceByReplicasStartingTogether(t *testing.T) {
	setupBootstrapPostgres(t)
	longest := strings.Repeat("t", 64)
	path := writeBootstrapFile(t, `
organization: {name: Plant 7, timezone: Europe/Zurich}
users:
  - {email: admin@plant7.example, role: owner, password: correct-horse-battery}
  - {email: viewer@plant7.example, role: readonly, password: correct-horse-battery}
projects:
  - {name: Line Controllers, token: `+longest+`}
  - {name: HMI Panels, framework: react, token: plant7-hmi-panels-ingest, sourceMapToken: `+strings.Repeat("s", 64)+`}
`)

	const replicas = 4
	applied := make([]bool, replicas)
	errs := make([]error, replicas)
	var wg sync.WaitGroup
	for i := range replicas {
		wg.Add(1)
		go func() {
			defer wg.Done()
			applied[i], errs[i] = applyBootstrapFile(path)
		}()
	}
	wg.Wait()

	winners := 0
	for i := range replicas {
		if errs[i] != nil {
			t.Fatalf("replica %d: %v", i, errs[i])
		}
		if applied[i] {
			winners++
		}
	}
	if winners != 1 {
		t.Fatalf("%d replicas applied the file, want exactly 1", winners)
	}

	for table, want := range map[string]int{"organizations": 1, "users": 2, "organization_users": 2, "projects": 2} {
		var count int
		if err := db.DB.QueryRow("SELECT COUNT(*) FROM " + table).Scan(&count); err != nil {
			t.Fatal(err)
		}
		if count != want {
			t.Errorf("%d rows in %s, want %d", count, table, want)
		}
	}
	var token string
	if err := db.DB.QueryRow("SELECT token FROM projects WHERE name = 'Line Controllers'").Scan(&token); err != nil {
		t.Fatal(err)
	}
	if token != longest {
		t.Errorf("the longest allowed token was stored as %q", token)
	}
}
