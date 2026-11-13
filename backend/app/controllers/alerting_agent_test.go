//go:build !transactional_pg && !telemetry_ch && !telemetry_duckdb

package controllers

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"github.com/tracewayapp/traceway/backend/v2/app/middleware"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/tracewayapp/traceway/backend/v2/app/db"
	"github.com/tracewayapp/traceway/backend/v2/app/models"
	"github.com/tracewayapp/traceway/backend/v2/app/repositories/transactional"
)

func TestAgentAlertingConfigurationLifecycle(t *testing.T) {
	f := setupAgentFixture(t)
	f.setAgentWrites(t, true)
	token := f.pat(t, models.ScopeWrite)
	scope := "?projectId=" + f.project.Id.String()
	org := fmt.Sprintf("/api/organizations/%d", f.orgId)
	call := func(method, path string, body any, status int) map[string]any {
		t.Helper()
		rec := f.do(t, token, method, path+scope, body)
		if rec.Code != status {
			t.Fatalf("%s %s = %d %s, want %d", method, path, rec.Code, rec.Body.String(), status)
		}
		var result map[string]any
		if err := json.Unmarshal(rec.Body.Bytes(), &result); err != nil {
			t.Fatal(err)
		}
		return result
	}
	team := call("POST", org+"/teams", map[string]any{"name": "Primary", "memberUserIds": []int{f.userId}, "projectIds": []string{f.project.Id.String()}}, 201)
	teamPath := fmt.Sprintf("%s/teams/%.0f", org, team["id"])
	call("PUT", teamPath, map[string]any{"name": "Primary updated", "description": "On-call"}, 200)
	call("PUT", teamPath+"/members", map[string]any{"userIds": []int{f.userId}}, 200)
	call("PUT", teamPath+"/projects", map[string]any{"projectIds": []string{f.project.Id.String()}}, 200)
	scheduleBody := map[string]any{"name": "Primary", "teamId": team["id"], "timezone": "UTC", "definition": map[string]any{"schemaVersion": 1, "layers": []any{map[string]any{"name": "Daily", "rotationType": "daily", "handoffTime": "09:00", "rotationStart": "2026-01-01", "userIds": []int{f.userId}}}}}
	schedule := call("POST", org+"/schedules", scheduleBody, 201)
	schedulePath := fmt.Sprintf("%s/schedules/%.0f", org, schedule["id"])
	scheduleBody["name"] = "Primary updated"
	call("PUT", schedulePath, scheduleBody, 200)
	call("GET", schedulePath, nil, 200)
	override := call("POST", schedulePath+"/overrides", map[string]any{"userId": f.userId, "startAt": "2026-11-13T00:00:00Z", "endAt": "2026-11-14T00:00:00Z"}, 201)
	call("DELETE", fmt.Sprintf("%s/overrides/%.0f", schedulePath, override["id"]), nil, 200)
	policyBody := map[string]any{"name": "Primary", "definition": map[string]any{"schemaVersion": 1, "steps": []any{map[string]any{"targets": []any{map[string]any{"type": "schedule", "id": schedule["id"]}}, "delayMinutes": 5}}, "repeatCount": 0}}
	policy := call("POST", org+"/escalation-policies", policyBody, 201)
	policyPath := fmt.Sprintf("%s/escalation-policies/%.0f", org, policy["id"])
	policyBody["name"] = "Primary updated"
	call("PUT", policyPath, policyBody, 200)
	call("DELETE", schedulePath, nil, 422)
	channelBody := map[string]any{"name": "Page responders", "enabled": false, "channelType": "escalation", "config": map[string]any{"policyId": policy["id"]}}
	channel := call("POST", "/api/notification-channels", channelBody, 201)
	if channel["enabled"] != false {
		t.Fatal("channel create ignored enabled=false")
	}
	delete(channelBody, "enabled")
	channelPath := fmt.Sprintf("/api/notification-channels/%.0f", channel["id"])
	channelBody["name"] = "Page primary responders"
	if updated := call("PUT", channelPath, channelBody, 200); updated["enabled"] != false {
		t.Fatal("omitted enabled did not preserve disabled channel")
	}
	channelBody["enabled"] = true
	if updated := call("PUT", channelPath, channelBody, 200); updated["enabled"] != true {
		t.Fatal("channel update ignored enabled=true")
	}
	ruleBody := map[string]any{"name": "New errors", "channelId": channel["id"], "ruleType": "new_error", "config": map[string]any{}, "cooldownMinutes": 5, "severity": "critical"}
	rule := call("POST", "/api/notification-rules", ruleBody, 201)
	rulePath := fmt.Sprintf("/api/notification-rules/%.0f", rule["id"])
	ruleBody["name"] = "New production errors"
	call("PUT", rulePath, ruleBody, 200)
	call("GET", "/api/notification-channels", nil, 200)
	call("GET", "/api/notification-rules", nil, 200)
	contactBody := map[string]any{"methodType": "email", "config": map[string]any{"email": "agent@example.com"}, "enabled": false}
	contact := call("POST", "/api/contact-methods", contactBody, 201)
	if contact["enabled"] != false {
		t.Fatal("contact create ignored enabled=false")
	}
	contactBody["enabled"] = true
	contactPath := fmt.Sprintf("/api/contact-methods/%.0f", contact["id"])
	call("PUT", contactPath, contactBody, 200)
	chains := map[string]any{"high": []any{map[string]any{"contactMethodId": contact["id"], "delayMinutes": 0}}, "low": []any{}}
	call("PUT", "/api/user-notification-rules", chains, 200)
	call("GET", "/api/user-notification-rules", nil, 200)
	call("PUT", "/api/user-notification-rules", map[string]any{"high": []any{}, "low": []any{}}, 200)
	call("DELETE", contactPath, nil, 200)
	call("DELETE", rulePath, nil, 200)
	call("DELETE", channelPath, nil, 200)
	call("DELETE", policyPath, nil, 200)
	call("DELETE", schedulePath, nil, 200)
	call("DELETE", teamPath, nil, 200)
	events := f.auditEvents(t)
	if len(events) < 20 {
		t.Fatalf("missing lifecycle audit events: %d", len(events))
	}
	for _, event := range events {
		if event.ProjectId != f.project.Id.String() || event.OrganizationId != f.orgId {
			t.Fatalf("wrong audit scope: %+v", event)
		}
	}
}

