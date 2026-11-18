package com.example.shop.cart;

import com.example.shop.checkout.Cart;
import com.example.shop.checkout.CartRepository;
import org.springframework.http.HttpStatus;
import org.springframework.http.ResponseEntity;
import org.springframework.jdbc.core.JdbcTemplate;
import org.springframework.web.bind.annotation.GetMapping;
import org.springframework.web.bind.annotation.PostMapping;
import org.springframework.web.bind.annotation.RequestBody;
import org.springframework.web.bind.annotation.RequestHeader;
import org.springframework.web.bind.annotation.RequestMapping;
import org.springframework.web.bind.annotation.RestController;

@RestController
@RequestMapping("/api/cart")
public class CartController {

    public record AddItem(String sku, int qty, long unitPriceCents) {}

    private final CartRepository carts;
    private final JdbcTemplate jdbc;

    public CartController(CartRepository carts, JdbcTemplate jdbc) {
        this.carts = carts;
        this.jdbc = jdbc;
    }

    /** GET /api/cart: the customer's cart. */
    @GetMapping
    public ResponseEntity<Cart> cart(@RequestHeader("X-Cart-Id") String cartId) {
        return ResponseEntity.of(carts.findWithItems(cartId));
    }

    /** POST /api/cart/items: add a line, if the SKU has stock left. */
    @PostMapping("/items")
    public ResponseEntity<Void> addItem(@RequestHeader("X-Cart-Id") String cartId, @RequestBody AddItem req) {
        Integer free = jdbc.query("select available, reserved from inventory where sku=?",
                rs -> rs.next() ? rs.getInt("available") - rs.getInt("reserved") : null, req.sku());
        if (free == null || free < req.qty()) {
            return ResponseEntity.status(HttpStatus.CONFLICT).build();
        }
        carts.addItem(cartId, req.sku(), req.qty(), req.unitPriceCents());
        return ResponseEntity.ok().build();
    }
}
