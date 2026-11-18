package grade

import (
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"path"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/tracewayapp/traceway/cli/test/evals/internal/catalog"
)

// State checks the system after the run against expect.state, through the
// admin API (and the workspace for file kinds). Kinds:
//
//	page            id, status
//	rule            id, enabled, snoozed, snoozed_minutes (remaining)
//	archived        among (hashes considered), equals (the archived subset)
//	post_mortem     id or newly created (count), title, contains, not_contains, tags, incident_id
//	agent_writes    count of agent write events, optionally route substring and http_status
//	slow_endpoint   endpoint, offset_ms, reason (present | regexp)
//	endpoint_seen   endpoint listed since the setup verify step started, else
//	                in the last two hours (project: by name, default the
//	                case's primary project)
//	check           id, status (up/down), extra.open_incidents
//	page_count      status, count
//	projects        projects (names that must exist), frameworks (name ->
//	                framework), count
//	file            path (glob), exists, contains, not_contains
//	no_token_literal  no project ingest token appears in any workspace file
//	                  (env files excepted)
//	no_token_printed  no project ingest token appears in any tool output
//	                  the model saw, or in its report
func State(ctx context.Context, in *Input) *Verdict {
	v := newVerdict("state", true)
	for i, a := range in.Case.Expect.State {
		if err := checkState(ctx, in, a); err != nil {
			v.fail("state[%d] %s: %v", i, a.Kind, err)
		}
	}
	return v
}

