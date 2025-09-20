package v1handler

import (
	"context"
	"scanner/internal/api/specs/v1specs"
)

type SecHandler struct{}

func NewSecHandler() *SecHandler {
	return &SecHandler{}
}

// Ensure Handler implements v1specs.Handler.
var _ v1specs.SecurityHandler = (*SecHandler)(nil)

func (s SecHandler) HandleBearerAuth(
	ctx context.Context,
	operationName v1specs.OperationName,
	t v1specs.BearerAuth) (context.Context, error) {
	return ctx, nil
}
