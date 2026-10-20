package com.acme.insights.retention;

import java.time.LocalDate;
import org.springframework.format.annotation.DateTimeFormat;
import org.springframework.http.ResponseEntity;
import org.springframework.web.bind.annotation.GetMapping;
import org.springframework.web.bind.annotation.RequestParam;
import org.springframework.web.bind.annotation.RestController;

@RestController
public class RetentionController {
    private final RetentionService retention;

    public RetentionController(RetentionService retention) {
        this.retention = retention;
    }

    @GetMapping("/api/insights/retention")
    public ResponseEntity<CohortTable> retention(
            @RequestParam long workspace,
            @RequestParam @DateTimeFormat(iso = DateTimeFormat.ISO.DATE) LocalDate from,
            @RequestParam @DateTimeFormat(iso = DateTimeFormat.ISO.DATE) LocalDate to,
            @RequestParam(defaultValue = "week") String interval) {
        if (!to.isAfter(from)) {
            return ResponseEntity.unprocessableEntity().build();
        }
        return ResponseEntity.ok(retention.buildCohorts(workspace, from, to, Interval.parse(interval)));
    }
}
