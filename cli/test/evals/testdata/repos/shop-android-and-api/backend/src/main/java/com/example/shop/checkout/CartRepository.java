package com.example.shop.checkout;

import java.util.Optional;
import org.springframework.jdbc.core.JdbcTemplate;
import org.springframework.stereotype.Repository;

@Repository
public class CartRepository {

    private final JdbcTemplate jdbc;

    public CartRepository(JdbcTemplate jdbc) {
        this.jdbc = jdbc;
    }

    /** The cart with its lines, or empty when the cart has none. */
    public Optional<Cart> findWithItems(String cartId) {
        var items = jdbc.query(
                "select c.id, ci.sku, ci.qty, ci.unit_price_cents from carts c join cart_items ci on ci.cart_id=c.id where c.id=?",
                (rs, n) -> new Cart.Item(rs.getString("sku"), rs.getInt("qty"), rs.getLong("unit_price_cents")),
                cartId);
        return items.isEmpty() ? Optional.empty() : Optional.of(new Cart(cartId, items));
    }

    public void addItem(String cartId, String sku, int qty, long unitPriceCents) {
        jdbc.update(
                "insert into cart_items (cart_id, sku, qty, unit_price_cents) values (?, ?, ?, ?) on conflict (cart_id, sku) do update set qty=cart_items.qty+excluded.qty",
                cartId, sku, qty, unitPriceCents);
    }
}
