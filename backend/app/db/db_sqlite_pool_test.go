//go:build !transactional_pg || (!telemetry_ch && !telemetry_duckdb)

package db

import (
	"context"
	"sync"
	"testing"
)

func TestOpenSQLite_MemoryTelemetryTableVisibleUnderConcurrency(t *testing.T) {
	d, err := openSQLite(":memory:", true)
	if err != nil {
		t.Fatalf("openSQLite: %v", err)
	}
	defer d.Close()

	if _, err := d.Exec("CREATE TABLE probe (id INTEGER)"); err != nil {
		t.Fatalf("create table: %v", err)
	}

	const goroutines = 32
	const iterations = 50
	ctx := context.Background()

	var wg sync.WaitGroup
	errCh := make(chan error, goroutines)
	for g := 0; g < goroutines; g++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < iterations; i++ {
				if _, err := d.ExecContext(ctx, "INSERT INTO probe (id) VALUES (1)"); err != nil {
					errCh <- err
					return
				}
				var n int
				if err := d.QueryRowContext(ctx, "SELECT count(*) FROM probe").Scan(&n); err != nil {
					errCh <- err
					return
				}
			}
		}()
	}
	wg.Wait()
	close(errCh)

	for err := range errCh {
		t.Fatalf("concurrent op hit a pooled connection without the table (separate in-memory DB): %v", err)
	}
}
