package com.example.shop.checkout;

import org.springframework.http.HttpStatus;
import org.springframework.web.bind.annotation.ResponseStatus;

@ResponseStatus(HttpStatus.NOT_FOUND)
class CartNotFoundException extends RuntimeException {
    CartNotFoundException(String cartId) {
        super("cart " + cartId + " not found");
    }
}

@ResponseStatus(HttpStatus.UNPROCESSABLE_ENTITY)
class EmptyCartException extends RuntimeException {
    EmptyCartException(String cartId) {
        super("cart " + cartId + " is empty");
    }
}

@ResponseStatus(HttpStatus.CONFLICT)
class OutOfStockException extends RuntimeException {
    OutOfStockException(String sku) {
        super(sku + " is out of stock");
    }
}
