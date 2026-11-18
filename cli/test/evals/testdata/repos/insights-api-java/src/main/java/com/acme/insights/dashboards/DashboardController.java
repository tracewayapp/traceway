package com.acme.insights.dashboards;

import java.time.Instant;
import java.util.List;
import org.springframework.http.ResponseEntity;
import org.springframework.jdbc.core.JdbcTemplate;
import org.springframework.web.bind.annotation.GetMapping;
import org.springframework.web.bind.annotation.PathVariable;
import org.springframework.web.bind.annotation.RequestParam;
import org.springframework.web.bind.annotation.RestController;

@RestController
public class DashboardController {
    private final JdbcTemplate jdbc;

    public DashboardController(JdbcTemplate jdbc) {
        this.jdbc = jdbc;
    }

    public record Card(long id, String kind, String query, int position) {}

    public record Dashboard(long id, String name, String layout, Instant updatedAt, List<Card> cards) {}

    @GetMapping("/api/insights/dashboards/{id}")
    public ResponseEntity<Dashboard> dashboard(@PathVariable long id, @RequestParam long workspace) {
        List<Dashboard> found = jdbc.query(
                "SELECT id, name, layout, updated_at FROM dashboards WHERE id = ? AND workspace_id = ?",
                (rs, n) -> new Dashboard(rs.getLong(1), rs.getString(2), rs.getString(3), rs.getTimestamp(4).toInstant(), List.of()),
                id, workspace);
        if (found.isEmpty()) {
            return ResponseEntity.notFound().build();
        }
        List<Card> cards = jdbc.query(
                "SELECT id, kind, query, position FROM dashboard_cards WHERE dashboard_id = ? ORDER BY position",
                (rs, n) -> new Card(rs.getLong(1), rs.getString(2), rs.getString(3), rs.getInt(4)),
                id);
        Dashboard d = found.get(0);
        return ResponseEntity.ok(new Dashboard(d.id(), d.name(), d.layout(), d.updatedAt(), cards));
    }
}
