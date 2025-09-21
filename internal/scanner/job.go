package scanner

import (
	"time"

	"github.com/riverqueue/river"
	"github.com/riverqueue/river/rivertype"
)

// JobArgs contains the arguments for a scan job submitted to River.
// The struct is used as the unique key for jobs to prevent duplicate work per URL.
type JobArgs struct {
	// URL is the address to scan. It is marked as unique so River can enforce
	// one job per URL according to InsertOpts.UniqueOpts.
	URL string `json:"url" river:"unique"`

	// maxAttempts configures the maximum number of times River should retry the job.
	maxAttempts int
	// uniqueJobPeriod defines the lookback window during which a job with the
	// same arguments is considered a duplicate across the specified states.
	uniqueJobPeriod time.Duration
}

// Kind returns the River job kind used to register and dispatch the scan worker.
func (args JobArgs) Kind() string { return "ScanURLJob" }

// InsertOpts returns the River options that control how the job is enqueued,
// including the maximum retry attempts and uniqueness constraints to prevent
// duplicate jobs for the same URL across multiple job states.
func (args JobArgs) InsertOpts() river.InsertOpts {
	return river.InsertOpts{
		MaxAttempts: args.maxAttempts,
		// make sure we only have one job per URL in any state
		UniqueOpts: river.UniqueOpts{
			ByArgs:   true,
			ByPeriod: args.uniqueJobPeriod,
			ByState: []rivertype.JobState{
				rivertype.JobStateAvailable,
				rivertype.JobStateCompleted,
				rivertype.JobStatePending,
				rivertype.JobStateRunning,
				rivertype.JobStateRetryable,
				rivertype.JobStateScheduled,
			},
		},
	}
}
