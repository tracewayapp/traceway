package com.example.pricing.engine;

import java.util.Comparator;
import java.util.List;
import org.springframework.stereotype.Component;

/**
 * Promotion rules engine: rules are evaluated in priority order; stackable
 * rules add up, the first exclusive rule that matches stops evaluation.
 */
@Component
public class PromotionRules {
    public record Rule(long id, String kind, int priority, String conditions, String effect) {
        boolean exclusive() {
            return conditions.contains("exclusive");
        }

        boolean matches(String sku, List<QuoteRequest.Line> cart) {
            if (conditions.contains("cart_min_lines:")) {
                int min = Integer.parseInt(conditions.replaceAll(".*cart_min_lines:(\\d+).*", "$1"));
                if (cart.size() < min) {
                    return false;
                }
            }
            return conditions.contains("sku:" + sku) || conditions.contains("sku:*");
        }
    }

    public long discount(List<Rule> rules, String sku, int qty, long unitMinor, List<QuoteRequest.Line> cart) {
        long total = 0;
        for (Rule r : rules.stream().sorted(Comparator.comparingInt(Rule::priority)).toList()) {
            if (!r.matches(sku, cart)) {
                continue;
            }
            long d = switch (r.kind()) {
                case "percent_off" -> unitMinor * qty * Long.parseLong(r.effect().replace("%", "")) / 100;
                case "amount_off" -> Math.min(unitMinor * qty, Long.parseLong(r.effect()));
                case "bundle" -> cart.size() >= 2 ? Long.parseLong(r.effect()) : 0;
                default -> 0;
            };
            total += d;
            if (d > 0 && r.exclusive()) {
                break;
            }
        }
        return Math.min(total, unitMinor * qty);
    }
}
