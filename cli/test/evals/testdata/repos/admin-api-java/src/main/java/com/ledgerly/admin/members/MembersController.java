package com.ledgerly.admin.members;

import java.time.OffsetDateTime;
import java.util.List;

import org.springframework.jdbc.core.JdbcTemplate;
import org.springframework.web.bind.annotation.GetMapping;
import org.springframework.web.bind.annotation.RequestMapping;
import org.springframework.web.bind.annotation.RestController;

import com.ledgerly.admin.security.AdminContext;

@RestController
@RequestMapping("/api/admin/members")
public class MembersController {

    private final JdbcTemplate jdbc;
    private final AdminContext admin;

    public MembersController(JdbcTemplate jdbc, AdminContext admin) {
        this.jdbc = jdbc;
        this.admin = admin;
    }

    public record Member(long id, String email, String fullName, String role, OffsetDateTime joinedAt) {
    }

    @GetMapping
    public List<Member> list() {
        long orgId = admin.currentOrgId();
        return jdbc.query(
                "SELECT u.id, u.email, u.full_name, m.role, m.joined_at FROM org_members m JOIN users u ON u.id = m.user_id WHERE m.org_id = ? ORDER BY u.full_name",
                (rs, i) -> new Member(rs.getLong(1), rs.getString(2), rs.getString(3), rs.getString(4),
                        rs.getObject(5, OffsetDateTime.class)),
                orgId);
    }
}
