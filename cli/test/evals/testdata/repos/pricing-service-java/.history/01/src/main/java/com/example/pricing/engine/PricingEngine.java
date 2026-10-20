package com.example.pricing.engine;

import io.opentelemetry.instrumentation.annotations.WithSpan;
import java.time.Instant;
import java.util.ArrayList;
import java.util.List;
import java.util.Map;
import org.springframework.jdbc.core.simple.JdbcClient;
import org.springframework.stereotype.Service;

@Service
public class PricingEngine {
    private final JdbcClient jdbc;
    private final PromotionRules rules;

    public PricingEngine(JdbcClient jdbc, PromotionRules rules) {
        this.jdbc = jdbc;
        this.rules = rules;
    }

    @WithSpan("PricingEngine.quote")
    public Quote quote(QuoteRequest req) {
        List<String> skus = req.lines().stream().map(QuoteRequest.Line::sku).toList();
        Map<String, Long> prices = listPrices(req.priceListId(), skus);
        List<PromotionRules.Rule> active = jdbc.sql(
                "SELECT id, kind, priority, conditions, effect FROM promotion_rules WHERE active AND ? BETWEEN starts_at AND ends_at ORDER BY priority")
            .param(Instant.now())
            .query(PromotionRules.Rule.class)
            .list();
        List<Quote.Line> lines = new ArrayList<>();
        long total = 0;
        for (QuoteRequest.Line l : req.lines()) {
            long unit = prices.getOrDefault(l.sku(), 0L);
            long discount = rules.discount(active, l.sku(), l.qty(), unit, req.lines());
            lines.add(new Quote.Line(l.sku(), l.qty(), unit, discount));
            total += unit * l.qty() - discount;
        }
        return new Quote(req.currency(), lines, total);
    }

    private Map<String, Long> listPrices(long priceListId, List<String> skus) {
        return jdbc.sql("SELECT sku, list_price_minor FROM price_list_items WHERE price_list_id = :list AND sku IN (:skus)")
            .param("list", priceListId)
            .param("skus", skus)
            .query((rs, i) -> Map.entry(rs.getString(1), rs.getLong(2)))
            .list().stream()
            .collect(java.util.stream.Collectors.toMap(Map.Entry::getKey, Map.Entry::getValue));
    }
}
