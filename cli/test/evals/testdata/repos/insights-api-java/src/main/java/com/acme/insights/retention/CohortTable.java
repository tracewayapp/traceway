package com.acme.insights.retention;

import java.time.LocalDate;
import java.util.ArrayList;
import java.util.List;
import java.util.Map;
import java.util.Set;
import java.util.TreeMap;

public record CohortTable(Interval interval, List<Row> rows) {
    public record Row(LocalDate cohort, int users, List<Double> retained) {}

    static CohortTable from(TreeMap<LocalDate, Set<Long>> cohorts,
                            Map<LocalDate, Map<LocalDate, Set<Long>>> active, Interval interval) {
        List<Row> rows = new ArrayList<>();
        for (Map.Entry<LocalDate, Set<Long>> c : cohorts.entrySet()) {
            int size = c.getValue().size();
            List<Double> retained = new ArrayList<>();
            for (Map.Entry<LocalDate, Set<Long>> p : new TreeMap<>(active.getOrDefault(c.getKey(), Map.of())).entrySet()) {
                retained.add(size == 0 ? 0.0 : (double) p.getValue().size() / size);
            }
            rows.add(new Row(c.getKey(), size, retained));
        }
        return new CohortTable(interval, rows);
    }
}
