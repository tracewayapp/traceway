package com.example.pricing.engine;

import static org.junit.jupiter.api.Assertions.assertEquals;

import java.util.List;
import org.junit.jupiter.api.Test;

class PromotionRulesTest {
    @Test
    void percentOff() {
        var rules = new PromotionRules();
        var rule = new PromotionRules.Rule(1, "percent_off", 1, "sku:SH-1", "10%");
        assertEquals(200, rules.discount(List.of(rule), "SH-1", 2, 1000, List.of(new QuoteRequest.Line("SH-1", 2))));
    }
}
