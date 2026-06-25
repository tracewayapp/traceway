package services

import (
	"testing"

	"github.com/tracewayapp/traceway/backend/app/models"
)

func statByName(stats []FunctionStat, name string) (FunctionStat, bool) {
	for _, s := range stats {
		if s.Name == name {
			return s, true
		}
	}
	return FunctionStat{}, false
}

func TestTopFunctions_FlatAndCum(t *testing.T) {
	stacks := []models.ProfileStackValue{
		{Stack: []string{"main", "serve", "checkout", "query"}, Value: 300},
		{Stack: []string{"main", "serve", "checkout"}, Value: 50},
		{Stack: []string{"main", "serve", "cart", "get"}, Value: 100},
	}

	stats := TopFunctions(stacks, 0)

	main, ok := statByName(stats, "main")
	if !ok {
		t.Fatal("main missing")
	}
	if main.Flat != 0 {
		t.Errorf("main flat = %d, want 0 (never a leaf)", main.Flat)
	}
	if main.Cum != 450 {
		t.Errorf("main cum = %d, want 450 (every sample passes through main)", main.Cum)
	}

	query, _ := statByName(stats, "query")
	if query.Flat != 300 || query.Cum != 300 {
		t.Errorf("query = flat %d cum %d, want 300/300", query.Flat, query.Cum)
	}

	checkout, _ := statByName(stats, "checkout")
	if checkout.Flat != 50 {
		t.Errorf("checkout flat = %d, want 50 (leaf only on the 50 sample)", checkout.Flat)
	}
	if checkout.Cum != 350 {
		t.Errorf("checkout cum = %d, want 350 (300 + 50)", checkout.Cum)
	}

	if want := float64(450) / float64(450) * 100; main.CumPct != want {
		t.Errorf("main cumPct = %f, want %f", main.CumPct, want)
	}
}

func TestTopFunctions_RecursionDedupesWithinStack(t *testing.T) {
	stacks := []models.ProfileStackValue{
		{Stack: []string{"main", "recurse", "recurse", "recurse", "leaf"}, Value: 100},
	}
	stats := TopFunctions(stacks, 0)

	recurse, _ := statByName(stats, "recurse")
	if recurse.Cum != 100 {
		t.Errorf("recurse cum = %d, want 100 (counted once per stack despite 3 frames)", recurse.Cum)
	}
	if recurse.Flat != 0 {
		t.Errorf("recurse flat = %d, want 0 (leaf is 'leaf')", recurse.Flat)
	}
}

func TestTopFunctions_SortedByFlatThenLimited(t *testing.T) {
	stacks := []models.ProfileStackValue{
		{Stack: []string{"a"}, Value: 10},
		{Stack: []string{"b"}, Value: 30},
		{Stack: []string{"c"}, Value: 20},
	}
	stats := TopFunctions(stacks, 2)
	if len(stats) != 2 {
		t.Fatalf("got %d stats, want 2 (limited)", len(stats))
	}
	if stats[0].Name != "b" || stats[1].Name != "c" {
		t.Errorf("order = [%s %s], want [b c] (flat desc)", stats[0].Name, stats[1].Name)
	}
}
