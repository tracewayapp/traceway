package feed

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"testing"
	"time"
)

type fakeStore struct{ cards []Card }

func (f fakeStore) Latest(context.Context, []string, int) ([]Card, error) { return f.cards, nil }

func (f fakeStore) Card(_ context.Context, id int64) (*Card, error) {
	for _, c := range f.cards {
		if c.ID == id {
			return &c, nil
		}
	}
	return nil, nil
}

func (f fakeStore) Profile(context.Context, int64) (*Profile, error) { return nil, nil }

func TestFeedRanksAnnouncementsFirst(t *testing.T) {
	now := time.Now()
	h := &Handler{Store: fakeStore{cards: []Card{
		{ID: 1, Kind: "post", Title: "a", PublishedAt: now},
		{ID: 2, Kind: "announcement", Title: "b", PublishedAt: now.Add(-time.Hour)},
		{ID: 3, Kind: "post", Title: "c", PublishedAt: now.Add(-time.Minute)},
	}}}
	rec := httptest.NewRecorder()
	h.Feed(rec, httptest.NewRequest("GET", "/api/feed", nil))
	var out struct {
		Cards []struct {
			ID int64 `json:"id"`
		} `json:"cards"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	if len(out.Cards) != 3 || out.Cards[0].ID != 2 || out.Cards[1].ID != 1 || out.Cards[2].ID != 3 {
		t.Fatalf("order %+v", out.Cards)
	}
}
