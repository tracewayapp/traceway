package com.example.pricing.api;

import org.springframework.http.ResponseEntity;
import org.springframework.jdbc.core.simple.JdbcClient;
import org.springframework.web.bind.annotation.GetMapping;
import org.springframework.web.bind.annotation.PathVariable;
import org.springframework.web.bind.annotation.RequestMapping;
import org.springframework.web.bind.annotation.RequestParam;
import org.springframework.web.bind.annotation.RestController;

@RestController
@RequestMapping("/api/prices")
public class PriceController {
    private final JdbcClient jdbc;

    public PriceController(JdbcClient jdbc) {
        this.jdbc = jdbc;
    }

    public record ListPrice(String sku, long listPriceMinor, String currency) {}

    @GetMapping("/{sku}")
    public ResponseEntity<ListPrice> price(@PathVariable String sku, @RequestParam(defaultValue = "1") long priceList) {
        return jdbc.sql("SELECT sku, list_price_minor, currency FROM price_list_items WHERE price_list_id = ? AND sku = ?")
            .param(priceList)
            .param(sku)
            .query((rs, i) -> new ListPrice(rs.getString(1), rs.getLong(2), rs.getString(3)))
            .optional()
            .map(ResponseEntity::ok)
            .orElseGet(() -> ResponseEntity.notFound().build());
    }
}
