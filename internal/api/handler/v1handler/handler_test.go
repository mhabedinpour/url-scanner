package v1handler_test

import (
	"context"
	"errors"
	"scanner/internal/api/handler/v1handler"
	"testing"

	"scanner/pkg/logger"
	"scanner/pkg/serrors"
)

func TestMain(m *testing.M) {
	// Initialize logger to avoid nil pointer deref during tests
	logger.Setup(logger.DevelopmentEnvironment)
	m.Run()
}

func TestNewError_InternalOnPlainError(t *testing.T) {
	h := v1handler.New()
	ctx := context.Background()

	res := h.NewError(ctx, errors.New("boom"))
	if res == nil {
		t.Fatalf("expected non-nil response")
	}
	if got, want := res.StatusCode, 500; got != want {
		t.Fatalf("status = %d, want %d", got, want)
	}
	if got, want := res.Response.Code, serrors.ErrInternal.Error(); got != want {
		t.Fatalf("code = %q, want %q", got, want)
	}
	if got, want := res.Response.Message, "internal error"; got != want {
		t.Fatalf("message = %q, want %q", got, want)
	}
}

func TestNewError_KindSentinelDirect_NotFound(t *testing.T) {
	h := v1handler.New()
	ctx := context.Background()

	// Pass the Kind sentinel directly
	res := h.NewError(ctx, serrors.ErrNotFound)
	if got, want := res.StatusCode, 404; got != want {
		t.Fatalf("status = %d, want %d", got, want)
	}
	if got, want := res.Response.Code, serrors.ErrNotFound.Error(); got != want {
		t.Fatalf("code = %q, want %q", got, want)
	}
	if got, want := res.Response.Message, "resource not found"; got != want {
		t.Fatalf("message = %q, want %q", got, want)
	}
}

func TestNewError_SemanticWithMessage_BadRequest(t *testing.T) {
	h := v1handler.New()
	ctx := context.Background()

	err := serrors.With(serrors.ErrBadRequest, "invalid payload: missing url")
	res := h.NewError(ctx, err)
	if got, want := res.StatusCode, 400; got != want {
		t.Fatalf("status = %d, want %d", got, want)
	}
	if got, want := res.Response.Code, serrors.ErrBadRequest.Error(); got != want {
		t.Fatalf("code = %q, want %q", got, want)
	}
	if got, want := res.Response.Message, "invalid payload: missing url"; got != want {
		t.Fatalf("message = %q, want %q", got, want)
	}
}

func TestNewError_SemanticWrap_Unauthorized(t *testing.T) {
	h := v1handler.New()
	ctx := context.Background()

	cause := errors.New("bad token")
	err := serrors.Wrap(serrors.ErrUnauthorized, cause, "unauthorized")
	res := h.NewError(ctx, err)
	if got, want := res.StatusCode, 401; got != want {
		t.Fatalf("status = %d, want %d", got, want)
	}
	if got, want := res.Response.Code, serrors.ErrUnauthorized.Error(); got != want {
		t.Fatalf("code = %q, want %q", got, want)
	}
	// Should include provided message, not the cause
	if got, want := res.Response.Message, "unauthorized"; got != want {
		t.Fatalf("message = %q, want %q", got, want)
	}
}

func TestNewError_InternalKind_GeneratesInternal(t *testing.T) {
	h := v1handler.New()
	ctx := context.Background()

	res := h.NewError(ctx, serrors.KindOnly(serrors.ErrInternal))
	if got, want := res.StatusCode, 500; got != want {
		t.Fatalf("status = %d, want %d", got, want)
	}
	if got, want := res.Response.Code, serrors.ErrInternal.Error(); got != want {
		t.Fatalf("code = %q, want %q", got, want)
	}
	if got, want := res.Response.Message, "internal error"; got != want {
		t.Fatalf("message = %q, want %q", got, want)
	}
}
