package v1handler

import (
	"context"
	"scanner/internal/api/specs/v1specs"
)

func (h Handler) CreateScan(ctx context.Context, req *v1specs.CreateScanRequest) (v1specs.CreateScanRes, error) {
	// TODO implement me
	panic("implement me")
}

func (h Handler) DeleteScan(ctx context.Context, params v1specs.DeleteScanParams) (v1specs.DeleteScanRes, error) {
	// TODO implement me
	panic("implement me")
}

func (h Handler) GetScan(ctx context.Context, params v1specs.GetScanParams) (v1specs.GetScanRes, error) {
	return &v1specs.Scan{}, nil
}

func (h Handler) ListScans(ctx context.Context, params v1specs.ListScansParams) (v1specs.ListScansRes, error) {
	// TODO implement me
	panic("implement me")
}
