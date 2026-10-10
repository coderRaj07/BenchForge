package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestOrdersAndHealth(t *testing.T) {
	h := newHandler(1)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/healthz", nil))
	if rec.Code != http.StatusOK {
		t.Fatal(rec.Code)
	}
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/orders?customer_id=7", nil))
	if rec.Code != http.StatusOK || rec.Body.Len() == 0 {
		t.Fatalf("%d %s", rec.Code, rec.Body.String())
	}
}
