package com.acme.insights.retention;

import java.sql.Timestamp;
import java.time.Instant;
import java.time.LocalDate;
import java.time.ZoneId;
import java.util.List;
import org.springframework.jdbc.core.JdbcTemplate;
import org.springframework.stereotype.Repository;

@Repository
public class EventRepository {
    private final JdbcTemplate jdbc;

    public EventRepository(JdbcTemplate jdbc) {
        this.jdbc = jdbc;
    }

    public record EventRow(long userId, String eventName, Instant occurredAt, String properties) {}

    public record Workspace(long id, String name, ZoneId timezone) {}

    public Workspace workspace(long id) {
        return jdbc.queryForObject("SELECT id, name, timezone FROM workspaces WHERE id = ?",
                (rs, n) -> new Workspace(rs.getLong(1), rs.getString(2), ZoneId.of(rs.getString(3))), id);
    }

    public List<EventRow> events(long workspaceId, LocalDate from, LocalDate to, ZoneId zone) {
        return jdbc.query(
                "SELECT user_id, event_name, occurred_at, properties FROM events WHERE workspace_id = ? AND occurred_at >= ? AND occurred_at < ? ORDER BY occurred_at",
                (rs, n) -> new EventRow(rs.getLong(1), rs.getString(2), rs.getTimestamp(3).toInstant(), rs.getString(4)),
                workspaceId,
                Timestamp.from(from.atStartOfDay(zone).toInstant()),
                Timestamp.from(to.atStartOfDay(zone).toInstant()));
    }
}
