package controller_test

import (
	"net/http"
	"net/http/httptest"
	"scanner/pkg/controller"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestPprofMux_Index(t *testing.T) {
	mux := controller.PprofMux()
	req := httptest.NewRequest(http.MethodGet, "http://pprof.local/", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	res := rec.Result()
	require.Equal(t, http.StatusOK, res.StatusCode)
	require.NotEmpty(t, res.Header.Get("Content-Type"))
}

func TestPprofMux_Cmdline_OK(t *testing.T) {
	mux := controller.PprofMux()
	req := httptest.NewRequest(http.MethodGet, "http://pprof.local/cmdline", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	res := rec.Result()
	require.Equal(t, http.StatusOK, res.StatusCode)
}
