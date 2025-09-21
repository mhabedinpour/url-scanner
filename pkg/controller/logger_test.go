package controller_test

import (
	"net/http"
	"net/http/httptest"
	"scanner/pkg/controller"
	"testing"

	"scanner/pkg/logger"

	"github.com/stretchr/testify/require"
)

func TestGetClientIP_XForwardedFor(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-Forwarded-For", "1.2.3.4, 5.6.7.8")
	require.Equal(t, "1.2.3.4", controller.GetClientIP(req))
}

func TestGetClientIP_XRealIP(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-Real-IP", "9.8.7.6")
	require.Equal(t, "9.8.7.6", controller.GetClientIP(req))
}

func TestGetClientIP_RemoteAddr(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "10.0.0.1:12345"
	require.Equal(t, "10.0.0.1", controller.GetClientIP(req))
}

func TestGetClientIP_InvalidRemoteAddr(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "not-an-addr"
	require.Equal(t, "not-an-addr", controller.GetClientIP(req))
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
	require.Equal(t, http.StatusCreated, res1.StatusCode)
	require.Equal(t, "abc-123", res1.Header.Get("X-Echo-Request-Id"))

	// Case 2: request without header should still receive a generated ID
	req2 := httptest.NewRequest(http.MethodGet, "/", nil)
	rec2 := httptest.NewRecorder()
	controller.WithLogger(next).ServeHTTP(rec2, req2)
	res2 := rec2.Result()
	require.Equal(t, http.StatusCreated, res2.StatusCode)
	require.NotEmpty(t, res2.Header.Get("X-Echo-Request-Id"))
}
