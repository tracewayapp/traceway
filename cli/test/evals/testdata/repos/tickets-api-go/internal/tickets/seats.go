// Package tickets serves events, seat maps and orders.
package tickets

import "time"

// Event is one event with its seat map.
type Event struct {
	ID          int64            `json:"id"`
	Title       string           `json:"title"`
	Description string           `json:"description"`
	VenueID     int64            `json:"venueId"`
	StartsAt    time.Time        `json:"startsAt"`
	Sections    []SectionSummary `json:"sections"`
}

// SeatBlock is a run of seats in one row of a section.
type SeatBlock struct {
	Section    string `json:"section"`
	Row        string `json:"row"`
	SeatCount  int    `json:"seatCount"`
	PriceCents int64  `json:"priceCents"`
	Available  int    `json:"available"`
}

// SectionSummary is one section of the seat map.
type SectionSummary struct {
	Section       string      `json:"section"`
	Available     int         `json:"available"`
	MinPriceCents int64       `json:"minPriceCents"`
	Rows          []SeatBlock `json:"rows"`
}

// SeatMap groups an event's seat blocks into sections, keeping the order the
// blocks arrive in.
func SeatMap(blocks []SeatBlock) []SectionSummary {
	var out []SectionSummary
	index := map[string]int{}
	for _, b := range blocks {
		i, ok := index[b.Section]
		if !ok {
			i = len(out)
			index[b.Section] = i
			out = append(out, SectionSummary{Section: b.Section, MinPriceCents: b.PriceCents})
		}
		s := &out[i]
		s.Available += b.Available
		if b.Available > 0 && (b.PriceCents < s.MinPriceCents || s.MinPriceCents == 0) {
			s.MinPriceCents = b.PriceCents
		}
		s.Rows = append(s.Rows, b)
	}
	return out
}
