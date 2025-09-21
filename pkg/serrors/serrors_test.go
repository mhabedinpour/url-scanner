package serrors_test

import (
	"errors"
	"scanner/pkg/serrors"
	"testing"

	"github.com/stretchr/testify/require"
)

type customError struct{ msg string }

func (e customError) Error() string { return e.msg }

func TestDefaultKindsDistinct(t *testing.T) {
	kinds := []serrors.Kind{
		serrors.ErrNotFound,
		serrors.ErrUnauthorized,
		serrors.ErrForbidden,
		serrors.ErrBadRequest,
		serrors.ErrConflict,
		serrors.ErrInternal,
		serrors.ErrTimeout,
		serrors.ErrUnavailable,
		serrors.ErrRateLimited,
	}
	seen := map[serrors.Kind]bool{}
	for i, k := range kinds {
		require.NotNil(t, k, "kind at index %d is nil", i)
		require.False(t, seen[k], "kind at index %d is duplicate: %v", i, k)
		seen[k] = true
	}

	// Ensure some expected inequalities
	require.NotEqual(t, serrors.ErrNotFound, serrors.ErrUnauthorized, "NotFound should not equal Unauthorized")
}

func TestErrorFormatting(t *testing.T) {
	base := errors.New("db down")

	e1 := serrors.With(serrors.ErrNotFound, "scan %d not found", 42)
	require.Equal(t, "scan 42 not found", e1.Error(), "With() Error() mismatch")

	e2 := serrors.Wrap(serrors.ErrNotFound, base, "getting scan")
	require.Equal(t, "getting scan: db down", e2.Error(), "Wrap() Error() mismatch")

	e3 := serrors.KindOnly(serrors.ErrNotFound)
	require.Equal(t, "NOT_FOUND", e3.Error(), "KindOnly Error() mismatch")
}

func TestIsMatchesKindAndWrapped(t *testing.T) {
	base := customError{"root cause"}
	e := serrors.Wrap(serrors.ErrNotFound, base, "reading")

	require.ErrorIs(t, e, serrors.ErrNotFound)
	require.ErrorIs(t, e, base)
	require.NotErrorIs(t, e, serrors.ErrUnauthorized, "errors.Is should not match a different kind")
}

func TestAsMatchesKindAndWrapped(t *testing.T) {
	base := &customError{"root cause"}
	e := serrors.Wrap(serrors.ErrNotFound, base, "reading")

	var k serrors.Kind
	require.ErrorAs(t, e, &k, "errors.As should extract Kind")
	require.Equal(t, serrors.ErrNotFound, k)

	var ce *customError
	require.ErrorAs(t, e, &ce, "errors.As should extract wrapped error type")
	require.Equal(t, base, ce, "extracted cause pointer mismatch")
}

func TestAccessors(t *testing.T) {
	base := errors.New("boom")
	e := serrors.Wrap(serrors.ErrUnauthorized, base, "no token")
	require.Equal(t, serrors.ErrUnauthorized, e.Kind())
	require.Equal(t, "no token", e.Message())
	require.Equal(t, base, e.Cause())
}
