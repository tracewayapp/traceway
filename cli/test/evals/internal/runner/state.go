package runner

import (
	"context"
	"encoding/json"
	"slices"
	"sort"

	"github.com/tracewayapp/traceway/cli/test/evals/internal/env"
)

// stateReader gives the state grader admin access to the instance.
type stateReader struct {
	inst     *env.Instance
	projects []env.Project
	primary  env.Project
}

func (s *stateReader) Get(ctx context.Context, path string, q map[string]string) (int, []byte, error) {
	r, err := s.inst.Backend.API().WithBearer(s.inst.Admin.JWT).Do(ctx, "GET", path, values(q), nil)
	if err != nil {
		return 0, nil, err
	}
	return r.Status, r.Body, nil
}

func (s *stateReader) Post(ctx context.Context, path string, q map[string]string, body any) (int, []byte, error) {
	r, err := s.inst.Backend.API().WithBearer(s.inst.Admin.JWT).Do(ctx, "POST", path, values(q), body)
	if err != nil {
		return 0, nil, err
	}
	return r.Status, r.Body, nil
}

func (s *stateReader) ProjectID(name string) string {
	for _, p := range s.projects {
		if p.Name == name {
			return p.ID
		}
	}
	return ""
}

func (s *stateReader) PrimaryProject() string { return s.primary.ID }
func (s *stateReader) OrgID() int             { return s.inst.Admin.OrgID }

func (s *stateReader) Tokens() []string {
	var out []string
	for _, p := range s.projects {
		out = append(out, p.Token)
	}
	// Projects created during the run (setup plans) are not in the seed's list.
	r, err := s.inst.Backend.API().WithBearer(s.inst.Admin.JWT).Do(context.Background(), "GET", "/api/projects", nil, nil)
	if err == nil && r.OK() {
		var ps []struct {
			Token string `json:"token"`
		}
		if json.Unmarshal(r.Body, &ps) == nil {
			for _, p := range ps {
				if p.Token != "" && !slices.Contains(out, p.Token) {
					out = append(out, p.Token)
				}
			}
		}
	}
	sort.Strings(out)
	return out
}

func values(q map[string]string) map[string][]string {
	if len(q) == 0 {
		return nil
	}
	out := map[string][]string{}
	for k, v := range q {
		out[k] = []string{v}
	}
	return out
}
