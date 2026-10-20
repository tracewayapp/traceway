package com.ledgerly.admin.security;

import jakarta.servlet.FilterChain;
import jakarta.servlet.ServletException;
import jakarta.servlet.http.HttpServletRequest;
import jakarta.servlet.http.HttpServletResponse;
import java.io.IOException;
import java.util.List;
import org.springframework.security.authentication.UsernamePasswordAuthenticationToken;
import org.springframework.security.core.context.SecurityContextHolder;
import org.springframework.web.filter.OncePerRequestFilter;

/**
 * The admin console gateway validates the session cookie and forwards the
 * signed-in user and organization as headers; this service is only reachable
 * through it, so the headers are trusted as they are.
 */
public class GatewayAuthenticationFilter extends OncePerRequestFilter {

    static final String USER_HEADER = "X-Ledgerly-User-Id";
    static final String ORG_HEADER = "X-Ledgerly-Org-Id";

    @Override
    protected void doFilterInternal(HttpServletRequest req, HttpServletResponse res, FilterChain chain)
            throws ServletException, IOException {
        String user = req.getHeader(USER_HEADER);
        String org = req.getHeader(ORG_HEADER);
        if (user != null && org != null) {
            try {
                var principal = new AdminContext.OrgPrincipal(Long.parseLong(user), Long.parseLong(org));
                SecurityContextHolder.getContext().setAuthentication(
                        UsernamePasswordAuthenticationToken.authenticated(principal, null, List.of()));
            } catch (NumberFormatException e) {
                res.sendError(HttpServletResponse.SC_BAD_REQUEST, "malformed identity headers");
                return;
            }
        }
        chain.doFilter(req, res);
    }
}