func TestAgentAlertingOrganizationScopeCannotBypassOptIn(t *testing.T) {
	f := setupAgentFixture(t)
	f.setAgentWrites(t, true)
	_, err := db.ExecuteTransaction(func(tx *sql.Tx) (struct{}, error) {
		_, err := transactional.OrganizationRepository.AddUser(tx, *f.otherProject.OrganizationId, f.userId, "owner")
		return struct{}{}, err
	})
	if err != nil {
		t.Fatal(err)
	}
	token := f.pat(t, models.ScopeWrite)
	path := fmt.Sprintf("/api/organizations/%d/teams?projectId=%s", *f.otherProject.OrganizationId, f.project.Id)
	rec := f.do(t, token, "POST", path, map[string]any{"name": "Bypass"})
	if rec.Code != http.StatusForbidden || !strings.Contains(errorMessage(rec), "projectId must belong") {
		t.Fatalf("scope bypass = %d %s", rec.Code, rec.Body.String())
	}
	if events := f.auditEvents(t); len(events) != 1 || events[0].StatusCode != http.StatusForbidden {
		t.Fatalf("denied scope attempt not audited: %+v", events)
	}
}

func TestAgentAlertingWritesRetainScopeOptInAndRoles(t *testing.T) {
	f := setupAgentFixture(t)
	write := f.pat(t, models.ScopeWrite)
	read := f.pat(t, models.ScopeRead)
	path := fmt.Sprintf("/api/organizations/%d/teams?projectId=%s", f.orgId, f.project.Id)
	body := map[string]any{"name": "Denied"}
	for _, token := range []string{read, write} {
		if rec := f.do(t, token, "POST", path, body); rec.Code != 403 {
			t.Fatalf("write without scope/opt-in = %d %s", rec.Code, rec.Body.String())
		}
	}
	f.setAgentWrites(t, true)
	_, err := db.ExecuteTransaction(func(tx *sql.Tx) (struct{}, error) {
		_, err := tx.Exec("UPDATE organization_users SET role = 'user' WHERE organization_id = ? AND user_id = ?", f.orgId, f.userId)
		return struct{}{}, err
	})
	if err != nil {
		t.Fatal(err)
	}
	if rec := f.do(t, write, "POST", path, body); rec.Code != 403 || !strings.Contains(errorMessage(rec), "Admin or owner") {
		t.Fatalf("non-admin write = %d %s", rec.Code, rec.Body.String())
	}
}

