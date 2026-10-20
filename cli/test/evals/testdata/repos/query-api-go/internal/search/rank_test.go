package search

import "testing"

func TestRankPrefersTitleMatchesAndFreshness(t *testing.T) {
	hits := []Hit{
		{ID: 1, Title: "Quarterly planning", Snippet: "notes on the budget", DBRank: 0.5, AgeDays: 10},
		{ID: 2, Title: "Budget 2026", Snippet: "the budget for next year", DBRank: 0.5, AgeDays: 10},
		{ID: 3, Title: "Budget 2019", Snippet: "an old budget", DBRank: 0.5, AgeDays: 2400},
	}
	got := Rank("budget", hits)
	if got[0].ID != 2 || got[2].ID != 3 {
		t.Fatalf("order %d %d %d", got[0].ID, got[1].ID, got[2].ID)
	}
	if got[0].Highlight != "the <mark>budget</mark> for next year" {
		t.Fatalf("highlight %q", got[0].Highlight)
	}
}
