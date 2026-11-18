package main

import (
	"encoding/json"
	"net/http"
)

type product struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	PriceCents int    `json:"priceCents"`
}

var catalog = []product{
	{ID: "1", Name: "Enamel mug", PriceCents: 1200},
	{ID: "2", Name: "Logo tee", PriceCents: 1950},
	{ID: "3", Name: "Dad cap", PriceCents: 1500},
}

func listProducts(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, catalog)
}

func getProduct(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	for _, p := range catalog {
		if p.ID == id {
			writeJSON(w, http.StatusOK, p)
			return
		}
	}
	writeJSON(w, http.StatusNotFound, map[string]string{"error": "product not found"})
}

type cartItem struct {
	ProductID string `json:"productId"`
	Qty       int    `json:"qty"`
}

func addCartItem(w http.ResponseWriter, r *http.Request) {
	var in cartItem
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil || in.ProductID == "" || in.Qty < 1 {
		writeJSON(w, http.StatusUnprocessableEntity, map[string]string{"error": "productId and a positive qty are required"})
		return
	}
	writeJSON(w, http.StatusCreated, in)
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
