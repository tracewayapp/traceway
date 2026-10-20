package com.example.pricing.engine;

import java.util.List;
import org.springframework.stereotype.Component;

@Component
public class PromotionRules {
    public record Rule(long id, String kind, int priority, String conditions, String effect) {}

    /** Percentage-off rules, best single rule wins. */
    public long discount(List<Rule> rules, String sku, int qty, long unitMinor, List<QuoteRequest.Line> cart) {
        long best = 0;
        for (Rule r : rules) {
            if (!"percent_off".equals(r.kind()) || !r.conditions().contains(sku)) {
                continue;
            }
            long pct = Long.parseLong(r.effect().replace("%", ""));
            best = Math.max(best, unitMinor * qty * pct / 100);
        }
        return best;
    }
}
