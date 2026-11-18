package seed

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/tracewayapp/traceway/cli/test/evals/internal/env"
)

// Group is an exception group read back from the API.
type Group struct {
	Hash       string    `json:"exceptionHash"`
	StackTrace string    `json:"stackTrace"`
	FirstSeen  time.Time `json:"firstSeen"`
	LastSeen   time.Time `json:"lastSeen"`
	Count      int       `json:"count"`
}

// Groups lists the project's exception groups over the last `within`.
func (p *Project) Groups(within time.Duration, includeArchived bool) ([]Group, error) {
	if p.s.Sink.Dry() {
		return nil, nil
	}
	res, err := p.s.Sink.Admin(http.MethodPost, "/api/exception-stack-traces", env.ProjectQuery(p.ID), map[string]any{
		"fromDate": p.s.Now.Add(-within).Format(time.RFC3339), "toDate": p.s.Now.Add(time.Hour).Format(time.RFC3339),
		"orderBy": "count", "pagination": map[string]int{"page": 1, "pageSize": 100}, "includeArchived": includeArchived,
	})
	if err != nil {
		return nil, err
	}
	if err := res.Err("list exceptions"); err != nil {
		return nil, err
	}
	var out struct {
		Data []Group `json:"data"`
	}
	if err := json.Unmarshal(res.Body, &out); err != nil {
		return nil, err
	}
	return out.Data, nil
}

// GroupMatching returns the hash of the group whose stack contains needle.
func (p *Project) GroupMatching(within time.Duration, needle string) (string, error) {
	if p.s.Sink.Dry() {
		return fmt.Sprintf("%016x", len(needle)), nil
	}
	groups, err := p.Groups(within, true)
	if err != nil {
		return "", err
	}
	for _, g := range groups {
		if containsFold(g.StackTrace, needle) {
			return g.Hash, nil
		}
	}
	return "", fmt.Errorf("no exception group in %s contains %q", p.Name, needle)
}

func containsFold(s, sub string) bool {
	return len(sub) == 0 || indexFold(s, sub) >= 0
}

func indexFold(s, sub string) int {
	n := len(sub)
	for i := 0; i+n <= len(s); i++ {
		match := true
		for j := 0; j < n; j++ {
			a, b := s[i+j], sub[j]
			if 'A' <= a && a <= 'Z' {
				a += 'a' - 'A'
			}
			if 'A' <= b && b <= 'Z' {
				b += 'a' - 'A'
			}
			if a != b {
				match = false
				break
			}
		}
		if match {
			return i
		}
	}
	return -1
}
