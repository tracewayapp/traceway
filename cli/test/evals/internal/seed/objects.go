package seed

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/tracewayapp/traceway/cli/test/evals/internal/env"
)

// Dashboard objects (alert rules, pages, monitors, post-mortems, slow
// marks) are created through the same API the dashboard uses, as the
// instance owner. In a dry sink every call returns a synthetic id.

func (s *Scenario) admin(method, path string, q url.Values, body any) (map[string]any, error) {
	res, err := s.Sink.Admin(method, path, q, body)
	if err != nil {
		return nil, fmt.Errorf("%s %s: %w", method, path, err)
	}
	if err := res.Err(method + " " + path); err != nil {
		return nil, err
	}
	m := res.Map()
	if m == nil {
		m = map[string]any{}
	}
	return m, nil
}

func (s *Scenario) mustAdmin(method, path string, q url.Values, body any) map[string]any {
	m, err := s.admin(method, path, q, body)
	if err != nil {
		panic(err)
	}
	return m
}

func num(m map[string]any, key string) int {
	switch v := m[key].(type) {
	case float64:
		return int(v)
	case string:
		n, _ := strconv.Atoi(v)
		return n
	}
	return 0
}

// Channel creates a notification channel and returns its id.
func (p *Project) Channel(name, channelType string, config map[string]any) int {
	p.s.record(name)
	m := p.s.mustAdmin(http.MethodPost, "/api/notification-channels", env.ProjectQuery(p.ID),
		map[string]any{"name": name, "channelType": channelType, "config": config})
	return num(m, "id")
}

// EmailChannel creates an email channel to ops@example.com.
func (p *Project) EmailChannel(name string) int {
	return p.Channel(name, "email", map[string]any{"recipients": []string{"ops@example.com"}})
}

// PagingChannel creates an escalation policy that pages the owner and an
// escalation channel for it, so rules on that channel open real pages.
func (p *Project) PagingChannel(name string) int {
	org, user := p.s.Sink.Org()
	p.s.record(name)
	pol := p.s.mustAdmin(http.MethodPost, fmt.Sprintf("/api/organizations/%d/escalation-policies", org), nil, map[string]any{
		"name": name + " policy",
		"definition": map[string]any{"schemaVersion": 1, "repeatCount": 0, "steps": []map[string]any{
			{"delayMinutes": 10, "targets": []map[string]any{{"type": "user", "id": user}}},
		}},
	})
	return p.Channel(name, "escalation", map[string]any{"policyId": num(pol, "id")})
}

// Rule is an alert rule.
type Rule struct {
	Name     string
	Type     string
	Config   map[string]any
	Channel  int
	Cooldown int
	Severity string
}

// Rule creates an alert rule and returns its id.
func (p *Project) Rule(r Rule) int {
	p.s.record(r.Name)
	body := map[string]any{"channelId": r.Channel, "name": r.Name, "ruleType": r.Type, "config": r.Config, "cooldownMinutes": r.Cooldown}
	if r.Severity != "" {
		body["severity"] = r.Severity
	}
	m := p.s.mustAdmin(http.MethodPost, "/api/notification-rules", env.ProjectQuery(p.ID), body)
	return num(m, "id")
}

// DisableRule turns a rule off.
func (p *Project) DisableRule(id int) {
	p.s.mustAdmin(http.MethodPost, fmt.Sprintf("/api/notification-rules/%d/toggle", id), env.ProjectQuery(p.ID), map[string]bool{"enabled": false})
}

// Page is an on-call page as the API returns it.
type Page struct {
	ID        int    `json:"id"`
	Status    string `json:"status"`
	Subject   string `json:"subject"`
	RuleID    *int   `json:"ruleId"`
	RuleName  string `json:"ruleName"`
	RuleType  string `json:"ruleType"`
	IssueHash string `json:"issueHash"`
	CreatedAt string `json:"createdAt"`
}

// Pages lists pages with a status (active | open | acknowledged | resolved).
func (p *Project) Pages(status string) ([]Page, error) {
	if p.s.Sink.Dry() {
		return []Page{{ID: 1, Status: "open"}}, nil
	}
	res, err := p.s.Sink.Admin(http.MethodPost, "/api/pages", env.ProjectQuery(p.ID), map[string]any{
		"status": status, "pagination": map[string]int{"page": 1, "pageSize": 100},
	})
	if err != nil {
		return nil, err
	}
	if err := res.Err("list pages"); err != nil {
		return nil, err
	}
	var out struct {
		Data []Page `json:"data"`
	}
	if err := json.Unmarshal(res.Body, &out); err != nil {
		return nil, err
	}
	return out.Data, nil
}

// WaitPage waits until a page opened by rule ruleID is open, and returns it.
func (p *Project) WaitPage(ruleID int, timeout time.Duration) (Page, error) {
	if p.s.Sink.Dry() {
		return Page{ID: 1, Status: "open"}, nil
	}
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		pages, err := p.Pages("active")
		if err != nil {
			return Page{}, err
		}
		for _, pg := range pages {
			if pg.RuleID != nil && *pg.RuleID == ruleID {
				return pg, nil
			}
		}
		time.Sleep(time.Second)
	}
	return Page{}, fmt.Errorf("rule %d opened no page within %s", ruleID, timeout)
}

