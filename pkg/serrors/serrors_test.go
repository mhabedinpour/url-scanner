package serrors_test

import (
	"errors"
	"scanner/pkg/serrors"
	"testing"
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
		if k == nil {
			t.Fatalf("kind at index %d is nil", i)
		}
		if seen[k] {
			t.Fatalf("kind at index %d is duplicate: %v", i, k)
		}
		seen[k] = true
	}

	// Ensure some expected inequalities
	if serrors.ErrNotFound == serrors.ErrUnauthorized {
		t.Fatalf("NotFound should not equal Unauthorized")
	}
}

func TestErrorFormatting(t *testing.T) {
	base := errors.New("db down")

	e1 := serrors.With(serrors.ErrNotFound, "scan %d not found", 42)
	if got, want := e1.Error(), "scan 42 not found"; got != want {
		t.Fatalf("With() Error() = %q, want %q", got, want)
	}

	e2 := serrors.Wrap(serrors.ErrNotFound, base, "getting scan")
	if got, want := e2.Error(), "getting scan: db down"; got != want {
		t.Fatalf("Wrap() Error() = %q, want %q", got, want)
	}

	e3 := serrors.KindOnly(serrors.ErrNotFound)
	if got, want := e3.Error(), "NOT_FOUND"; got != want {
		t.Fatalf("KindOnly Error() = %q, want %q", got, want)
	}
}

func TestIsMatchesKindAndWrapped(t *testing.T) {
	base := customError{"root cause"}
	e := serrors.Wrap(serrors.ErrNotFound, base, "reading")

	if !errors.Is(e, serrors.ErrNotFound) {
		t.Fatalf("errors.Is should match kind sentinel")
	}
	if !errors.Is(e, base) {
		t.Fatalf("errors.Is should match wrapped error")
	}
	if errors.Is(e, serrors.ErrUnauthorized) {
		t.Fatalf("errors.Is should not match a different kind")
	}
}

func TestAsMatchesKindAndWrapped(t *testing.T) {
	base := &customError{"root cause"}
	e := serrors.Wrap(serrors.ErrNotFound, base, "reading")

	var k serrors.Kind
	if !errors.As(e, &k) {
		t.Fatalf("errors.As should extract Kind")
	}
	if k != serrors.ErrNotFound {
		t.Fatalf("extracted Kind = %v, want %v", k, serrors.ErrNotFound)
	}

	var ce *customError
	if !errors.As(e, &ce) {
		t.Fatalf("errors.As should extract wrapped error type")
	}
	if ce != base {
		t.Fatalf("extracted cause pointer mismatch: got %p want %p", ce, base)
	}
}

func TestAccessors(t *testing.T) {
	base := errors.New("boom")
	e := serrors.Wrap(serrors.ErrUnauthorized, base, "no token")
	if e.Kind() != serrors.ErrUnauthorized {
		t.Fatalf("Kind() = %v, want %v", e.Kind(), serrors.ErrUnauthorized)
	}
	if e.Message() != "no token" {
		t.Fatalf("Message() = %q, want %q", e.Message(), "no token")
	}
	if e.Cause() != base { //nolint: errorlint
		t.Fatalf("Cause() mismatch")
	}
}
