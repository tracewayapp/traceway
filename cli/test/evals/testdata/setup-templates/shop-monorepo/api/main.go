package main

import (
	"log"
	"net/http"
	"os"
)

func main() {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/products", listProducts)
	mux.HandleFunc("GET /api/products/{id}", getProduct)
	mux.HandleFunc("POST /api/cart/items", addCartItem)
	addr := ":" + envOr("PORT", "8080")
	log.Printf("shop api listening on %s", addr)
	log.Fatal(http.ListenAndServe(addr, mux))
}

func envOr(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}
