package com.acme.insights.retention;

import io.opentelemetry.instrumentation.annotations.WithSpan;
import java.time.LocalDate;
import java.time.ZoneId;
import java.util.ArrayList;
import java.util.HashMap;
import java.util.HashSet;
import java.util.List;
import java.util.Map;
import java.util.Set;
import java.util.TreeMap;
import org.springframework.stereotype.Service;

@Service
public class RetentionService {
    private final EventRepository events;

    public RetentionService(EventRepository events) {
        this.events = events;
    }

    /**
     * Weekly (or daily) retention: users grouped by the period of their first
     * event, and for each later period the share of them who came back.
     */
    @WithSpan
    public CohortTable buildCohorts(long workspaceId, LocalDate from, LocalDate to, Interval interval) {
        EventRepository.Workspace ws = events.workspace(workspaceId);
        ZoneId zone = ws.timezone();
        List<EventRepository.EventRow> rows = events.events(workspaceId, from, to, zone);

        Map<Long, List<EventRepository.EventRow>> byUser = new HashMap<>();
        for (EventRepository.EventRow row : rows) {
            byUser.computeIfAbsent(row.userId(), k -> new ArrayList<>()).add(row);
        }

        TreeMap<LocalDate, Set<Long>> cohorts = new TreeMap<>();
        Map<LocalDate, Map<LocalDate, Set<Long>>> activeByCohort = new HashMap<>();
        for (Map.Entry<Long, List<EventRepository.EventRow>> e : byUser.entrySet()) {
            List<EventRepository.EventRow> userEvents = new ArrayList<>(e.getValue());
            userEvents.sort((a, b) -> a.occurredAt().compareTo(b.occurredAt()));
            LocalDate cohort = interval.bucket(userEvents.get(0).occurredAt().atZone(zone).toLocalDate());
            cohorts.computeIfAbsent(cohort, k -> new HashSet<>()).add(e.getKey());
            for (EventRepository.EventRow ev : userEvents) {
                LocalDate period = interval.bucket(ev.occurredAt().atZone(zone).toLocalDate());
                activeByCohort.computeIfAbsent(cohort, k -> new HashMap<>())
                        .computeIfAbsent(period, k -> new HashSet<>())
                        .add(e.getKey());
            }
        }
        return CohortTable.from(cohorts, activeByCohort, interval);
    }
}
