package controller_test

import (
	"net/http"
	"net/http/httptest"
	"scanner/pkg/controller"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestWithCORS_Preflight(t *testing.T) {
	called := false
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
	})

	req := httptest.NewRequest(http.MethodOptions, "/anything", nil)
	rec := httptest.NewRecorder()

	controller.WithCORS(next).ServeHTTP(rec, req)

	require.False(t, called, "next handler should not be called for OPTIONS preflight")
	res := rec.Result()
	require.Equal(t, http.StatusNoContent, res.StatusCode)

	// headers should be present
	require.Equal(t, "*", res.Header.Get("Access-Control-Allow-Origin"))
	require.Equal(t, "true", res.Header.Get("Access-Control-Allow-Credentials"))
	require.NotEmpty(t, res.Header.Get("Access-Control-Allow-Headers"))
	require.NotEmpty(t, res.Header.Get("Access-Control-Allow-Methods"))
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

	require.True(t, called, "next handler should be called for non-OPTIONS request")
	res := rec.Result()
	require.Equal(t, http.StatusTeapot, res.StatusCode)

	// headers should be present
	require.Equal(t, "*", res.Header.Get("Access-Control-Allow-Origin"))
	require.Equal(t, "true", res.Header.Get("Access-Control-Allow-Credentials"))
	require.NotEmpty(t, res.Header.Get("Access-Control-Allow-Headers"))
	require.NotEmpty(t, res.Header.Get("Access-Control-Allow-Methods"))
}
