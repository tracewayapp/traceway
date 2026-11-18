//go:build !transactional_pg

package transactional

import (
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/tracewayapp/traceway/backend/v2/app/clientmetrics"
	"github.com/tracewayapp/traceway/backend/v2/app/db"
)

func TestClientMetricGuardEntries(t *testing.T) {
	setupTestDB(t)
	schema, err := os.ReadFile("../../migrations/sqlite/0089_create_client_metric_guard_entries.up.sql")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.DB.Exec(string(schema)); err != nil {
		t.Fatal(err)
	}
	project, other := uuid.New(), uuid.New()
	const today = 20000
	start := time.Now().UTC()
	entries := []clientmetrics.GuardEntry{
		{ProjectId: project, Day: today, Scope: "dim:os.name", Value: "Android"},
		{ProjectId: project, Day: today - 1, Scope: "dim:os.name", Value: "iOS"},
		{ProjectId: project, Day: 0, Scope: "metric", Value: "checkout.step"},
		{ProjectId: other, Day: today, Scope: "dim:url.path", Value: "/orders/:id"},
	}
	if err := ClientMetricGuardRepository.Insert(db.DB, entries, start); err != nil {
		t.Fatal(err)
	}
	// Another instance admitting the same value is not an error and not a second row.
	if err := ClientMetricGuardRepository.Insert(db.DB, entries[:1], start.Add(time.Minute)); err != nil {
		t.Fatal(err)
	}

	all, err := ClientMetricGuardRepository.ListSince(db.DB, today, time.Time{})
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != 3 {
		t.Fatalf("today's entries and registrations: %+v", all)
	}
	seen := map[clientmetrics.GuardEntry]bool{}
	for _, entry := range all {
		seen[entry] = true
	}
	for _, want := range []clientmetrics.GuardEntry{entries[0], entries[2], entries[3]} {
		if !seen[want] {
			t.Errorf("missing %+v", want)
		}
	}

	later := []clientmetrics.GuardEntry{{ProjectId: project, Day: today, Scope: "dim:os.name", Value: "Windows"}}
	if err := ClientMetricGuardRepository.Insert(db.DB, later, start.Add(10*time.Minute)); err != nil {
		t.Fatal(err)
	}
	fresh, err := ClientMetricGuardRepository.ListSince(db.DB, today, start.Add(5*time.Minute))
	if err != nil || len(fresh) != 1 || fresh[0] != later[0] {
		t.Fatalf("entries since the last read: %+v (err %v)", fresh, err)
	}

	if err := ClientMetricGuardRepository.PruneBefore(db.DB, today); err != nil {
		t.Fatal(err)
	}
	var days, registrations int
	if err := db.DB.QueryRow("SELECT COUNT(*) FROM client_metric_guard_entries WHERE day > 0 AND day < ?", today).Scan(&days); err != nil {
		t.Fatal(err)
	}
	if err := db.DB.QueryRow("SELECT COUNT(*) FROM client_metric_guard_entries WHERE day = 0").Scan(&registrations); err != nil {
		t.Fatal(err)
	}
	if days != 0 || registrations != 1 {
		t.Errorf("after pruning: %d old daily entries, %d registrations", days, registrations)
	}
}
