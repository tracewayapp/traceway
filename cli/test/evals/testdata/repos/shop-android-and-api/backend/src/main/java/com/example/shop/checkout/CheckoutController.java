package com.example.shop.checkout;

import org.springframework.http.ResponseEntity;
import org.springframework.web.bind.annotation.PostMapping;
import org.springframework.web.bind.annotation.RequestBody;
import org.springframework.web.bind.annotation.RequestMapping;
import org.springframework.web.bind.annotation.RestController;

@RestController
@RequestMapping("/api")
public class CheckoutController {

    public record CheckoutRequest(String cartId, String paymentMethodId) {}

    public record OrderResponse(String id, String status, long totalCents) {}

    private final CheckoutService checkoutService;

    public CheckoutController(CheckoutService checkoutService) {
        this.checkoutService = checkoutService;
    }

    /** POST /api/checkout: reserve stock, charge, create the order. */
    @PostMapping("/checkout")
    public ResponseEntity<OrderResponse> checkout(@RequestBody CheckoutRequest req) {
        // The authenticated customer comes from the security context.
        var order = checkoutService.placeOrder(req.cartId(), req.paymentMethodId());
        return ResponseEntity.ok(new OrderResponse(order.id(), order.status(), order.totalCents()));
    }
}
