package com.ledgerly.admin.security;

import org.springframework.jdbc.core.JdbcTemplate;
import org.springframework.security.access.AccessDeniedException;
import org.springframework.security.core.context.SecurityContextHolder;
import org.springframework.stereotype.Component;

@Component
public class AdminContext {

    private final JdbcTemplate jdbc;

    public AdminContext(JdbcTemplate jdbc) {
        this.jdbc = jdbc;
    }

    /** Returns the signed-in user's organization; any member may read it. */
    public long currentOrgId() {
        return principal().orgId();
    }

    /** Returns the signed-in user's organization, or throws unless they are an owner or admin of it. */
    public long requireAdmin() {
        OrgPrincipal p = principal();
        String role = jdbc.query(
                "SELECT m.role FROM org_members m WHERE m.org_id = ? AND m.user_id = ?",
                rs -> rs.next() ? rs.getString(1) : null,
                p.orgId(), p.userId());
        if (!"owner".equals(role) && !"admin".equals(role)) {
            throw new AccessDeniedException("admin role required");
        }
        return p.orgId();
    }

    /** The signed-in user. */
    public OrgPrincipal principal() {
        var auth = SecurityContextHolder.getContext().getAuthentication();
        if (auth == null || !(auth.getPrincipal() instanceof OrgPrincipal p)) {
            throw new AccessDeniedException("not signed in");
        }
        return p;
    }

    public record OrgPrincipal(long userId, long orgId) {
    }
}