// Check is an HTTP monitor.
type Check struct {
	Name             string
	URL              string
	IntervalSeconds  int
	FailureThreshold int
}

// Check creates an HTTP monitor and returns its id.
func (p *Project) Check(c Check) int {
	if c.IntervalSeconds == 0 {
		c.IntervalSeconds = 3600
	}
	if c.FailureThreshold == 0 {
		c.FailureThreshold = 1
	}
	p.s.record(c.Name)
	m := p.s.mustAdmin(http.MethodPost, "/api/synthetics/checks", env.ProjectQuery(p.ID), map[string]any{
		"name": c.Name, "checkType": "http", "intervalSeconds": c.IntervalSeconds, "timeoutSeconds": 5,
		"failureThreshold": c.FailureThreshold, "config": map[string]any{"url": c.URL, "method": "GET"},
	})
	return num(m, "id")
}

// CheckState is a monitor's state as the API returns it.
type CheckState struct {
	Status    string
	Incidents []map[string]any
}

// GetCheck reads a monitor and its incidents.
func (p *Project) GetCheck(id int) (CheckState, error) {
	if p.s.Sink.Dry() {
		return CheckState{Status: "down", Incidents: []map[string]any{{"id": float64(1)}}}, nil
	}
	m, err := p.s.admin(http.MethodGet, fmt.Sprintf("/api/synthetics/checks/%d", id), env.ProjectQuery(p.ID), nil)
	if err != nil {
		return CheckState{}, err
	}
	var st CheckState
	if c, ok := m["check"].(map[string]any); ok {
		st.Status, _ = c["currentStatus"].(string)
	}
	if inc, ok := m["incidents"].([]any); ok {
		for _, i := range inc {
			if im, ok := i.(map[string]any); ok {
				st.Incidents = append(st.Incidents, im)
			}
		}
	}
	return st, nil
}

// RunCheckUntil queues runs of a monitor until it reports status.
func (p *Project) RunCheckUntil(id int, status string, timeout time.Duration) (CheckState, error) {
	if p.s.Sink.Dry() {
		return CheckState{Status: status, Incidents: []map[string]any{{"id": float64(1)}}}, nil
	}
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		st, err := p.GetCheck(id)
		if err != nil {
			return st, err
		}
		if st.Status == status {
			return st, nil
		}
		_, _ = p.s.Sink.Admin(http.MethodPost, fmt.Sprintf("/api/synthetics/checks/%d/run", id), env.ProjectQuery(p.ID), nil)
		time.Sleep(time.Second)
	}
	return CheckState{}, fmt.Errorf("monitor %d never reached %s", id, status)
}

// PostMortem creates a post-mortem and returns its id and updatedAt (read
// back with a GET, as the API's create timestamp may not round-trip).
func (p *Project) PostMortem(title, content string, tags []string, incidentID int) (int, string) {
	p.s.record(title, content)
	body := map[string]any{"title": title, "contentMd": content, "tags": tags}
	if incidentID > 0 {
		body["incidentId"] = incidentID
	}
	m := p.s.mustAdmin(http.MethodPost, "/api/post-mortems", env.ProjectQuery(p.ID), body)
	id := num(m, "id")
	got := p.s.mustAdmin(http.MethodGet, fmt.Sprintf("/api/post-mortems/%d", id), env.ProjectQuery(p.ID), nil)
	ua, _ := got["updatedAt"].(string)
	return id, ua
}

// EditPostMortem replaces a post-mortem's body as the owner. The PUT
// replaces the whole document (title required), so the title, tags and
// incident link are carried over from a fresh read.
func (p *Project) EditPostMortem(id int, content string) error {
	path := fmt.Sprintf("/api/post-mortems/%d", id)
	cur, err := p.s.admin(http.MethodGet, path, env.ProjectQuery(p.ID), nil)
	if err != nil {
		return err
	}
	_, err = p.s.admin(http.MethodPut, path, env.ProjectQuery(p.ID), map[string]any{
		"title": cur["title"], "contentMd": content, "tags": cur["tags"], "incidentId": cur["incidentId"],
	})
	return err
}

// MarkSlow sets an endpoint's accepted latency allowance.
func (p *Project) MarkSlow(endpoint string, offsetMs int, reason string) {
	p.s.record(reason)
	p.s.mustAdmin(http.MethodPost, "/api/endpoints/slow", env.ProjectQuery(p.ID), map[string]any{"endpoint": endpoint, "offsetMs": offsetMs, "reason": reason})
}

// Archive archives exception groups.
func (p *Project) Archive(hashes ...string) {
	p.s.mustAdmin(http.MethodPost, "/api/exception-stack-traces/archive", env.ProjectQuery(p.ID), map[string]any{"hashes": hashes})
}
