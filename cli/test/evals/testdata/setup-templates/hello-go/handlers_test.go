package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestHelloName(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/hello/ada", nil)
	req.SetPathValue("name", "ada")
	rec := httptest.NewRecorder()
	helloName(rec, req)
	if !strings.Contains(rec.Body.String(), "Hello, ada!") {
		t.Fatal(rec.Body.String())
	}
}