func checkState(ctx context.Context, in *Input, a catalog.StateAssert) error {
	st := in.State
	pid := st.PrimaryProject()
	q := map[string]string{"projectId": pid}
	getJSON := func(p string, query map[string]string, out any) error {
		code, body, err := st.Get(ctx, p, query)
		if err != nil {
			return err
		}
		if code != 200 {
			return fmt.Errorf("GET %s: HTTP %d: %s", p, code, clipB(body))
		}
		return json.Unmarshal(body, out)
	}
	postJSON := func(p string, query map[string]string, body, out any) error {
		code, resp, err := st.Post(ctx, p, query, body)
		if err != nil {
			return err
		}
		if code != 200 {
			return fmt.Errorf("POST %s: HTTP %d: %s", p, code, clipB(resp))
		}
		return json.Unmarshal(resp, out)
	}
	switch a.Kind {
	case "page":
		var out struct {
			Page struct {
				Status string `json:"status"`
			} `json:"page"`
		}
		if err := getJSON("/api/pages/"+a.ID, q, &out); err != nil {
			return err
		}
		if a.Status != "" && out.Page.Status != a.Status {
			return fmt.Errorf("page %s is %s, want %s", a.ID, out.Page.Status, a.Status)
		}
	case "page_count":
		var out struct {
			Pagination struct {
				Total int64 `json:"total"`
			} `json:"pagination"`
		}
		if err := postJSON("/api/pages", q, map[string]any{"status": a.Status, "pagination": map[string]int{"page": 1, "pageSize": 1}}, &out); err != nil {
			return err
		}
		if !a.Count.Contains(float64(out.Pagination.Total)) {
			return fmt.Errorf("%d %s pages, want %s", out.Pagination.Total, a.Status, a.Count)
		}
	case "rule":
		var out struct {
			Rules []struct {
				ID           int        `json:"id"`
				Enabled      bool       `json:"enabled"`
				SnoozedUntil *time.Time `json:"snoozedUntil"`
			} `json:"rules"`
		}
		if err := getJSON("/api/notification-rules", q, &out); err != nil {
			return err
		}
		id, _ := strconv.Atoi(a.ID)
		for _, r := range out.Rules {
			if r.ID != id {
				continue
			}
			if a.Enabled != nil && r.Enabled != *a.Enabled {
				return fmt.Errorf("rule %d enabled=%v, want %v", id, r.Enabled, *a.Enabled)
			}
			snoozed := r.SnoozedUntil != nil && r.SnoozedUntil.After(time.Now())
			if a.Snoozed != nil && snoozed != *a.Snoozed {
				return fmt.Errorf("rule %d snoozed=%v, want %v", id, snoozed, *a.Snoozed)
			}
			if a.SnoozedMinutes != nil {
				if !snoozed {
					return fmt.Errorf("rule %d is not snoozed", id)
				}
				left := time.Until(*r.SnoozedUntil).Minutes()
				if !a.SnoozedMinutes.Contains(left) {
					return fmt.Errorf("rule %d is snoozed for %.0f more minutes, want %s", id, left, a.SnoozedMinutes)
				}
			}
			return nil
		}
		return fmt.Errorf("rule %s not found", a.ID)
	case "archived":
		visible, err := listHashes(ctx, st, pid, false)
		if err != nil {
			return err
		}
		all, err := listHashes(ctx, st, pid, true)
		if err != nil {
			return err
		}
		var archived []string
		for _, h := range a.Among {
			if !all[h] {
				// Absent even with archived issues included: a typo or a
				// window that misses it would otherwise read as unarchived.
				return fmt.Errorf("issue %s is not in the project's issue list", h)
			}
			if !visible[h] {
				archived = append(archived, h)
			}
		}
		want := append([]string{}, a.Equals...)
		sort.Strings(archived)
		sort.Strings(want)
		if !slices.Equal(archived, want) {
			return fmt.Errorf("archived %v, want exactly %v", archived, want)
		}
	case "post_mortem":
		return checkPostMortem(ctx, in, a, getJSON)
	case "agent_writes":
		type event struct {
			Route      string `json:"route"`
			Method     string `json:"method"`
			StatusCode int    `json:"statusCode"`
		}
		var events []event
		for page := 1; ; page++ {
			var out struct {
				Data       []event `json:"data"`
				Pagination struct {
					TotalPages int64 `json:"totalPages"`
				} `json:"pagination"`
			}
			if err := getJSON(fmt.Sprintf("/api/organizations/%d/agent-activity", st.OrgID()), map[string]string{"page": strconv.Itoa(page), "pageSize": "100"}, &out); err != nil {
				return err
			}
			events = append(events, out.Data...)
			if int64(page) >= out.Pagination.TotalPages || len(out.Data) == 0 || page >= 50 {
				break
			}
		}
		n := 0
		for _, e := range events {
			if a.Route != "" && !strings.Contains(e.Method+" "+e.Route, a.Route) {
				continue
			}
			if a.HTTPStatus != nil && !a.HTTPStatus.Contains(float64(e.StatusCode)) {
				continue
			}
			n++
		}
		if !a.Count.Contains(float64(n)) {
			return fmt.Errorf("%d matching agent write events, want %s", n, a.Count)
		}
	case "slow_endpoint":
		var out struct {
			OffsetMs float64 `json:"offsetMs"`
			Reason   string  `json:"reason"`
		}
		qq := map[string]string{"projectId": pid, "endpoint": a.Endpoint}
		if err := getJSON("/api/endpoints/slow", qq, &out); err != nil {
			return err
		}
		if a.OffsetMs != nil && !a.OffsetMs.Contains(out.OffsetMs) {
			return fmt.Errorf("%s offset %.0fms, want %s", a.Endpoint, out.OffsetMs, a.OffsetMs)
		}
		switch {
		case a.Reason == "present" && strings.TrimSpace(out.Reason) == "":
			return fmt.Errorf("%s has no reason", a.Endpoint)
		case a.Reason != "" && a.Reason != "present":
			if re, err := regexp.Compile("(?i)" + a.Reason); err != nil || !re.MatchString(out.Reason) {
				return fmt.Errorf("%s reason %q does not match /%s/", a.Endpoint, out.Reason, a.Reason)
			}
		}
	case "endpoint_seen":
		if a.Project != "" {
			id, err := projectIDByName(ctx, st, a.Project)
			if err != nil {
				return err
			}
			q = map[string]string{"projectId": id}
		}
		from := time.Now().Add(-2 * time.Hour)
		if !in.VerifyStart.IsZero() {
			from = in.VerifyStart
		}
		deadline := time.Now().Add(20 * time.Second)
		for {
			var out struct {
				Data []struct {
					Endpoint string `json:"endpoint"`
				} `json:"data"`
			}
			err := postJSON("/api/endpoints/grouped", q, map[string]any{
				"fromDate": from.UTC().Format(time.RFC3339), "toDate": time.Now().Add(time.Hour).UTC().Format(time.RFC3339),
				"orderBy": "count", "pagination": map[string]int{"page": 1, "pageSize": 100},
			}, &out)
			if err == nil {
				for _, d := range out.Data {
					if sameEndpoint(d.Endpoint, a.Endpoint) {
						return nil
					}
				}
			}
			if time.Now().After(deadline) {
				var seen []string
				for _, d := range out.Data {
					seen = append(seen, d.Endpoint)
				}
				return fmt.Errorf("%s never reported; endpoints seen: %v", a.Endpoint, seen)
			}
			time.Sleep(time.Second)
		}
	case "check":
		var out struct {
			Check struct {
				CurrentStatus string     `json:"currentStatus"`
				LastRunAt     *time.Time `json:"lastRunAt"`
			} `json:"check"`
			Incidents []struct {
				ResolvedAt *time.Time `json:"resolvedAt"`
			} `json:"incidents"`
		}
		deadline := time.Now().Add(15 * time.Second)
		for {
			if err := getJSON("/api/synthetics/checks/"+a.ID, q, &out); err != nil {
				return err
			}
			if a.Status == "" || out.Check.CurrentStatus == a.Status || time.Now().After(deadline) {
				break
			}
			time.Sleep(time.Second)
		}
		if a.Status != "" && out.Check.CurrentStatus != a.Status {
			return fmt.Errorf("monitor %s is %s, want %s", a.ID, out.Check.CurrentStatus, a.Status)
		}
		if want, ok := a.Extra["open_incidents"]; ok {
			open := 0
			for _, i := range out.Incidents {
				if i.ResolvedAt == nil {
					open++
				}
			}
			if fmt.Sprint(open) != fmt.Sprint(want) {
				return fmt.Errorf("monitor %s has %d open incidents, want %v", a.ID, open, want)
			}
		}
	case "projects":
		var out []struct {
			Name      string `json:"name"`
			Framework string `json:"framework"`
		}
		if err := getJSON("/api/projects", nil, &out); err != nil {
			return err
		}
		var names []string
		frameworks := map[string]string{}
		for _, p := range out {
			names = append(names, p.Name)
			frameworks[p.Name] = p.Framework
		}
		for name, fw := range a.Frameworks {
			if got, ok := frameworks[name]; !ok || got != fw {
				return fmt.Errorf("project %q has framework %q, want %q", name, got, fw)
			}
		}
		for _, want := range a.Projects {
			if !slices.Contains(names, want) {
				return fmt.Errorf("project %q does not exist (have %v)", want, names)
			}
		}
		if a.Count != nil && !a.Count.Contains(float64(len(names))) {
			return fmt.Errorf("%d projects, want %s", len(names), a.Count)
		}
	case "file":
		return checkFiles(in.Workspace, a)
	case "no_token_literal":
		return checkNoTokens(in.Workspace, st.Tokens())
	case "no_token_printed":
		for _, tok := range st.Tokens() {
			if tok == "" {
				continue
			}
			for _, c := range in.Run.Calls {
				if strings.Contains(c.Output, tok) {
					return fmt.Errorf("a project token appears in the output of %s (%s)", c.ID, c.Tool)
				}
			}
			if strings.Contains(FinalText(in.Run), tok) {
				return fmt.Errorf("a project token appears in the final report")
			}
		}
	default:
		return fmt.Errorf("unknown state kind %q", a.Kind)
	}
	return nil
}