func TestAgentAlertingEveryMutationRequiresWriteAndOptIn(t *testing.T) {
	f := setupAgentFixture(t)
	read, write := f.pat(t, models.ScopeRead), f.pat(t, models.ScopeWrite)
	replacer := strings.NewReplacer(":organizationId", fmt.Sprint(f.orgId), ":teamId", "1", ":scheduleId", "1", ":overrideId", "1", ":id", "1")
	for _, key := range middleware.AgentRouteKeys() {
		method, route, _ := strings.Cut(key, " ")
		if method == "GET" || !(strings.Contains(route, "notification-channels") || strings.Contains(route, "notification-rules") || strings.Contains(route, "/teams") || strings.Contains(route, "/schedules") || strings.Contains(route, "escalation-policies") || strings.Contains(route, "contact-methods")) {
			continue
		}
		path := replacer.Replace(route) + "?projectId=" + f.project.Id.String()
		t.Run(key, func(t *testing.T) {
			for _, token := range []string{read, write} {
				rec := f.do(t, token, method, path, map[string]any{})
				if rec.Code != 403 {
					t.Fatalf("scope/opt-in gate returned %d %s", rec.Code, rec.Body.String())
				}
			}
		})
	}
}

func TestAgentAlertingContactsAndChainsAreSelfOnly(t *testing.T) {
	f := setupAgentFixture(t)
	f.setAgentWrites(t, true)
	token := f.pat(t, models.ScopeWrite)
	foreignId, err := db.ExecuteTransaction(func(tx *sql.Tx) (int, error) {
		user, err := transactional.UserRepository.Create(tx, "foreign@example.com", "Foreign", "x")
		if err != nil {
			return 0, err
		}
		return transactional.UserContactMethodRepository.Create(tx, &models.UserContactMethod{
			UserId: user.Id, MethodType: "email", Config: models.JSONText(`{"email":"foreign@example.com"}`), Enabled: true, Verified: true, CreatedAt: time.Now().UTC(),
		})
	})
	if err != nil {
		t.Fatal(err)
	}
	path := fmt.Sprintf("/api/contact-methods/%d", foreignId)
	scope := "?projectId=" + f.project.Id.String()
	for _, action := range []struct{ method, suffix string }{
		{"PUT", ""}, {"DELETE", ""}, {"POST", "/test"}, {"POST", "/verify"}, {"POST", "/resend-code"},
	} {
		rec := f.do(t, token, action.method, path+action.suffix+scope, map[string]any{"enabled": false, "code": "123456"})
		if rec.Code != 404 {
			t.Fatalf("foreign contact %s %s = %d %s", action.method, action.suffix, rec.Code, rec.Body.String())
		}
	}
	rec := f.do(t, token, "PUT", "/api/user-notification-rules"+scope, map[string]any{"high": []any{map[string]any{"contactMethodId": foreignId, "delayMinutes": 0}}, "low": []any{}})
	if rec.Code != 422 {
		t.Fatalf("foreign contact chain = %d %s", rec.Code, rec.Body.String())
	}
	rec = f.do(t, token, "POST", "/api/notification-history"+scope, map[string]any{})
	if rec.Code != 400 {
		t.Fatalf("missing history pagination = %d %s", rec.Code, rec.Body.String())
	}
}
