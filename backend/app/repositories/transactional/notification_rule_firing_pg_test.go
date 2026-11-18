//go:build transactional_pg

package transactional

import (
	"database/sql"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/tracewayapp/lit/v2"
	"github.com/tracewayapp/traceway/backend/v2/app/db"
	"github.com/tracewayapp/traceway/backend/v2/app/models"
)

func TestMetricSeriesLongTokenPostgres(t *testing.T) {
	host := os.Getenv("TEST_POSTGRES_HOST")
	if host == "" {
		t.Skip("TEST_POSTGRES_HOST not set")
	}
	params := url.Values{"host": {host}, "sslmode": {"disable"}}
	if port := os.Getenv("TEST_POSTGRES_PORT"); port != "" {
		params.Set("port", port)
	}
	dsn := url.URL{Scheme: "postgres", User: url.UserPassword(os.Getenv("TEST_POSTGRES_USERNAME"), os.Getenv("TEST_POSTGRES_PASSWORD")), Path: "/" + os.Getenv("TEST_POSTGRES_DATABASE"), RawQuery: params.Encode()}
	conn, err := sql.Open("postgres", dsn.String())
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	db.Driver = lit.PostgreSQL
	models.Init(db.Driver)
	tx, err := conn.Begin()
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()

	project, err := ProjectRepository.Create(tx, "grouped metric test", "gin")
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	channelID, err := NotificationChannelRepository.Create(tx, &models.NotificationChannel{ProjectId: project.Id, Name: "Ops", ChannelType: "slack", Config: []byte("{}"), Enabled: true, CreatedAt: now, UpdatedAt: now})
	if err != nil {
		t.Fatal(err)
	}
	ruleID, err := NotificationRuleRepository.Create(tx, &models.NotificationRule{ProjectId: project.Id, ChannelId: channelID, Name: "Latency", RuleType: "metric_threshold", Config: []byte("{}"), Enabled: true, CreatedAt: now, UpdatedAt: now})
	if err != nil {
		t.Fatal(err)
	}
	// Incompressible tag text can exceed PostgreSQL's B-tree entry size.
	var label strings.Builder
	for range 256 {
		label.WriteString(uuid.NewString())
	}
	token := "latency{site=" + label.String() + "}"
	repo := NotificationRuleFiringRepository
	for _, want := range []bool{true, false} {
		if changed, err := repo.Insert(tx, ruleID, token, now); err != nil || changed != want {
			t.Fatalf("insert changed=%v, want %v, err=%v", changed, want, err)
		}
	}
	if err := repo.SetSilentSince(tx, ruleID, token, &now); err != nil {
		t.Fatal(err)
	}
	rows, err := repo.FindByRule(tx, ruleID)
	if err != nil || len(rows) != 1 || rows[0].Token != token || rows[0].SilentSince == nil {
		t.Fatalf("expected one open, silent series, err=%v", err)
	}
	for _, want := range []bool{true, false} {
		if changed, err := repo.Delete(tx, ruleID, token); err != nil || changed != want {
			t.Fatalf("delete changed=%v, want %v, err=%v", changed, want, err)
		}
	}
}
