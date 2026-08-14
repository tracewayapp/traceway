package oncall

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/tracewayapp/traceway/backend/app/db"
	"github.com/tracewayapp/traceway/backend/app/outbox"
	"github.com/tracewayapp/traceway/backend/app/repositories/transactional"
)

// AutoResolveByDedupKeyTx resolves the unresolved page holding the
// rule-scoped dedup key with no resolving user (a system resolve, e.g. a
// synthetic check recovering), cancelling its queued deliveries. Runs in the
// caller's transaction. Returns whether a page was resolved.
func AutoResolveByDedupKeyTx(tx *sql.Tx, ruleId int, dedupToken string, now time.Time) (bool, error) {
	key := pageDedupKey(fmt.Sprintf("%d", ruleId), dedupToken)
	page, err := transactional.PageRepository.FindUnresolvedByDedupKey(tx, key)
	if err != nil {
		return false, err
	}
	if page == nil {
		return false, nil
	}
	resolved, err := transactional.PageRepository.ResolveSystem(tx, page.Id, now)
	if err != nil || !resolved {
		return resolved, err
	}
	return true, outbox.CancelByKey(tx, outbox.PageCancelKey(page.Id))
}

// AutoResolveByDedupKey is the transaction-owning variant, registered as the
// notifications page resolver in cmd/run.go (that caller holds no
// transaction; nesting one would deadlock the single-connection SQLite main
// DB).
func AutoResolveByDedupKey(ruleId int, dedupToken string) error {
	_, err := db.ExecuteTransaction(func(tx *sql.Tx) (bool, error) {
		return AutoResolveByDedupKeyTx(tx, ruleId, dedupToken, time.Now().UTC())
	})
	return err
}
