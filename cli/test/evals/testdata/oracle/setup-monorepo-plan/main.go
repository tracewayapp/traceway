package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	shutdown, err := setupTelemetry(ctx)
	if err != nil {
		log.Fatalf("telemetry: %v", err)
	}
	defer func() {
		c, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = shutdown(c)
	}()

	mux := http.NewServeMux()
	// Wrap each handler, not the mux: otelhttp reads http.route from
	// r.Pattern, which ServeMux sets only after it matched.
	handle := func(pattern string, h http.HandlerFunc, name string) {
		mux.Handle(pattern, otelhttp.NewHandler(h, name))
	}
	handle("GET /api/products", listProducts, "list-products")
	handle("GET /api/products/{id}", getProduct, "get-product")
	handle("POST /api/cart/items", addCartItem, "add-cart-item")
	addr := ":" + envOr("PORT", "8080")
	srv := &http.Server{Addr: addr, Handler: mux}
	go func() {
		<-ctx.Done()
		_ = srv.Shutdown(context.Background())
	}()
	log.Printf("shop api listening on %s", addr)
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatal(err)
	}
}

func envOr(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}
