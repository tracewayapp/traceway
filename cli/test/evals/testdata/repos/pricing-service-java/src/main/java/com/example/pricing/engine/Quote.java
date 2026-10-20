package com.example.pricing.engine;

import java.util.List;

public record Quote(String currency, List<Line> lines, long totalMinor) {
    public record Line(String sku, int qty, long unitMinor, long discountMinor) {}
}
