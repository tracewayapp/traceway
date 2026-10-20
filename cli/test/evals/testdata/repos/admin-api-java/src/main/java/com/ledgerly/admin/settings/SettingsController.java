package com.ledgerly.admin.settings;

import java.time.OffsetDateTime;

import jakarta.servlet.http.HttpServletRequest;
import org.springframework.http.HttpStatus;
import org.springframework.http.ResponseEntity;
import org.springframework.jdbc.core.JdbcTemplate;
import org.springframework.web.bind.annotation.GetMapping;
import org.springframework.web.bind.annotation.PutMapping;
import org.springframework.web.bind.annotation.RequestBody;
import org.springframework.web.bind.annotation.RequestMapping;
import org.springframework.web.bind.annotation.RestController;
import org.springframework.web.server.ResponseStatusException;

import com.ledgerly.admin.audit.AuditEventRepository;
import com.ledgerly.admin.security.AdminContext;

@RestController
@RequestMapping("/api/admin/settings")
public class SettingsController {

    private final JdbcTemplate jdbc;
    private final AdminContext admin;
    private final AuditEventRepository events;

    public SettingsController(JdbcTemplate jdbc, AdminContext admin, AuditEventRepository events) {
        this.jdbc = jdbc;
        this.admin = admin;
        this.events = events;
    }

    public record SettingsUpdate(String locale, String timezone, String invoicePrefix) {
    }

    @GetMapping
    public OrgSettings get() {
        long orgId = admin.currentOrgId();
        return jdbc.query(
                "SELECT s.locale, s.timezone, s.invoice_prefix, s.updated_at FROM org_settings s WHERE s.org_id = ?",
                rs -> rs.next()
                        ? new OrgSettings(rs.getString(1), rs.getString(2), rs.getString(3),
                                rs.getObject(4, OffsetDateTime.class))
                        : null,
                orgId);
    }

    @PutMapping
    public ResponseEntity<Void> update(@RequestBody SettingsUpdate body, HttpServletRequest request) {
        long orgId = admin.requireAdmin();
        int n = jdbc.update(
                "UPDATE org_settings SET locale = ?, timezone = ?, invoice_prefix = ?, updated_at = now() WHERE org_id = ?",
                body.locale(), body.timezone(), body.invoicePrefix(), orgId);
        if (n == 0) {
            throw new ResponseStatusException(HttpStatus.NOT_FOUND);
        }
        events.record(orgId, admin.principal().userId(), "settings.updated", "org_settings", Long.toString(orgId),
                request.getRemoteAddr(), request.getHeader("User-Agent"));
        return ResponseEntity.noContent().build();
    }
}
