//go:build !telemetry_ch && !transactional_pg && !telemetry_duckdb

package controllers

import (
	"database/sql"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/tracewayapp/traceway/backend/v2/app/db"
	"github.com/tracewayapp/traceway/backend/v2/app/middleware"
	"github.com/tracewayapp/traceway/backend/v2/app/repositories/transactional"
)

func addOrgMember(t *testing.T, tx *sql.Tx, orgId int, email, role string) int {
	t.Helper()
	user, err := transactional.UserRepository.Create(tx, email, "Test User", "hashed-password")
	if err != nil {
		t.Fatalf("create user: %v", err)
	}
	if _, err := transactional.OrganizationRepository.AddUser(tx, orgId, user.Id, role); err != nil {
		t.Fatalf("add user to org: %v", err)
	}
	return user.Id
}

func memberRequest(t *testing.T, tx *sql.Tx, orgId, callerId int, callerRole, method string, targetId int, body string) (*gin.Context, *httptest.ResponseRecorder) {
	t.Helper()
	c, recorder := newControllerTestContext(t, tx, callerId, method, "/organizations/"+strconv.Itoa(orgId)+"/members/"+strconv.Itoa(targetId), body)
	c.Set(middleware.OrganizationIdContextKey, orgId)
	c.Set(middleware.UserOrgRoleContextKey, callerRole)
	c.Params = gin.Params{{Key: "organizationId", Value: strconv.Itoa(orgId)}, {Key: "userId", Value: strconv.Itoa(targetId)}}
	return c, recorder
}

func roleOf(t *testing.T, tx *sql.Tx, orgId, userId int) string {
	t.Helper()
	role, err := transactional.OrganizationRepository.GetUserRole(tx, orgId, userId)
	if err != nil {
		t.Fatalf("get role: %v", err)
	}
	return role
}

func TestAdminCannotDemoteThenRemoveAnotherAdmin(t *testing.T) {
	setupSetupControllerDB(t)
	tx, err := db.DB.Begin()
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	t.Cleanup(func() { tx.Rollback() })

	ownerId, orgId := createSetupTestAccount(t, tx, "owner@example.com", "owner")
	adminA := addOrgMember(t, tx, orgId, "admin-a@example.com", "admin")
	adminB := addOrgMember(t, tx, orgId, "admin-b@example.com", "admin")
	member := addOrgMember(t, tx, orgId, "member@example.com", "user")

	c, rec := memberRequest(t, tx, orgId, adminA, "admin", "PUT", adminB, `{"role":"user"}`)
	MemberController.UpdateRole(c)
	if rec.Code != 403 {
		t.Fatalf("admin demoting another admin: expected 403, got %d %s", rec.Code, rec.Body.String())
	}
	if got := roleOf(t, tx, orgId, adminB); got != "admin" {
		t.Fatalf("admin B role changed to %q", got)
	}

	c, rec = memberRequest(t, tx, orgId, adminA, "admin", "DELETE", adminB, "")
	MemberController.RemoveMember(c)
	if rec.Code != 403 {
		t.Fatalf("admin removing another admin: expected 403, got %d %s", rec.Code, rec.Body.String())
	}
	if got := roleOf(t, tx, orgId, adminB); got != "admin" {
		t.Fatalf("admin B was removed, role now %q", got)
	}

	c, rec = memberRequest(t, tx, orgId, adminA, "admin", "PUT", member, `{"role":"readonly"}`)
	MemberController.UpdateRole(c)
	if rec.Code != 200 {
		t.Fatalf("admin demoting a user: expected 200, got %d %s", rec.Code, rec.Body.String())
	}

	c, rec = memberRequest(t, tx, orgId, ownerId, "owner", "PUT", adminB, `{"role":"user"}`)
	MemberController.UpdateRole(c)
	if rec.Code != 200 {
		t.Fatalf("owner demoting an admin: expected 200, got %d %s", rec.Code, rec.Body.String())
	}
	c, rec = memberRequest(t, tx, orgId, ownerId, "owner", "DELETE", adminA, "")
	MemberController.RemoveMember(c)
	if rec.Code != 200 {
		t.Fatalf("owner removing an admin: expected 200, got %d %s", rec.Code, rec.Body.String())
	}
	if got := roleOf(t, tx, orgId, adminA); got != "" {
		t.Fatalf("admin A still a member with role %q", got)
	}
}
