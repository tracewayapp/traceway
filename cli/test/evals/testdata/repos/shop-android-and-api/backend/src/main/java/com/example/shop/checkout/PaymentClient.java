package com.example.shop.checkout;

import java.io.IOException;
import java.io.UncheckedIOException;
import java.net.URI;
import java.net.http.HttpClient;
import java.net.http.HttpRequest;
import java.net.http.HttpResponse;
import java.time.Duration;
import org.springframework.beans.factory.annotation.Value;
import org.springframework.stereotype.Component;

/** The payment provider's REST API. */
@Component
public class PaymentClient {

    public record Charge(String reference) {}

    private final HttpClient http = HttpClient.newBuilder().connectTimeout(Duration.ofSeconds(3)).build();
    private final String apiKey;

    public PaymentClient(@Value("${payments.api-key}") String apiKey) {
        this.apiKey = apiKey;
    }

    public Charge charge(String paymentMethodId, long amountCents, String currency) {
        var body = "{\"payment_method\":\"%s\",\"amount\":%d,\"currency\":\"%s\"}".formatted(paymentMethodId, amountCents, currency.toLowerCase());
        var request = HttpRequest.newBuilder(URI.create("https://api.payments.example/v1/charges"))
                .timeout(Duration.ofSeconds(10))
                .header("Authorization", "Bearer " + apiKey)
                .header("Content-Type", "application/json")
                .POST(HttpRequest.BodyPublishers.ofString(body))
                .build();
        try {
            var response = http.send(request, HttpResponse.BodyHandlers.ofString());
            if (response.statusCode() != 201) {
                throw new IllegalStateException("charge failed: " + response.statusCode());
            }
            return new Charge(response.headers().firstValue("Charge-Id").orElse(""));
        } catch (IOException e) {
            throw new UncheckedIOException(e);
        } catch (InterruptedException e) {
            Thread.currentThread().interrupt();
            throw new IllegalStateException(e);
        }
    }
}
