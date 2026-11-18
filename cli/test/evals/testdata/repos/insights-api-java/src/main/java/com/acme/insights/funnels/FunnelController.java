package com.acme.insights.funnels;

import java.sql.Date;
import java.time.LocalDate;
import java.util.List;
import org.springframework.format.annotation.DateTimeFormat;
import org.springframework.jdbc.core.JdbcTemplate;
import org.springframework.web.bind.annotation.GetMapping;
import org.springframework.web.bind.annotation.RequestParam;
import org.springframework.web.bind.annotation.RestController;

@RestController
public class FunnelController {
    private final JdbcTemplate jdbc;

    public FunnelController(JdbcTemplate jdbc) {
        this.jdbc = jdbc;
    }

    public record Step(int step, long users) {}

    /** Distinct users who reached each step of the funnel since `from`. */
    @GetMapping("/api/insights/funnels")
    public List<Step> funnel(
            @RequestParam long workspace,
            @RequestParam long funnel,
            @RequestParam @DateTimeFormat(iso = DateTimeFormat.ISO.DATE) LocalDate from) {
        return jdbc.query(
                "SELECT step, count(DISTINCT user_id) FROM funnel_steps WHERE workspace_id = ? AND funnel_id = ? AND day >= ? GROUP BY step ORDER BY step",
                (rs, n) -> new Step(rs.getInt(1), rs.getLong(2)),
                workspace, funnel, Date.valueOf(from));
    }
}
