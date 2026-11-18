package com.example.shop.checkout;

import java.sql.Timestamp;
import java.time.Instant;
import org.springframework.jdbc.core.JdbcTemplate;
import org.springframework.stereotype.Repository;

@Repository
public class OrderRepository {

    private final JdbcTemplate jdbc;

    public OrderRepository(JdbcTemplate jdbc) {
        this.jdbc = jdbc;
    }

    public void insert(String orderId, Cart cart, String paymentRef) {
        jdbc.update(
                "insert into orders (id, cart_id, total_cents, currency, payment_ref, status, created_at) values (?, ?, ?, ?, ?, ?, ?)",
                orderId, cart.id(), cart.totalCents(), cart.currency(), paymentRef, "placed", Timestamp.from(Instant.now()));
        jdbc.batchUpdate(
                "insert into order_lines (order_id, sku, qty, unit_price_cents) values (?, ?, ?, ?)",
                cart.items().stream().map(i -> new Object[] {orderId, i.sku(), i.qty(), i.unitPriceCents()}).toList());
    }
}
