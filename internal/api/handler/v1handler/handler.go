// Package v1handler contains HTTP handlers for the version 1 API.
// It implements the ogen-generated interfaces and translates domain errors
// into structured API responses and appropriate HTTP status codes.
package v1handler

import (
	"context"
	"errors"
	"net/http"
	"scanner/internal/api/specs/v1specs"
	"scanner/internal/scanner"
	"scanner/pkg/logger"
	"scanner/pkg/serrors"

	"github.com/ogen-go/ogen/ogenerrors"
	"go.uber.org/zap"
)

// Deps lists all dependencies of the Handler.
type Deps struct {
	Scanner scanner.Scanner
}

// Handler implements v1specs.Handler and provides endpoint methods for the v1 API.
type Handler struct {
	deps Deps
}

// Ensure Handler implements v1specs.Handler.
var _ v1specs.Handler = (*Handler)(nil)

// New constructs and returns a new Handler instance.
func New(deps Deps) *Handler {
	return &Handler{
		deps: deps,
	}
}

// NewError maps internal errors into an API-friendly error response with an HTTP status code.
// It inspects wrapped semantic errors (serrors.Error) and well-known kinds
// to select status code and message. Internal/unknown errors are logged and
// converted to a generic 500 response.
func (h Handler) NewError(ctx context.Context, err error) *v1specs.ServerErrorStatusCode {
	var kind serrors.Kind
	var sem *serrors.Error
	// try to extract semantic kind or error wrapper
	if errors.As(err, &sem) && sem != nil && sem.Kind() != nil {
		kind = sem.Kind()
	} else {
		// try to set kind directly
		errors.As(err, &kind)
	}

	if errors.Is(err, ogenerrors.ErrSecurityRequirementIsNotSatisfied) {
		kind = serrors.ErrUnauthorized
	}

	status := http.StatusInternalServerError
	code := serrors.ErrInternal.Error()
	msg := "internal error"

	// map known kinds to HTTP status codes and messages.
	if kind != nil {
		switch kind {
		case serrors.ErrNotFound:
			status = http.StatusNotFound
			code = serrors.ErrNotFound.Error()
			msg = "resource not found"
		case serrors.ErrUnauthorized:
			status = http.StatusUnauthorized
			code = serrors.ErrUnauthorized.Error()
			msg = "unauthorized"
		case serrors.ErrBadRequest:
			status = http.StatusBadRequest
			code = serrors.ErrBadRequest.Error()
			msg = "bad request"
		case serrors.ErrInternal:
			// keep defaults
		}
	}

	// for internal or non-sentinel errors, log full error and respond with generic internal error
	if kind == nil || errors.Is(kind, serrors.ErrInternal) {
		logger.Error(ctx, "error in handling requests", zap.Error(err))

		return &v1specs.ServerErrorStatusCode{
			StatusCode: http.StatusInternalServerError,
			Response:   v1specs.Error{Code: code, Message: msg},
		}
	}

	// for known non-internal kinds, include message if provided
	if sem != nil && sem.Message() != "" {
		msg = sem.Message()
	}

	return &v1specs.ServerErrorStatusCode{
		StatusCode: status,
		Response:   v1specs.Error{Code: code, Message: msg},
	}
}
