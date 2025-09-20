package controller

import (
	"context"
	"net"
	"net/http"
	"scanner/pkg/logger"
	"strings"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"
)

// statusRecorder wraps http.ResponseWriter to capture the final HTTP status
// code written by the downstream handler.
type statusRecorder struct {
	http.ResponseWriter

	status int
}

// WriteHeader records the status code and forwards the call to the underlying writer.
func (rec *statusRecorder) WriteHeader(code int) {
	rec.status = code
	rec.ResponseWriter.WriteHeader(code)
}

// GetClientIP attempts to determine the originating client IP address for the
// given request by checking X-Forwarded-For and X-Real-IP headers before
// falling back to the connection's remote address.
func GetClientIP(r *http.Request) string {
	// check X-Forwarded-For first
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		// may contain multiple IPs: "client, proxy1, proxy2"
		ips := strings.Split(xff, ",")

		return strings.TrimSpace(ips[0]) // the first is original client
	}

	// then check X-Real-IP
	if xrip := r.Header.Get("X-Real-IP"); xrip != "" {
		return xrip
	}

	// fallback to RemoteAddr
	ip, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}

	return ip
}

// CtxKey is a string-based type used for storing values in request contexts.
// It avoids collisions with other packages' context keys.
type CtxKey string

const (
	// RequestIDKey is the context key under which the current request ID is stored.
	RequestIDKey CtxKey = "RequestID"
)

// WithLogger returns a middleware that injects a request-scoped logger and
// request ID into the context, then logs a structured access log after the
// handler finishes.
func WithLogger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		// set request ID
		requestID := r.Header.Get("X-Request-Id")
		if requestID == "" {
			requestID = uuid.New().String()
		}
		ctx = context.WithValue(ctx, RequestIDKey, requestID)

		// set logger
		ctx = logger.WithFields(ctx, zap.String(string(RequestIDKey), requestID))

		start := time.Now()
		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}

		next.ServeHTTP(rec, r.WithContext(ctx))

		logger.Info(ctx, "Access log",
			zap.Int("status_code", rec.status),
			zap.Float64("latency", time.Since(start).Seconds()),
			zap.String("client_ip", GetClientIP(r)),
			zap.String("user_agent", r.UserAgent()),
			zap.String("url", r.URL.String()),
			zap.String("referer", r.Referer()),
			zap.String("method", r.Method),
		)
	})
}
