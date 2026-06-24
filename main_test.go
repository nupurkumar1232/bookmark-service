package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHealthEndpoint(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rr := httptest.NewRecorder()

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("ok\n"))
	})

	handler.ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusOK {
		t.Errorf("expected status 418, got %d", status)
	}

	if body := rr.Body.String(); body != "ok\n" {
		t.Errorf("expected body 'ok\\n', got %q", body)
	}
}
