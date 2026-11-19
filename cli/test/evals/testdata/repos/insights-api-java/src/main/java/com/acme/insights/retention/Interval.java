package com.acme.insights.retention;

import java.time.DayOfWeek;
import java.time.LocalDate;
import java.time.temporal.TemporalAdjusters;

public enum Interval {
    DAY, WEEK;

    public static Interval parse(String s) {
        return "day".equalsIgnoreCase(s) ? DAY : WEEK;
    }

    public LocalDate bucket(LocalDate d) {
        return this == DAY ? d : d.with(TemporalAdjusters.previousOrSame(DayOfWeek.MONDAY));
    }
}
