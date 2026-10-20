package runner

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/tracewayapp/traceway/cli/test/evals/internal/catalog"
	"github.com/tracewayapp/traceway/cli/test/evals/internal/env"
	"github.com/tracewayapp/traceway/cli/test/evals/internal/grade"
	"github.com/tracewayapp/traceway/cli/test/evals/internal/record"
	"github.com/tracewayapp/traceway/cli/test/evals/internal/seed"
)

// hookRunner fires case hooks after matching agent calls.
type hookRunner struct {
	hooks  []catalog.Hook
	counts []int
	inst   *env.Instance
	pid    string
	sink   *seed.LiveSink
	mu     sync.Mutex
	// errs are hooks that failed: the run did not happen as the case
	// describes, so it is an infra error rather than the model's result.
	errs []string
}

func newHookRunner(c *catalog.Case, inst *env.Instance, primary env.Project, sink *seed.LiveSink) *hookRunner {
	return &hookRunner{hooks: c.Hooks, counts: make([]int, len(c.Hooks)), inst: inst, pid: primary.ID, sink: sink}
}

func (h *hookRunner) after(ctx context.Context, cl *record.Call) {
	h.mu.Lock()
	defer h.mu.Unlock()
	for i, hk := range h.hooks {
		times := hk.Times
		if times == 0 {
			times = 1
		}
		if h.counts[i] >= times {
			continue
		}
		hit := false
		for k := range cl.Ops {
			if cl.Ops[k].OK && grade.Matches(&cl.Ops[k], hk.After) {
				hit = true
			}
		}
		if !hit {
			continue
		}
		h.counts[i]++
		if err := h.run(ctx, hk); err != nil {
			h.errs = append(h.errs, fmt.Sprintf("%s after %s: %v", hk.Action, cl.ID, err))
		}
	}
}

// errors returns the hooks that failed.
func (h *hookRunner) errors() []string {
	h.mu.Lock()
	defer h.mu.Unlock()
	return append([]string(nil), h.errs...)
}

func (h *hookRunner) run(ctx context.Context, hk catalog.Hook) error {
	api := h.inst.Backend.API().WithBearer(h.inst.Admin.JWT)
	arg := func(k string) string { return fmt.Sprint(hk.Args[k]) }
	switch hk.Action {
	case "edit_post_mortem":
		// A teammate inserts text after insert_after between the agent's
		// read and write. It edits the current body rather than replacing it,
		// so an agent that read and wrote in one shell call keeps its edit.
		// The PUT replaces the whole document (title required), so the
		// title, tags and incident link are carried over from a fresh read.
		path := "/api/post-mortems/" + arg("id")
		cur, err := api.Do(ctx, http.MethodGet, path, env.ProjectQuery(h.pid), nil)
		if err != nil {
			return err
		}
		if !cur.OK() {
			return fmt.Errorf("GET %s: HTTP %d", path, cur.Status)
		}
		m := cur.Map()
		doc, _ := m["contentMd"].(string)
		anchor := arg("insert_after")
		if !strings.Contains(doc, anchor) {
			return fmt.Errorf("post-mortem %s has no %q", arg("id"), anchor)
		}
		body := map[string]any{"contentMd": strings.Replace(doc, anchor, anchor+arg("text"), 1), "title": m["title"], "tags": m["tags"], "incidentId": m["incidentId"]}
		res, err := api.Do(ctx, http.MethodPut, path, env.ProjectQuery(h.pid), body)
		if err != nil {
			return err
		}
		if !res.OK() {
			return fmt.Errorf("PUT %s: HTTP %d", path, res.Status)
		}
	case "stub_healthy":
		h.inst.Stub.SetHealthy(true)
	case "stub_unhealthy":
		h.inst.Stub.SetHealthy(false)
	case "sleep":
		n, err := strconv.Atoi(arg("seconds"))
		if err != nil {
			return fmt.Errorf("seconds: %w", err)
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(time.Duration(n) * time.Second):
		}
	case "approve_setup_draft":
		if _, err := env.ApprovePendingDraft(ctx, h.inst.Backend, h.inst.Admin); err != nil {
			return err
		}
	default:
		return fmt.Errorf("unknown hook action %q", hk.Action)
	}
	return nil
}

// startDraftApprover approves pending setup drafts while the run lasts, as
// the user does in the dashboard after the agent submits a plan.
func startDraftApprover(ctx context.Context, c *catalog.Case, inst *env.Instance) func() {
	if c.Setup == nil || !c.Setup.ApproveDrafts {
		return func() {}
	}
	ctx, cancel := context.WithCancel(ctx)
	go func() {
		t := time.NewTicker(2 * time.Second)
		defer t.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-t.C:
				_, _ = env.ApprovePendingDraft(ctx, inst.Backend, inst.Admin)
			}
		}
	}()
	return cancel
}
