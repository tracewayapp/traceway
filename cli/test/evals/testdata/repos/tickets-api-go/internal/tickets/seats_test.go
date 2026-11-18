package tickets

import "testing"

func TestSeatMapGroupsBySection(t *testing.T) {
	got := SeatMap([]SeatBlock{
		{Section: "A", Row: "1", SeatCount: 10, PriceCents: 5000, Available: 2},
		{Section: "A", Row: "2", SeatCount: 10, PriceCents: 4500, Available: 0},
		{Section: "B", Row: "1", SeatCount: 12, PriceCents: 3000, Available: 12},
	})
	if len(got) != 2 || got[0].Available != 2 || got[1].Available != 12 {
		t.Fatalf("unexpected sections: %+v", got)
	}
	if got[0].MinPriceCents != 5000 {
		t.Fatalf("section A min price %d, want 5000 (row 2 is sold out)", got[0].MinPriceCents)
	}
}
