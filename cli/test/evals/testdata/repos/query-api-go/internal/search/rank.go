// Package search serves the query, suggest and document routes.
package search

import (
	"html"
	"math"
	"sort"
	"strings"
	"unicode"
)

// Hit is one candidate document from the full-text query.
type Hit struct {
	ID        int64   `json:"id"`
	Title     string  `json:"title"`
	Snippet   string  `json:"snippet"`
	DBRank    float64 `json:"-"`
	AgeDays   float64 `json:"-"`
	Score     float64 `json:"score"`
	Highlight string  `json:"highlight"`
}

// Rank re-scores the database's top candidates: text rank blended with
// title matches and freshness, then the best 20.
func Rank(terms []string, hits []Hit) []Hit {
	for i := range hits {
		h := &hits[i]
		title := strings.ToLower(h.Title)
		boost := 0.0
		for _, t := range terms {
			if strings.Contains(title, t) {
				boost += 0.35
			}
		}
		h.Score = h.DBRank * (1 + boost) * math.Exp(-h.AgeDays/365)
	}
	sort.SliceStable(hits, func(i, j int) bool { return hits[i].Score > hits[j].Score })
	if len(hits) > 20 {
		hits = hits[:20]
	}
	return hits
}

// Terms lowercases and splits a query.
func Terms(q string) []string {
	return strings.FieldsFunc(strings.ToLower(q), func(r rune) bool { return !unicode.IsLetter(r) && !unicode.IsDigit(r) })
}

// Highlight wraps query terms in <mark>.
func Highlight(s string, terms []string) string {
	words := strings.Fields(s)
	for i, w := range words {
		lw := strings.ToLower(strings.Trim(w, ".,;:!?\"'()"))
		for _, t := range terms {
			if lw == t {
				words[i] = "<mark>" + w + "</mark>"
				break
			}
		}
	}
	return strings.Join(words, " ")
}

// RenderHTML renders a document body as HTML paragraphs.
func RenderHTML(body string) string {
	var b strings.Builder
	for _, p := range strings.Split(strings.ReplaceAll(body, "\r\n", "\n"), "\n\n") {
		if p = strings.TrimSpace(p); p != "" {
			b.WriteString("<p>" + strings.ReplaceAll(html.EscapeString(p), "\n", "<br>") + "</p>")
		}
	}
	return b.String()
}
