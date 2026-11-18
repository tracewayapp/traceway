package com.ledgerly.admin.audit;

import java.util.List;

import org.springframework.web.bind.annotation.GetMapping;
import org.springframework.web.bind.annotation.RequestMapping;
import org.springframework.web.bind.annotation.RestController;

import com.ledgerly.admin.security.AdminContext;

@RestController
@RequestMapping("/api/admin")
public class AuditLogController {

    private final AuditEventRepository events;
    private final AdminContext admin;

    public AuditLogController(AuditEventRepository events, AdminContext admin) {
        this.events = events;
        this.admin = admin;
    }

    /** The Audit log page. The UI renders the list and filters it client side. */
    @GetMapping("/audit-log")
    public List<AuditEvent> list() {
        long orgId = admin.requireAdmin();
        return events.findByOrg(orgId);
    }

    /** The dashboard widget: the last 20 events. */
    @GetMapping("/dashboard/activity")
    public List<AuditEvent> activity() {
        long orgId = admin.requireAdmin();
        return events.findRecent(orgId);
    }
}
