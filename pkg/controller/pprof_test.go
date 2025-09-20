package controller_test

import (
	"net/http"
	"net/http/httptest"
	"scanner/pkg/controller"
	"testing"
)

func TestPprofMux_Index(t *testing.T) {
	mux := controller.PprofMux()
	req := httptest.NewRequest(http.MethodGet, "http://pprof.local/", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	res := rec.Result()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("expected status 200, got %d", res.StatusCode)
	}
	if ct := res.Header.Get("Content-Type"); ct == "" {
		t.Errorf("expected Content-Type to be set")
	}
}

func TestPprofMux_Cmdline_OK(t *testing.T) {
	mux := controller.PprofMux()
	req := httptest.NewRequest(http.MethodGet, "http://pprof.local/cmdline", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	res := rec.Result()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("expected status 200, got %d", res.StatusCode)
	}
}
