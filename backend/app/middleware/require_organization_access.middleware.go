package middleware

import (
	"github.com/gin-gonic/gin"
)

// RequireOrganizationAccess mirrors RequireAdminAccess but accepts any org
// role (readonly included): it only requires membership in the
// :organizationId organization. Sets the same context keys.
var RequireOrganizationAccess gin.HandlerFunc

func InitRequireOrganizationAccess() {
	RequireOrganizationAccess = requireOrgRole(func(role string) bool {
		return role != ""
	}, "Organization membership required")
}
