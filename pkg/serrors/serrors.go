package serrors

import (
	"errors"
	"fmt"
)

// Kind is a marker interface implemented by all semantic error kinds created
// with NewKind. It allows distinguishing semantic kinds from ordinary errors.
type Kind interface {
	error
	isKind()
}

// kind is an unexported implementation of Kind used as a sentinel value for a
// semantic error category.
type kind struct{ s string }

func (k kind) Error() string { return k.s }
func (k kind) isKind()       {}

// NewKind creates a new semantic error kind (a sentinel) with the provided
// name/description. Kinds are comparable and can be used with errors.Is/As
// through the serrors.Error wrapper.
func NewKind(name string) Kind { return kind{s: name} }

// Default Kinds provide a common set of categories for typical application
// semantics. They are implemented as sentinels and can be used with errors.Is/As
// through the Error wrapper defined in this package.
var (
	// ErrNotFound indicates the requested entity was not found.
	ErrNotFound = NewKind("NOT_FOUND")
	// ErrUnauthorized indicates missing or invalid authentication.
	ErrUnauthorized = NewKind("UNAUTHORIZED")
	// ErrForbidden indicates the caller is authenticated but not allowed to perform the operation.
	ErrForbidden = NewKind("FORBIDDEN")
	// ErrBadRequest indicates the client sent invalid data.
	ErrBadRequest = NewKind("BAD_REQUEST")
	// ErrConflict indicates a state conflict (e.g., resource already exists or version mismatch).
	ErrConflict = NewKind("CONFLICT")
	// ErrInternal indicates an internal server error.
	ErrInternal = NewKind("INTERNAL")
	// ErrTimeout indicates the operation timed out.
	ErrTimeout = NewKind("TIMEOUT")
	// ErrUnavailable indicates the service is temporarily unavailable.
	ErrUnavailable = NewKind("UNAVAILABLE")
	// ErrRateLimited indicates too many requests.
	ErrRateLimited = NewKind("RATE_LIMITED")
)

// Error represents a semantic error carrying a kind (sentinel), an optional
// wrapped error and an optional arbitrary message. It fully supports
// errors.Is/errors.As and unwrapping.
//
// Matching semantics:
//   - errors.Is(err, target) will match if target matches either the kind
//     sentinel or the wrapped error.
//   - errors.As(err, target) will succeed for either the kind sentinel or the
//     wrapped error.
//
// Error string formatting:
//   - If both msg and err are set: "<msg>: <err>"
//   - If only msg is set: "<msg>"
//   - If only err is set: "<err>"
//   - If neither set: the kind's Error() string.
type Error struct {
	kind Kind  // semantic kind sentinel
	err  error // wrapped error (optional)
	msg  string
}

// With constructs a new semantic error with the given kind and an arbitrary
// human-readable message. Use Wrap if you also want to wrap a concrete cause.
func With(k Kind, msgFmt string, args ...any) *Error {
	return &Error{kind: k, msg: fmt.Sprintf(msgFmt, args...)}
}

// Wrap constructs a new semantic error with the given kind, wraps the provided
// cause (err) and allows adding an arbitrary message.
func Wrap(k Kind, err error, msgFmt string, args ...any) *Error {
	return &Error{kind: k, err: err, msg: fmt.Sprintf(msgFmt, args...)}
}

// KindOnly creates a semantic error carrying only the kind without extra
// message or concrete cause.
func KindOnly(k Kind) *Error { return &Error{kind: k} }

// Error implements the error interface.
func (e *Error) Error() string {
	switch {
	case e == nil:
		return "<nil>"
	case e.msg != "" && e.err != nil:
		return e.msg + ": " + e.err.Error()
	case e.msg != "":
		return e.msg
	case e.err != nil:
		return e.err.Error()
	default:
		if e.kind != nil {
			return e.kind.Error()
		}

		return "unknown error"
	}
}

// Unwrap returns the wrapped error, enabling errors.Unwrap/Is/As to traverse
// the underlying cause chain.
func (e *Error) Unwrap() error { return e.err }

// Is enables matching against either the semantic kind sentinel or the wrapped
// error in the chain. This ensures that errors.Is works for both.
func (e *Error) Is(target error) bool {
	if e == nil || target == nil {
		return e == nil && target == nil
	}
	if e.kind != nil && errors.Is(e.kind, target) {
		return true
	}
	if e.err != nil && errors.Is(e.err, target) {
		return true
	}

	return false
}

// As enables type assertions against either the semantic kind sentinel or the
// wrapped error in the chain.
func (e *Error) As(target any) bool {
	if e == nil || target == nil {
		return false
	}
	if e.kind != nil && errors.As(e.kind, target) {
		return true
	}
	if e.err != nil && errors.As(e.err, target) {
		return true
	}

	return false
}

// Kind returns the semantic kind sentinel associated with this error, or nil.
func (e *Error) Kind() Kind { return e.kind }

// Message returns the arbitrary message attached to this error.
func (e *Error) Message() string { return e.msg }

// Cause returns the wrapped cause (may be nil).
func (e *Error) Cause() error { return e.err }
