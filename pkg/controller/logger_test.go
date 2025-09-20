package controller_test

import (
	"net/http"
	"net/http/httptest"
	"scanner/pkg/controller"
	"testing"

	"scanner/pkg/logger"
)

func TestGetClientIP_XForwardedFor(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-Forwarded-For", "1.2.3.4, 5.6.7.8")
	if ip := controller.GetClientIP(req); ip != "1.2.3.4" {
		t.Fatalf("expected 1.2.3.4, got %q", ip)
	}
}

func TestGetClientIP_XRealIP(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-Real-IP", "9.8.7.6")
	if ip := controller.GetClientIP(req); ip != "9.8.7.6" {
		t.Fatalf("expected 9.8.7.6, got %q", ip)
	}
}

func TestGetClientIP_RemoteAddr(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "10.0.0.1:12345"
	if ip := controller.GetClientIP(req); ip != "10.0.0.1" {
		t.Fatalf("expected 10.0.0.1, got %q", ip)
	}
}

func TestGetClientIP_InvalidRemoteAddr(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "not-an-addr"
	if ip := controller.GetClientIP(req); ip != "not-an-addr" {
		t.Fatalf("expected passthrough of invalid RemoteAddr, got %q", ip)
	}
}

func TestWithLogger_SetsRequestIDAndPassesStatus(t *testing.T) {
	// initialize default logger to avoid nil pointer in middleware
	logger.Setup(logger.DevelopmentEnvironment)
	// Handler echoes request ID from context into a header so we can assert it.
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		val := r.Context().Value(controller.RequestIDKey)
		if s, _ := val.(string); s != "" {
			w.Header().Set("X-Echo-Request-Id", s)
		}
		w.WriteHeader(http.StatusCreated)
	})

	// Case 1: request provides X-Request-Id header
	req1 := httptest.NewRequest(http.MethodGet, "/", nil)
	req1.Header.Set("X-Request-Id", "abc-123")
	rec1 := httptest.NewRecorder()
	controller.WithLogger(next).ServeHTTP(rec1, req1)
	res1 := rec1.Result()
	if res1.StatusCode != http.StatusCreated {
		t.Fatalf("expected status %d, got %d", http.StatusCreated, res1.StatusCode)
	}
	if got := res1.Header.Get("X-Echo-Request-Id"); got != "abc-123" {
		t.Fatalf("expected echoed request id \"abc-123\", got %q", got)
	}

	// Case 2: request without header should still receive a generated ID
	req2 := httptest.NewRequest(http.MethodGet, "/", nil)
	rec2 := httptest.NewRecorder()
	controller.WithLogger(next).ServeHTTP(rec2, req2)
	res2 := rec2.Result()
	if res2.StatusCode != http.StatusCreated {
		t.Fatalf("expected status %d, got %d", http.StatusCreated, res2.StatusCode)
	}
	if got := res2.Header.Get("X-Echo-Request-Id"); got == "" {
		t.Fatalf("expected a generated request id to be present")
	}
}
