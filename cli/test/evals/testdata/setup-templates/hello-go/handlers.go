package main

import (
	"encoding/json"
	"net/http"
	"strings"
)

type greeting struct {
	Message string `json:"message"`
}

func hello(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, greeting{Message: "Hello, world!"})
}

func helloName(w http.ResponseWriter, r *http.Request) {
	name := strings.TrimSpace(r.PathValue("name"))
	if name == "" {
		http.Error(w, "name is required", http.StatusBadRequest)
		return
	}
	writeJSON(w, greeting{Message: "Hello, " + name + "!"})
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}
