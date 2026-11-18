package seed

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/tracewayapp/traceway/cli/test/evals/internal/env"
)

// Verify reads the seeded data back through the dashboard API until every
// declared endpoint is listed, so a run never starts on half-ingested data.
// After timeout the run is an infra_error.
func (s *Scenario) Verify(timeout time.Duration) error {
	if s.Sink.Dry() {
		return nil
	}
	type want struct {
		p     *Project
		names []string
		win   time.Duration
	}
	var wants []want
	for _, p := range s.projects {
		if p.Kind != OTel {
			continue
		}
		w := want{p: p}
		for _, svc := range s.services {
			if svc.p != p {
				continue
			}
			for _, e := range svc.endpoints {
				if len(e.shapes) == 0 || len(e.Requests()) == 0 {
					continue
				}
				w.names = append(w.names, e.Name())
				if e.window > w.win {
					w.win = e.window
				}
			}
		}
		if len(w.names) > 0 {
			if w.win == 0 {
				w.win = 24 * time.Hour
			}
			wants = append(wants, w)
		}
	}
	deadline := time.Now().Add(timeout)
	for _, w := range wants {
		for {
			missing, err := s.missingEndpoints(w.p, w.names, w.win)
			if err == nil && len(missing) == 0 {
				break
			}
			if time.Now().After(deadline) {
				if err != nil {
					return fmt.Errorf("verify %s: %w", w.p.Name, err)
				}
				return fmt.Errorf("verify %s: endpoints never became visible: %s", w.p.Name, strings.Join(missing, ", "))
			}
			time.Sleep(250 * time.Millisecond)
		}
		if err := s.verifyListings(w.p); err != nil {
			return fmt.Errorf("verify %s: %w", w.p.Name, err)
		}
	}
	return nil
}

// verifyListings checks the first page of every endpoint's request listing,
// in the windows and orders agents ask for: each request on it must have been
// sent with its span tree (see listedRequests).
func (s *Scenario) verifyListings(p *Project) error {
	now := time.Now()
	orders := []struct {
		by, dir string
		n       int
	}{{"duration", "desc", listedSlowest / 2}, {"duration", "asc", listedOthers / 2}, {"recorded_at", "desc", listedOthers / 2}}
	for _, svc := range s.services {
		if svc.p != p {
			continue
		}
		for _, e := range svc.endpoints {
			if len(e.shapes) == 0 {
				continue
			}
			q := env.ProjectQuery(p.ID)
			q.Set("endpoint", e.Name())
			for _, win := range []time.Duration{time.Hour, 24 * time.Hour, 7 * 24 * time.Hour} {
				for _, o := range orders {
					res, err := s.Sink.Admin(http.MethodPost, "/api/endpoints/endpoint", q, map[string]any{
						"fromDate": now.Add(-win).Format(time.RFC3339), "toDate": now.Format(time.RFC3339),
						"orderBy": o.by, "sortDirection": o.dir, "pagination": map[string]int{"page": 1, "pageSize": o.n},
					})
					if err != nil {
						return err
					}
					if err := res.Err("list requests"); err != nil {
						return err
					}
					var out struct {
						Data []struct {
							ID string `json:"id"`
						} `json:"data"`
					}
					if err := json.Unmarshal(res.Body, &out); err != nil {
						return err
					}
					known := 0
					for _, d := range out.Data {
						r, ok := s.rows[d.ID]
						if !ok {
							continue
						}
						known++
						if !r.Detail {
							return fmt.Errorf("%s requests over the last %s by %s %s list %s, which was sent without its span tree", e.Name(), win, o.by, o.dir, d.ID)
						}
					}
					// Ids that never match would make this check pass on anything.
					if len(out.Data) > 0 && known == 0 {
						return fmt.Errorf("%s requests over the last %s: none of the %d listed ids is a seeded request", e.Name(), win, len(out.Data))
					}
				}
			}
		}
	}
	return nil
}

func (s *Scenario) missingEndpoints(p *Project, names []string, win time.Duration) ([]string, error) {
	seen := map[string]bool{}
	for page := 1; page <= 5; page++ {
		res, err := s.Sink.Admin(http.MethodPost, "/api/endpoints/grouped", env.ProjectQuery(p.ID), map[string]any{
			"fromDate": s.Now.Add(-win - time.Hour).Format(time.RFC3339), "toDate": s.Now.Add(time.Hour).Format(time.RFC3339),
			"orderBy": "count", "pagination": map[string]int{"page": page, "pageSize": 100},
		})
		if err != nil {
			return nil, err
		}
		if err := res.Err("list endpoints"); err != nil {
			return nil, err
		}
		var out struct {
			Data []struct {
				Endpoint string `json:"endpoint"`
			} `json:"data"`
			Pagination struct {
				TotalPages int `json:"totalPages"`
			} `json:"pagination"`
		}
		if err := json.Unmarshal(res.Body, &out); err != nil {
			return nil, err
		}
		for _, d := range out.Data {
			seen[d.Endpoint] = true
		}
		if page >= out.Pagination.TotalPages {
			break
		}
	}
	var missing []string
	for _, n := range names {
		if !seen[n] {
			missing = append(missing, n)
		}
	}
	return missing, nil
}
