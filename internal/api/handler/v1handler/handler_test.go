package v1handler_test

import (
	"context"
	"errors"
	"scanner/internal/api/handler/v1handler"
	"testing"

	"scanner/pkg/logger"
	"scanner/pkg/serrors"

	"github.com/stretchr/testify/require"
)

func TestMain(m *testing.M) {
	// Initialize logger to avoid nil pointer deref during tests
	logger.Setup(logger.DevelopmentEnvironment)
	m.Run()
}

func TestNewError_InternalOnPlainError(t *testing.T) {
	h := v1handler.New(v1handler.Deps{})
	ctx := context.Background()

	res := h.NewError(ctx, errors.New("boom"))
	require.NotNil(t, res)
	require.Equal(t, 500, res.StatusCode)
	require.Equal(t, serrors.ErrInternal.Error(), res.Response.Code)
	require.Equal(t, "internal error", res.Response.Message)
}

func TestNewError_KindSentinelDirect_NotFound(t *testing.T) {
	h := v1handler.New(v1handler.Deps{})
	ctx := context.Background()

	// Pass the Kind sentinel directly
	res := h.NewError(ctx, serrors.ErrNotFound)
	require.Equal(t, 404, res.StatusCode)
	require.Equal(t, serrors.ErrNotFound.Error(), res.Response.Code)
	require.Equal(t, "resource not found", res.Response.Message)
}

func TestNewError_SemanticWithMessage_BadRequest(t *testing.T) {
	h := v1handler.New(v1handler.Deps{})
	ctx := context.Background()

	err := serrors.With(serrors.ErrBadRequest, "invalid payload: missing url")
	res := h.NewError(ctx, err)
	require.Equal(t, 400, res.StatusCode)
	require.Equal(t, serrors.ErrBadRequest.Error(), res.Response.Code)
	require.Equal(t, "invalid payload: missing url", res.Response.Message)
}

func TestNewError_SemanticWrap_Unauthorized(t *testing.T) {
	h := v1handler.New(v1handler.Deps{})
	ctx := context.Background()

	cause := errors.New("bad token")
	err := serrors.Wrap(serrors.ErrUnauthorized, cause, "unauthorized")
	res := h.NewError(ctx, err)
	require.Equal(t, 401, res.StatusCode)
	require.Equal(t, serrors.ErrUnauthorized.Error(), res.Response.Code)
	// Should include provided message, not the cause
	require.Equal(t, "unauthorized", res.Response.Message)
}

func TestNewError_InternalKind_GeneratesInternal(t *testing.T) {
	h := v1handler.New(v1handler.Deps{})
	ctx := context.Background()

	res := h.NewError(ctx, serrors.KindOnly(serrors.ErrInternal))
	require.Equal(t, 500, res.StatusCode)
	require.Equal(t, serrors.ErrInternal.Error(), res.Response.Code)
	require.Equal(t, "internal error", res.Response.Message)
}
