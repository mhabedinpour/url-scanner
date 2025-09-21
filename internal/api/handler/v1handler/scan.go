package v1handler

import (
	"context"
	"fmt"
	"net/url"
	"scanner/internal/api/specs/v1specs"
	"scanner/pkg/domain"

	"github.com/google/uuid"
)

const DefaultLimit = 20

func DomainScanResultToV1Specs(in *domain.ScanResult) *v1specs.ScanResult {
	var out v1specs.ScanResult
	// Page
	//nolint: nestif
	if in.Page != nil {
		p := in.Page
		var page v1specs.ScanResultPage
		if p.URL != "" {
			if u, err := url.Parse(p.URL); err == nil {
				page.URL = v1specs.NewOptURI(*u)
			}
		}
		if p.Domain != "" {
			page.Domain = v1specs.NewOptString(p.Domain)
		}
		if p.IP != "" {
			page.IP = v1specs.NewOptString(p.IP)
		}
		if p.ASN != "" {
			page.Asn = v1specs.NewOptString(p.ASN)
		}
		if p.Country != "" {
			page.Country = v1specs.NewOptString(p.Country)
		}
		if p.Server != "" {
			page.Server = v1specs.NewOptString(p.Server)
		}
		out.Page = page
	}
	// Verdicts
	if in.Verdict != nil {
		v := in.Verdict
		var ver v1specs.ScanResultVerdicts
		ver.Malicious = v1specs.NewOptBool(v.Malicious)
		if v.Score != 0 {
			ver.Score = v1specs.NewOptInt(v.Score)
		}
		out.Verdicts = ver
	}
	// Stats
	if in.Stats != nil {
		s := in.Stats
		var stats v1specs.ScanResultStats
		if s.Malicious != 0 {
			stats.Malicious = v1specs.NewOptInt(s.Malicious)
		}
		out.Stats = stats
	}

	return &out
}

func DomainScanToV1Specs(in *domain.Scan) (*v1specs.Scan, error) {
	URL, err := url.Parse(in.URL)
	if err != nil {
		return nil, fmt.Errorf("could not parse URL: %w", err)
	}

	updateAt := v1specs.OptDateTime{}
	if !in.UpdatedAt.IsZero() {
		updateAt.SetTo(in.UpdatedAt)
	}

	return &v1specs.Scan{
		ID:        uuid.UUID(in.ID),
		URL:       *URL,
		Status:    v1specs.ScanStatus(in.Status),
		Result:    *DomainScanResultToV1Specs(&in.Result),
		Attempts:  int(in.Attempts), //nolint: gosec
		CreatedAt: in.CreatedAt,
		UpdatedAt: updateAt,
	}, nil
}

// CreateScan schedules a new scan based on the provided request payload.
func (h Handler) CreateScan(ctx context.Context, req *v1specs.CreateScanRequest) (v1specs.CreateScanRes, error) {
	s, err := h.deps.Scanner.Enqueue(ctx, GetUserIDFromContext(ctx), req.URL.String())
	if err != nil {
		return nil, err //nolint: wrapcheck
	}

	return DomainScanToV1Specs(s)
}

// DeleteScan deletes a scan by ID.
func (h Handler) DeleteScan(ctx context.Context, params v1specs.DeleteScanParams) (v1specs.DeleteScanRes, error) {
	err := h.deps.Scanner.Delete(ctx, GetUserIDFromContext(ctx), domain.ScanID(params.ID))
	if err != nil {
		return nil, err //nolint: wrapcheck
	}

	return &v1specs.DeleteScanNoContent{}, nil
}

// GetScan returns details of a scan by ID.
func (h Handler) GetScan(ctx context.Context, params v1specs.GetScanParams) (v1specs.GetScanRes, error) {
	s, err := h.deps.Scanner.Result(ctx, GetUserIDFromContext(ctx), domain.ScanID(params.ID))
	if err != nil {
		return nil, err //nolint: wrapcheck
	}

	return DomainScanToV1Specs(s)
}

// ListScans returns a paginated list of scans.
func (h Handler) ListScans(ctx context.Context, params v1specs.ListScansParams) (v1specs.ListScansRes, error) {
	scans, nextCursor, err := h.deps.Scanner.UserScans(ctx,
		GetUserIDFromContext(ctx),
		domain.ScanStatus(params.Status.Value),
		params.Cursor.Value,
		uint(params.Limit.Or(DefaultLimit))) //nolint: gosec
	if err != nil {
		return nil, err //nolint: wrapcheck
	}

	items := make([]v1specs.Scan, 0, len(scans))
	for i := range scans {
		v1s, err := DomainScanToV1Specs(&scans[i])
		if err != nil {
			return nil, err
		}
		items = append(items, *v1s)
	}

	var cursorOpt v1specs.OptNilString
	if nextCursor != "" {
		cursorOpt = v1specs.NewOptNilString(nextCursor)
	}

	return &v1specs.ScanList{
		Items:      items,
		NextCursor: cursorOpt,
	}, nil
}
