package reserve

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"example.com/inventory-api/internal/db"
)

type fakeTx struct {
	stmts      []string
	args       [][]any
	outOfStock map[string]bool
	failSKU    map[string]error
	returning  []any
	committed  bool
	rolledBack bool
}

func (t *fakeTx) Exec(_ context.Context, sql string, args ...any) (int64, error) {
	t.stmts = append(t.stmts, sql)
	t.args = append(t.args, args)
	if strings.HasPrefix(sql, "UPDATE inventory") && len(args) > 1 {
		if sku, ok := args[1].(string); ok {
			if err := t.failSKU[sku]; err != nil {
				return 0, err
			}
			if t.outOfStock[sku] {
				return 0, nil
			}
		}
	}
	return 1, nil
}

func (t *fakeTx) QueryRow(_ context.Context, sql string, args ...any) db.Row {
	t.stmts = append(t.stmts, sql)
	t.args = append(t.args, args)
	return fakeRow(t.returning)
}

func (t *fakeTx) Commit(context.Context) error   { t.committed = true; return nil }
func (t *fakeTx) Rollback(context.Context) error { t.rolledBack = true; return nil }

type fakeRow []any

func (r fakeRow) Scan(dest ...any) error {
	if len(r) == 0 {
		return db.ErrNoRows
	}
	*dest[0].(*string), *dest[1].(*int64), *dest[2].(*int) = r[0].(string), r[1].(int64), r[2].(int)
	return nil
}

type fakePool struct{ tx *fakeTx }

func (p *fakePool) Begin(context.Context) (db.Tx, error) { return p.tx, nil }

func TestReserveHoldsEveryLine(t *testing.T) {
	tx := &fakeTx{}
	err := Reserve(context.Background(), &fakePool{tx}, Request{CartID: "c_1", Lines: []Line{
		{SKU: "SKU-1", WarehouseID: 3, Qty: 1}, {SKU: "SKU-2", WarehouseID: 3, Qty: 2},
	}})
	if err != nil {
		t.Fatal(err)
	}
	if !tx.committed {
		t.Fatal("not committed")
	}
	held := map[string]bool{}
	for i, s := range tx.stmts {
		if strings.HasPrefix(s, "UPDATE inventory") {
			for _, a := range tx.args[i] {
				if v, ok := a.(string); ok {
					held[v] = true
				}
			}
		}
	}
	if !held["SKU-1"] || !held["SKU-2"] {
		t.Fatalf("lines not held: %v", tx.stmts)
	}
}

func TestReserveRollsBackWhenOutOfStock(t *testing.T) {
	tx := &fakeTx{outOfStock: map[string]bool{"SKU-2": true}}
	err := Reserve(context.Background(), &fakePool{tx}, Request{CartID: "c_1", Lines: []Line{
		{SKU: "SKU-1", WarehouseID: 3, Qty: 1}, {SKU: "SKU-2", WarehouseID: 3, Qty: 2},
	}})
	if !errors.Is(err, ErrOutOfStock) {
		t.Fatalf("err = %v", err)
	}
	if tx.committed || !tx.rolledBack {
		t.Fatal("expected a rollback")
	}
}

func TestReserveNamesTheLineThatFailed(t *testing.T) {
	busy := &db.PgError{Code: "55P03", Message: "canceling statement due to lock timeout"}
	tx := &fakeTx{failSKU: map[string]error{"SKU-2": busy}}
	err := Reserve(context.Background(), &fakePool{tx}, Request{CartID: "c_1", Lines: []Line{
		{SKU: "SKU-1", WarehouseID: 3, Qty: 1}, {SKU: "SKU-2", WarehouseID: 3, Qty: 2},
	}})
	var le *LineError
	if !errors.As(err, &le) || le.SKU != "SKU-2" {
		t.Fatalf("err = %v", err)
	}
	if db.SQLState(err) != "55P03" || err.Error() != busy.Error() {
		t.Fatalf("err = %v", err)
	}
	if !tx.rolledBack {
		t.Fatal("expected a rollback")
	}
}

func TestReleaseGivesTheStockBack(t *testing.T) {
	tx := &fakeTx{returning: []any{"SKU-7", int64(3), 2}}
	h := &Handler{Pool: &fakePool{tx}}
	req := httptest.NewRequest(http.MethodPost, "/api/reservations/41/release", nil)
	req.SetPathValue("id", "41")
	rec := httptest.NewRecorder()
	h.Release(rec, req)
	if rec.Code != http.StatusNoContent || !tx.committed {
		t.Fatalf("status %d, committed %v", rec.Code, tx.committed)
	}
	last := tx.args[len(tx.args)-1]
	if len(last) != 3 || last[0] != 2 || last[1] != "SKU-7" || last[2] != int64(3) {
		t.Fatalf("update args = %v", last)
	}
}
