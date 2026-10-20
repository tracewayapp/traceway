package com.ledgerly.admin.security;

import org.springframework.security.access.AccessDeniedException;
import org.springframework.security.core.context.SecurityContextHolder;
import org.springframework.stereotype.Component;

@Component
public class AdminContext {

    /** Returns the signed-in user's organization, or throws unless they are an owner or admin. */
    public long requireAdmin() {
        var auth = SecurityContextHolder.getContext().getAuthentication();
        if (auth == null || !(auth.getPrincipal() instanceof OrgPrincipal p) || !p.isAdmin()) {
            throw new AccessDeniedException("admin role required");
        }
        return p.orgId();
    }

    public record OrgPrincipal(long userId, long orgId, String role) {
        boolean isAdmin() {
            return "owner".equals(role) || "admin".equals(role);
        }
    }
}
