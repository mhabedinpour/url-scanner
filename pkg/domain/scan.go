package domain

import (
	"time"

	"github.com/google/uuid"
)

// ScanID uniquely identifies a scan job.
// It wraps uuid.UUID to provide type safety at the domain layer.
type ScanID uuid.UUID

// ScanStatus represents the lifecycle state of a scan.
// It can be pending, completed, or failed.
type ScanStatus string

const (
	// ScanStatusPending indicates the scan has been enqueued but not processed yet.
	ScanStatusPending ScanStatus = "PENDING"
	// ScanStatusCompleted indicates the scan finished successfully and a result is available.
	ScanStatusCompleted ScanStatus = "COMPLETED"
	// ScanStatusFailed indicates the scan ended with an error; see LastError and Attempts for details.
	ScanStatusFailed ScanStatus = "FAILED"
)

// ScanResult holds the normalized outcome of a URL scan, including
// page metadata, a verdict, and aggregated stats.
type ScanResult struct {
	Page *struct {
		URL      string `json:"url,omitempty"`
		Domain   string `json:"domain,omitempty"`
		IP       string `json:"ip,omitempty"`
		ASN      string `json:"asn,omitempty"`
		Country  string `json:"country,omitempty"`
		Server   string `json:"server,omitempty"`
		Status   int    `json:"status,omitempty"`
		MimeType string `json:"mimeType,omitempty"`
	} `json:"page,omitempty"`

	Verdict *struct {
		Malicious bool `json:"malicious,omitempty"`
		Score     int  `json:"score,omitempty"`
	} `json:"verdicts,omitempty"`

	Stats *struct {
		Malicious int `json:"malicious,omitempty"`
	} `json:"stats,omitempty"`
}

// Scan represents a single URL scan request and its current state.
// It tracks the target URL, status, result, error information, and timestamps.
type Scan struct {
	// ID is the unique identifier of the scan.
	ID ScanID `json:"id"`
	// UserID is the identifier of the user who requested the scan.
	UserID UserID `json:"userId"`

	// URL is the target that will be scanned.
	URL string `json:"url"`
	// Status is the current lifecycle state of the scan.
	Status ScanStatus `json:"status"`
	// Result contains the latest known outcome of the scan.
	Result ScanResult `json:"result"`

	// Attempts is the number of times the system has tried to process this scan.
	Attempts uint `json:"attempts"`
	// LastError stores the most recent error message, if any, encountered while processing the scan.
	LastError string `json:"-"`

	// CreatedAt is the time when the scan request was created.
	CreatedAt time.Time `json:"createdAt"`
	// UpdatedAt is the time when the scan was last updated (e.g., status or result changed).
	UpdatedAt time.Time `json:"updatedAt"`
	// DeletedAt marks when the scan was soft-deleted; zero value means not deleted.
	DeletedAt time.Time `json:"-"`
}
