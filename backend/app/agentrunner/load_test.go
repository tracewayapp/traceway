//go:build !transactional_pg && !telemetry_ch && !telemetry_duckdb

package agentrunner

import (
	"context"
	"database/sql"
	"fmt"
	"sort"
	"sync"
	"testing"
	"time"

	"github.com/tracewayapp/traceway/backend/app/agent"
	"github.com/tracewayapp/traceway/backend/app/db"
	"github.com/tracewayapp/traceway/backend/app/models"
	"github.com/tracewayapp/traceway/backend/app/repositories/transactional"
)

// TestTwentyAttemptsFourWorkers is the launch gate's load check: the queue
// drains in creation order across concurrent in-process workers on the
// single-connection SQLite main database, every attempt finishes, and no
// worker ever sees a lock error.
func TestTwentyAttemptsFourWorkers(t *testing.T) {
	h := setupHarness(t, "")
	h.scriptAgent(`subject=$(printf '%s' "$prompt" | grep -oE '[0-9a-f]{16}' | head -1)
printf 'package main\n\nfunc main() {}\n' > main.go
echo ---REPORT---
echo "STATUS: fixed"
echo "SUBJECT: $subject"
echo "Fixed it."`)
	const attempts, workers = 20, 4

	var ids []string
	for i := 0; i < attempts; i++ {
		hash := fmt.Sprintf("%016x", 0x1000+i)
		attempt := h.start(hash)
		ids = append(ids, attempt.Id.String())
		time.Sleep(2 * time.Millisecond)
	}

	var mu sync.Mutex
	var claimOrder []string
	var wg sync.WaitGroup
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Minute)
	defer cancel()
	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for ctx.Err() == nil {
				claimed, err := h.runner.Claimer.Claim(ctx, 1)
				if err != nil {
					t.Errorf("claim: %v", err)
					return
				}
				if len(claimed) == 0 {
					return
				}
				mu.Lock()
				claimOrder = append(claimOrder, claimed[0].Id.String())
				mu.Unlock()
				h.runner.Run(ctx, claimed[0])
			}
		}()
	}
	wg.Wait()

	if len(claimOrder) != attempts {
		t.Fatalf("claimed %d of %d", len(claimOrder), attempts)
	}
	created := map[string]int{}
	for i, id := range ids {
		created[id] = i
	}
	var positions []int
	for _, id := range claimOrder {
		positions = append(positions, created[id])
	}
	if !sort.IntsAreSorted(positions) {
		t.Fatalf("claims left creation order: %v", positions)
	}

	rows, err := db.ExecuteTransaction(func(tx *sql.Tx) ([]*models.AgentAttempt, error) {
		return transactional.AgentAttemptRepository.FindByProject(tx, h.project.Id, "", 100, 0)
	})
	if err != nil {
		t.Fatal(err)
	}
	finished := 0
	for _, row := range rows {
		if row.Status == models.AttemptAwaitingReview {
			finished++
			continue
		}
		t.Errorf("attempt %d ended %s: %s", row.Number, row.Status, row.Error)
	}
	if finished != attempts {
		t.Fatalf("finished %d of %d", finished, attempts)
	}
	stats, err := agent.HealthSnapshot()
	if err != nil || stats.Queued != 0 || stats.ByStatus[models.AttemptAwaitingReview] != attempts || stats.StartedTotal < attempts {
		t.Fatalf("health = %+v, %v", stats, err)
	}
	if len(h.host.pulls) != attempts {
		t.Fatalf("pull requests = %d", len(h.host.pulls))
	}
}
