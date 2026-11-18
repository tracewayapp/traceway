package com.ledgerly.admin.invoices;

import java.time.OffsetDateTime;
import java.util.List;

import org.springframework.jdbc.core.JdbcTemplate;
import org.springframework.web.bind.annotation.GetMapping;
import org.springframework.web.bind.annotation.RequestMapping;
import org.springframework.web.bind.annotation.RestController;

import com.ledgerly.admin.security.AdminContext;

@RestController
@RequestMapping("/api/admin/invoices")
public class InvoicesController {

    private final JdbcTemplate jdbc;
    private final AdminContext admin;

    public InvoicesController(JdbcTemplate jdbc, AdminContext admin) {
        this.jdbc = jdbc;
        this.admin = admin;
    }

    public record Invoice(long id, String number, String status, long totalCents, OffsetDateTime issuedAt) {
    }

    /** The latest 50 invoices of the organization. */
    @GetMapping
    public List<Invoice> list() {
        long orgId = admin.currentOrgId();
        return jdbc.query(
                "SELECT i.id, i.number, i.status, i.total_cents, i.issued_at FROM invoices i WHERE i.org_id = ? ORDER BY i.issued_at DESC LIMIT 50",
                (rs, i) -> new Invoice(rs.getLong(1), rs.getString(2), rs.getString(3), rs.getLong(4),
                        rs.getObject(5, OffsetDateTime.class)),
                orgId);
    }
}
