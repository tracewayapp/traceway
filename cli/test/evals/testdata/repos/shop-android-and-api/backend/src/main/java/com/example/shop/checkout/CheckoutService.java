package com.example.shop.checkout;

import java.util.UUID;
import org.springframework.stereotype.Service;
import org.springframework.transaction.annotation.Transactional;

@Service
public class CheckoutService {

    public record Order(String id, String status, long totalCents) {}

    private final CartRepository carts;
    private final InventoryRepository inventory;
    private final PaymentClient payments;
    private final OrderRepository orders;

    public CheckoutService(CartRepository carts, InventoryRepository inventory, PaymentClient payments, OrderRepository orders) {
        this.carts = carts;
        this.inventory = inventory;
        this.payments = payments;
        this.orders = orders;
    }

    /**
     * One transaction: reserve every line's stock, charge the card, write the
     * order. If the charge fails the reservation rolls back with it.
     */
    @Transactional
    public Order placeOrder(String cartId, String paymentMethodId) {
        var cart = carts.findWithItems(cartId)
                .orElseThrow(() -> new CartNotFoundException(cartId));
        if (cart.items().isEmpty()) {
            throw new EmptyCartException(cartId);
        }

        // Reserve in cart order: the UPDATE only matches while
        // available - reserved still covers the quantity.
        for (var item : cart.items()) {
            int updated = inventory.reserve(item.sku(), item.qty());
            if (updated == 0) {
                throw new OutOfStockException(item.sku());
            }
            inventory.flush();
        }

        var charge = payments.charge(paymentMethodId, cart.totalCents(), cart.currency());

        var orderId = UUID.randomUUID().toString();
        orders.insert(orderId, cart, charge.reference());
        return new Order(orderId, "placed", cart.totalCents());
    }
}