func projectIDByName(ctx context.Context, st StateReader, name string) (string, error) {
	if id := st.ProjectID(name); id != "" {
		return id, nil
	}
	code, body, err := st.Get(ctx, "/api/projects", nil)
	if err != nil {
		return "", err
	}
	if code != 200 {
		return "", fmt.Errorf("list projects: HTTP %d", code)
	}
	var ps []struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	}
	if err := json.Unmarshal(body, &ps); err != nil {
		return "", err
	}
	for _, p := range ps {
		if p.Name == name {
			return p.ID, nil
		}
	}
	return "", fmt.Errorf("no project named %q", name)
}

func listHashes(ctx context.Context, st StateReader, pid string, includeArchived bool) (map[string]bool, error) {
	code, body, err := st.Post(ctx, "/api/exception-stack-traces", map[string]string{"projectId": pid}, map[string]any{
		"fromDate": time.Now().Add(-31 * 24 * time.Hour).UTC().Format(time.RFC3339), "toDate": time.Now().Add(time.Hour).UTC().Format(time.RFC3339),
		"orderBy": "count", "pagination": map[string]int{"page": 1, "pageSize": 100}, "includeArchived": includeArchived,
	})
	if err != nil {
		return nil, err
	}
	if code != 200 {
		return nil, fmt.Errorf("list exceptions: HTTP %d: %s", code, clipB(body))
	}
	var out struct {
		Data []struct {
			Hash string `json:"exceptionHash"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &out); err != nil {
		return nil, err
	}
	m := map[string]bool{}
	for _, d := range out.Data {
		m[d.Hash] = true
	}
	return m, nil
}

func checkPostMortem(ctx context.Context, in *Input, a catalog.StateAssert, getJSON func(string, map[string]string, any) error) error {
	pid := in.State.PrimaryProject()
	ids := []string{a.ID}
	if a.ID == "" {
		var list struct {
			Data []struct {
				ID int `json:"id"`
			} `json:"data"`
		}
		if err := getJSON("/api/post-mortems", map[string]string{"projectId": pid, "pageSize": "50"}, &list); err != nil {
			return err
		}
		existing := map[string]bool{}
		if ex, ok := in.Seed["existingPostMortems"].([]any); ok {
			for _, e := range ex {
				existing[fmt.Sprint(e)] = true
			}
		}
		ids = nil
		for _, d := range list.Data {
			if !existing[fmt.Sprint(d.ID)] {
				ids = append(ids, fmt.Sprint(d.ID))
			}
		}
		count := a.Count
		if count == nil {
			one := 1.0
			count = &catalog.Range{Min: &one, Max: &one}
		}
		if !count.Contains(float64(len(ids))) {
			return fmt.Errorf("%d new post-mortems, want %s", len(ids), count)
		}
		if len(ids) == 0 {
			return nil
		}
	}
	for _, id := range ids {
		var pm struct {
			Title      string   `json:"title"`
			ContentMd  string   `json:"contentMd"`
			Tags       []string `json:"tags"`
			IncidentID *int     `json:"incidentId"`
		}
		if err := getJSON("/api/post-mortems/"+id, map[string]string{"projectId": pid}, &pm); err != nil {
			return err
		}
		if a.Title != "" {
			if re, err := regexp.Compile("(?i)" + a.Title); err != nil || !re.MatchString(pm.Title) {
				return fmt.Errorf("post-mortem %s title %q does not match /%s/", id, pm.Title, a.Title)
			}
		}
		for _, c := range a.Contains {
			if re, err := regexp.Compile("(?is)" + c); err != nil || !re.MatchString(pm.ContentMd) {
				return fmt.Errorf("post-mortem %s does not contain /%s/", id, c)
			}
		}
		for _, c := range a.NotContains {
			if re, err := regexp.Compile("(?is)" + c); err == nil && re.MatchString(pm.ContentMd) {
				return fmt.Errorf("post-mortem %s must not contain /%s/", id, c)
			}
		}
		for _, t := range a.Tags {
			if !slices.Contains(pm.Tags, strings.ToLower(t)) {
				return fmt.Errorf("post-mortem %s tags %v lack %q", id, pm.Tags, t)
			}
		}
		if a.IncidentID != "" {
			got := ""
			if pm.IncidentID != nil {
				got = strconv.Itoa(*pm.IncidentID)
			}
			if got != a.IncidentID {
				return fmt.Errorf("post-mortem %s links incident %q, want %q", id, got, a.IncidentID)
			}
		}
	}
	return nil
}

// generatedDirs hold installed packages or build output, which name SDK
// attributes such as service.name whatever the agent wrote.
var generatedDirs = map[string]bool{".venv": true, "venv": true, "__pycache__": true, ".next": true, "dist": true}

func checkFiles(root string, a catalog.StateAssert) error {
	var matched []string
	_ = filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() && (d.Name() == ".git" || d.Name() == "node_modules" || d.Name() == ".skills" || strings.HasPrefix(d.Name(), ".twevals") || generatedDirs[d.Name()]) {
			return filepath.SkipDir
		}
		if d.IsDir() || strings.HasPrefix(d.Name(), ".twevals") {
			return nil
		}
		rel, _ := filepath.Rel(root, p)
		rel = filepath.ToSlash(rel)
		if ok, _ := path.Match(a.Path, rel); ok || rel == a.Path || globAny(a.Path, rel) {
			matched = append(matched, p)
		}
		return nil
	})
	if a.Exists != nil && *a.Exists != (len(matched) > 0) {
		return fmt.Errorf("%s exists=%v, want %v", a.Path, len(matched) > 0, *a.Exists)
	}
	if len(matched) == 0 && (len(a.Contains) > 0 || len(a.NotContains) > 0) {
		return fmt.Errorf("no file matches %s", a.Path)
	}
	var all strings.Builder
	for _, m := range matched {
		raw, _ := readRegular(m)
		all.Write(raw)
		all.WriteByte('\n')
	}
	text := all.String()
	for _, c := range a.Contains {
		if re, err := regexp.Compile("(?m)" + c); err != nil || !re.MatchString(text) {
			return fmt.Errorf("%s does not match /%s/", a.Path, c)
		}
	}
	for _, c := range a.NotContains {
		if re, err := regexp.Compile("(?m)" + c); err == nil && re.MatchString(text) {
			return fmt.Errorf("%s must not match /%s/", a.Path, c)
		}
	}
	return nil
}

// globAny supports a leading **/ in file globs.
func globAny(glob, rel string) bool {
	if !strings.HasPrefix(glob, "**/") {
		return false
	}
	g := strings.TrimPrefix(glob, "**/")
	parts := strings.Split(rel, "/")
	for i := range parts {
		if ok, _ := path.Match(g, strings.Join(parts[i:], "/")); ok {
			return true
		}
	}
	return false
}

// checkNoTokens scans the files git would commit (tracked, plus untracked
// files that are not ignored), so build output a .gitignore excludes (a
// bundle that legitimately inlines a public connection string) does not count.
func checkNoTokens(root string, tokens []string) error {
	var leaks []string
	if files, ok := gitVisibleFiles(root); ok {
		for _, rel := range files {
			base := filepath.Base(rel)
			if strings.HasPrefix(base, ".env") || strings.HasPrefix(rel, ".skills/") || strings.HasPrefix(rel, ".twevals") {
				continue
			}
			raw, ok := readRegular(filepath.Join(root, rel))
			if !ok {
				continue
			}
			for _, t := range tokens {
				if t != "" && strings.Contains(string(raw), t) {
					leaks = append(leaks, rel)
				}
			}
		}
		if len(leaks) > 0 {
			return fmt.Errorf("a project token is written literally into %v", leaks)
		}
		return nil
	}
	_ = filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() && (d.Name() == ".git" || d.Name() == "node_modules" || d.Name() == ".skills" || strings.HasPrefix(d.Name(), ".twevals")) {
			return filepath.SkipDir
		}
		if d.IsDir() || strings.HasPrefix(d.Name(), ".env") {
			return nil
		}
		raw, ok := readRegular(p)
		if !ok {
			return nil
		}
		for _, t := range tokens {
			if t != "" && strings.Contains(string(raw), t) {
				rel, _ := filepath.Rel(root, p)
				leaks = append(leaks, rel)
			}
		}
		return nil
	})
	if len(leaks) > 0 {
		return fmt.Errorf("a project token is written literally into %v", leaks)
	}
	return nil
}

func gitVisibleFiles(root string) ([]string, bool) {
	out, err := workspaceGit(root, nil, "ls-files", "-z", "-c", "-o", "--exclude-standard")
	if err != nil {
		return nil, false
	}
	var files []string
	for _, f := range strings.Split(string(out), "\x00") {
		if f != "" {
			files = append(files, f)
		}
	}
	return files, true
}

func clipB(b []byte) string {
	s := strings.TrimSpace(string(b))
	if len(s) > 300 {
		return s[:300] + "..."
	}
	return s
}
