package storage

import (
	"context"

	"github.com/riverqueue/river"
)

// JobStorage defines the minimal interface for enqueueing background jobs.
// Implementations are responsible for persisting the job into the underlying
// queue backend. The args parameter contains the job payload and opts can be
// used to customize insertion behavior (e.g., queue name, delay, priority).
// Implementations should return a non-nil error if the job could not be queued.
// The provided context controls cancellation and timeouts of the operation.
//
// Typical implementations live under pkg/storage/<backend>/ and are used via
// the higher-level Storage or TxStorage interfaces.
//
// Example:
//
//	err := storage.AddJob(ctx, MyJobArgs{URL: "https://example.com"}, nil)
//	if err != nil { /* handle error */ }
//
// Note: This interface is intentionally small to keep backends decoupled from
// specific job systems while allowing different drivers to integrate.
type JobStorage interface {
	// AddJob enqueues a new job with the given arguments. It should be atomic
	// with respect to any surrounding transaction when supported by the backend.
	AddJob(ctx context.Context, args river.JobArgs, opts *river.InsertOpts) (bool, error)
}
