package controller_test

import (
	"net/http"
	"net/http/httptest"
	"scanner/pkg/controller"
	"testing"
)

func TestWithCORS_Preflight(t *testing.T) {
	called := false
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
	})

	req := httptest.NewRequest(http.MethodOptions, "/anything", nil)
	rec := httptest.NewRecorder()

	controller.WithCORS(next).ServeHTTP(rec, req)

	if called {
		t.Fatalf("next handler should not be called for OPTIONS preflight")
	}
	res := rec.Result()
	if res.StatusCode != http.StatusNoContent {
		t.Fatalf("expected status %d, got %d", http.StatusNoContent, res.StatusCode)
	}

	// headers should be present
	if got := res.Header.Get("Access-Control-Allow-Origin"); got != "*" {
		t.Errorf("Access-Control-Allow-Origin = %q", got)
	}
	if got := res.Header.Get("Access-Control-Allow-Credentials"); got != "true" {
		t.Errorf("Access-Control-Allow-Credentials = %q", got)
	}
	if got := res.Header.Get("Access-Control-Allow-Headers"); got == "" {
		t.Errorf("Access-Control-Allow-Headers should not be empty")
	}
	if got := res.Header.Get("Access-Control-Allow-Methods"); got == "" {
		t.Errorf("Access-Control-Allow-Methods should not be empty")
	}
}

func TestWithCORS_NormalRequest(t *testing.T) {
	called := false
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusTeapot)
	})

	req := httptest.NewRequest(http.MethodGet, "/path", nil)
	rec := httptest.NewRecorder()

	controller.WithCORS(next).ServeHTTP(rec, req)

	if !called {
		t.Fatalf("next handler should be called for non-OPTIONS request")
	}
	res := rec.Result()
	if res.StatusCode != http.StatusTeapot {
		t.Fatalf("expected status %d, got %d", http.StatusTeapot, res.StatusCode)
	}

	// headers should be present
	if got := res.Header.Get("Access-Control-Allow-Origin"); got != "*" {
		t.Errorf("Access-Control-Allow-Origin = %q", got)
	}
	if got := res.Header.Get("Access-Control-Allow-Credentials"); got != "true" {
		t.Errorf("Access-Control-Allow-Credentials = %q", got)
	}
	if got := res.Header.Get("Access-Control-Allow-Headers"); got == "" {
		t.Errorf("Access-Control-Allow-Headers should not be empty")
	}
	if got := res.Header.Get("Access-Control-Allow-Methods"); got == "" {
		t.Errorf("Access-Control-Allow-Methods should not be empty")
	}
}
