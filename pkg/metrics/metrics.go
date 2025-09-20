package metrics

// DefaultBuckets provides a common set of histogram buckets in seconds that can
// be reused across the application for latency metrics.
var DefaultBuckets = []float64{.001, .005, .01, .025, .05, .1, .25, .5, 1, 2.5, 5, 10} //nolint: gochecknoglobals
