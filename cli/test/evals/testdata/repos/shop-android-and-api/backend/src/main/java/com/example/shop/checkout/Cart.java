package com.example.shop.checkout;

import java.util.List;

/** A cart and its lines. The shop sells in euros only. */
public record Cart(String id, List<Item> items) {

    public record Item(String sku, int qty, long unitPriceCents) {}

    public long totalCents() {
        return items.stream().mapToLong(i -> i.qty() * i.unitPriceCents()).sum();
    }

    public String currency() {
        return "EUR";
    }
}
