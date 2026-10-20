package reserve

import (
	"context"
	"errors"
	"strings"
	"testing"

	"example.com/inventory-api/internal/db"
)

type fakeTx struct {
	stmts      []string
	args       [][]any
	outOfStock map[string]bool
	committed  bool
	rolledBack bool
}

func (t *fakeTx) Exec(_ context.Context, sql string, args ...any) (int64, error) {
	t.stmts = append(t.stmts, sql)
	t.args = append(t.args, args)
	if strings.HasPrefix(sql, "UPDATE inventory") && len(args) > 1 {
		if sku, ok := args[1].(string); ok && t.outOfStock[sku] {
			return 0, nil
		}
	}
	return 1, nil
}
func (t *fakeTx) Commit(context.Context) error   { t.committed = true; return nil }
func (t *fakeTx) Rollback(context.Context) error { t.rolledBack = true; return nil }

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
