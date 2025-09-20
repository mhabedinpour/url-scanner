package v1handler

import (
	"context"
	"net/http"
	"scanner/internal/api/specs/v1specs"
	"scanner/pkg/logger"
)

type Handler struct{}

// Ensure Handler implements v1specs.Handler.
var _ v1specs.Handler = (*Handler)(nil)

func New() *Handler {
	return &Handler{}
}

func (h Handler) NewError(ctx context.Context, err error) *v1specs.ServerErrorStatusCode {
	logger.Error(ctx, err.Error())

	return &v1specs.ServerErrorStatusCode{
		StatusCode: http.StatusInternalServerError,
	}
}
