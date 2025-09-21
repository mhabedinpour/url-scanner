// Package urlscanner defines interfaces and data types used to submit
// URLs for scanning and retrieve scan results from a backing provider.
package urlscanner

import (
	"context"
	"scanner/pkg/domain"
	"time"
)

// RateLimitStatus describes the current API rate‑limit status returned by the
// underlying URL scanning provider.
type RateLimitStatus struct {
	Limit     int       // Limit is the total number of allowed requests in the current window.
	Remaining int       // Remaining indicates how many requests are left in the current window.
	ResetAt   time.Time // ResetAt is when the rate‑limit window resets.
}

// SubmitRes represents the response of a successful URL submission.
type SubmitRes struct {
	ID string // ID is the scan job identifier returned by the provider.
}

// Client is the abstraction for URL scanners. Implementations submit URLs
// for scanning and later fetch their results.
//
//go:generate mockgen -package mockurlscanner -source=interface.go -destination=mock/mockurlscanner.go *
type Client interface {
	// SubmitURL submits the target URL for scanning and returns a provider
	// job ID plus the current rate‑limit status.
	SubmitURL(ctx context.Context, URL string) (SubmitRes, RateLimitStatus, error)
	// Result retrieves the result for a previously submitted job by its ID.
	Result(ctx context.Context, scanID string) (*domain.ScanResult, error)
}
