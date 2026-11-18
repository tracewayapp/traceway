package com.example.pricing.engine;

import java.util.List;

public record QuoteRequest(long priceListId, String currency, List<Line> lines) {
    public record Line(String sku, int qty) {}
}
