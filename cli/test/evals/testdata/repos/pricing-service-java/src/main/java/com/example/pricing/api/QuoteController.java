package com.example.pricing.api;

import com.example.pricing.engine.PricingEngine;
import com.example.pricing.engine.Quote;
import com.example.pricing.engine.QuoteRequest;
import org.springframework.web.bind.annotation.PostMapping;
import org.springframework.web.bind.annotation.RequestBody;
import org.springframework.web.bind.annotation.RequestMapping;
import org.springframework.web.bind.annotation.RestController;

@RestController
@RequestMapping("/api/prices")
public class QuoteController {
    private final PricingEngine engine;

    public QuoteController(PricingEngine engine) {
        this.engine = engine;
    }

    @PostMapping("/quote")
    public Quote quote(@RequestBody QuoteRequest request) {
        return engine.quote(request);
    }
}
