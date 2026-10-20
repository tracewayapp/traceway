package com.ledgerly.admin.audit;

import java.time.OffsetDateTime;
import java.util.List;

import org.springframework.jdbc.core.JdbcTemplate;
import org.springframework.jdbc.core.RowMapper;
import org.springframework.stereotype.Repository;

@Repository
public class AuditEventRepository {

    private static final RowMapper<AuditEvent> FULL = (rs, i) -> new AuditEvent(
            rs.getLong("id"),
            rs.getLong("actor_id"),
            rs.getString("action"),
            rs.getString("target_type"),
            rs.getString("target_id"),
            rs.getString("ip_address"),
            rs.getString("user_agent"),
            rs.getObject("created_at", OffsetDateTime.class));

    private static final RowMapper<AuditEvent> SUMMARY = (rs, i) -> new AuditEvent(
            rs.getLong("id"),
            rs.getLong("actor_id"),
            rs.getString("action"),
            null, null, null, null,
            rs.getObject("created_at", OffsetDateTime.class));

    private final JdbcTemplate jdbc;

    public AuditEventRepository(JdbcTemplate jdbc) {
        this.jdbc = jdbc;
    }

    /** Every event of the organization, newest first (the Audit log page). */
    public List<AuditEvent> findByOrg(long orgId) {
        return jdbc.query(
                "SELECT e.id, e.actor_id, e.action, e.target_type, e.target_id, e.ip_address, e.user_agent, e.created_at "
                        + "FROM audit_events e WHERE e.org_id = ? ORDER BY e.created_at DESC",
                FULL, orgId);
    }

    /** The latest events for the dashboard widget. */
    public List<AuditEvent> findRecent(long orgId) {
        return jdbc.query(
                "SELECT e.id, e.actor_id, e.action, e.created_at "
                        + "FROM audit_events e WHERE e.org_id = ? ORDER BY e.created_at DESC LIMIT 20",
                SUMMARY, orgId);
    }

    public void record(long orgId, long actorId, String action, String targetType, String targetId, String ip, String userAgent) {
        jdbc.update(
                "INSERT INTO audit_events (org_id, actor_id, action, target_type, target_id, ip_address, user_agent, created_at) "
                        + "VALUES (?, ?, ?, ?, ?, ?, ?, now())",
                orgId, actorId, action, targetType, targetId, ip, userAgent);
    }
}
