package com.example.pricing.engine;

import java.util.List;
import org.springframework.stereotype.Component;

@Component
public class PromotionRules {
    public record Rule(long id, String kind, int priority, String conditions, String effect) {}

    /** Percentage-off and bundle rules, best single rule wins. */
    public long discount(List<Rule> rules, String sku, int qty, long unitMinor, List<QuoteRequest.Line> cart) {
        long best = 0;
        for (Rule r : rules) {
            if (!r.conditions().contains(sku)) {
                continue;
            }
            if ("percent_off".equals(r.kind())) {
                long pct = Long.parseLong(r.effect().replace("%", ""));
                best = Math.max(best, unitMinor * qty * pct / 100);
            } else if ("bundle".equals(r.kind()) && cart.size() >= 2) {
                best = Math.max(best, Long.parseLong(r.effect()));
            }
        }
        return best;
    }
}
