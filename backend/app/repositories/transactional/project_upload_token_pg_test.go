//go:build transactional_pg

package transactional

import (
	"database/sql"
	"net/url"
	"os"
	"testing"

	"github.com/google/uuid"
)

// Both transactions observed an empty token before either started its backfill.
// The losing writer must return the winner's token without rotating it.
func TestConcurrentProjectUploadTokenBackfill(t *testing.T) {
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
	id := uuid.New()
	_, err = conn.Exec("INSERT INTO projects (id,name,framework,token,created_at) VALUES ($1,$2,'react',$3,CURRENT_TIMESTAMP)", id, "backfill "+id.String(), uuid.NewString())
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Exec("DELETE FROM projects WHERE id=$1", id)
	var txs []*sql.Tx
	for range 2 {
		tx, err := conn.Begin()
		if err != nil {
			t.Fatal(err)
		}
		defer tx.Rollback()
		var initial sql.NullString
		if err := tx.QueryRow("SELECT source_map_token FROM projects WHERE id=$1", id).Scan(&initial); err != nil || initial.Valid {
			t.Fatalf("initial token should be NULL, err=%v", err)
		}
		txs = append(txs, tx)
	}
	type result struct {
		token string
		added bool
		err   error
	}
	results := make(chan result, 2)
	for _, tx := range txs {
		go func() {
			token, added, err := EnsureSourceMapToken(tx, id)
			if err == nil {
				err = tx.Commit()
			} else {
				tx.Rollback()
			}
			results <- result{token, added, err}
		}()
	}
	first, second := <-results, <-results
	if first.err != nil || second.err != nil || first.token == "" || first.token != second.token || first.added == second.added {
		t.Fatalf("backfills disagree, errors %v and %v, inserted %t and %t", first.err, second.err, first.added, second.added)
	}
}
