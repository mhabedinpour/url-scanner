package v1handler_test

import (
	"context"
	"net/url"
	"testing"
	"time"

	"github.com/google/uuid"
	"go.uber.org/mock/gomock"

	"scanner/internal/api/handler/v1handler"
	"scanner/internal/api/specs/v1specs"
	mockscanner "scanner/internal/scanner/mock"
	"scanner/pkg/domain"
)

func Test_toV1Result_Mapping(t *testing.T) {
	in := &domain.ScanResult{
		Page: &struct {
			URL      string `json:"url,omitempty"`
			Domain   string `json:"domain,omitempty"`
			IP       string `json:"ip,omitempty"`
			ASN      string `json:"asn,omitempty"`
			Country  string `json:"country,omitempty"`
			Server   string `json:"server,omitempty"`
			Status   int    `json:"status,omitempty"`
			MimeType string `json:"mimeType,omitempty"`
		}{
			URL:      "https://example.com/index.html",
			Domain:   "example.com",
			IP:       "93.184.216.34",
			ASN:      "AS15133",
			Country:  "US",
			Server:   "nginx",
			Status:   200,
			MimeType: "text/html",
		},
		Verdict: &struct {
			Malicious bool `json:"malicious,omitempty"`
			Score     int  `json:"score,omitempty"`
		}{
			Malicious: true,
			Score:     42,
		},
		Stats: &struct {
			Malicious int `json:"malicious,omitempty"`
		}{
			Malicious: 3,
		},
	}

	out := v1handler.DomainScanResultToV1Specs(in)

	if !out.Page.URL.IsSet() {
		t.Fatalf("expected page.url to be set")
	}
	if got := out.Page.URL.Value.String(); got != "https://example.com/index.html" {
		t.Fatalf("url = %q", got)
	}
	if got := out.Page.Domain.Value; got != "example.com" {
		t.Fatalf("domain = %q", got)
	}
	if got := out.Page.IP.Value; got != "93.184.216.34" {
		t.Fatalf("ip = %q", got)
	}
	if got := out.Page.Asn.Value; got != "AS15133" {
		t.Fatalf("asn = %q", got)
	}
	if got := out.Page.Country.Value; got != "US" {
		t.Fatalf("country = %q", got)
	}
	if got := out.Page.Server.Value; got != "nginx" {
		t.Fatalf("server = %q", got)
	}
	if got := out.Page.Status.Value; got != 200 {
		t.Fatalf("status = %d", got)
	}
	if got := out.Page.MimeType.Value; got != "text/html" {
		t.Fatalf("mime = %q", got)
	}

	if !out.Verdicts.Malicious.Value {
		t.Fatalf("malicious expected true")
	}
	if got := out.Verdicts.Score.Value; got != 42 {
		t.Fatalf("score = %d", got)
	}

	if got := out.Stats.Malicious.Value; got != 3 {
		t.Fatalf("stats.malicious = %d", got)
	}
}

func Test_toV1Result_OptionalFieldsUnset_WhenEmpty(t *testing.T) {
	// Supply empty result to ensure no optional fields are set
	in := &domain.ScanResult{}
	out := v1handler.DomainScanResultToV1Specs(in)
	// page is zero; Page fields should be zero-values
	if (out.Page != v1specs.ScanResultPage{}) {
		t.Fatalf("expected empty page struct, got %#v", out.Page)
	}
	if out.Verdicts.Malicious.IsSet() {
		t.Fatalf("malicious should not be set by default")
	}
	if out.Stats.Malicious.IsSet() {
		t.Fatalf("stats.malicious should not be set by default")
	}
}

func Test_toV1Specs_Success(t *testing.T) {
	id := uuid.New()
	now := time.Now().UTC().Truncate(time.Second)
	in := &domain.Scan{
		ID:        domain.ScanID(id),
		URL:       "https://example.org/x",
		Status:    domain.ScanStatusCompleted,
		Attempts:  2,
		CreatedAt: now,
		UpdatedAt: now,
	}
	out, err := v1handler.DomainScanToV1Specs(in)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out.ID != id {
		t.Fatalf("id mismatch")
	}
	if out.URL.String() != "https://example.org/x" {
		t.Fatalf("url mismatch: %s", out.URL.String())
	}
	if out.Status != v1specs.ScanStatus(domain.ScanStatusCompleted) {
		t.Fatalf("status mismatch")
	}
	if out.Attempts != 2 {
		t.Fatalf("attempts mismatch")
	}
	if !out.CreatedAt.Equal(now) {
		t.Fatalf("createdAt mismatch")
	}
	if !out.UpdatedAt.IsSet() {
		t.Fatalf("updatedAt should be set")
	}
}

func Test_toV1Specs_InvalidURL_Error(t *testing.T) {
	in := &domain.Scan{URL: "://bad url"}
	_, err := v1handler.DomainScanToV1Specs(in)
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
}

func TestHandler_CreateScan(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	m := mockscanner.NewMockScanner(ctrl)
	h := v1handler.New(v1handler.Deps{Scanner: m})

	userID := domain.UserID(uuid.New())
	ctx := context.WithValue(context.Background(), v1handler.UserIDKey, userID)

	// input
	u, _ := url.Parse("https://e.com")
	req := &v1specs.CreateScanRequest{URL: *u}

	// expect
	scan := sampleScan(userID, "https://e.com")
	m.EXPECT().Enqueue(ctx, userID, "https://e.com").Return(&scan, nil)

	res, err := h.CreateScan(ctx, req)
	if err != nil {
		t.Fatalf("CreateScan error: %v", err)
	}
	if res == nil {
		t.Fatalf("nil response")
	}
	got := res.(*v1specs.Scan)
	if got.URL.String() != "https://e.com" {
		t.Fatalf("URL mismatch: %s", got.URL.String())
	}
}

func TestHandler_DeleteScan(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	m := mockscanner.NewMockScanner(ctrl)
	h := v1handler.New(v1handler.Deps{Scanner: m})

	userID := domain.UserID(uuid.New())
	ctx := context.WithValue(context.Background(), v1handler.UserIDKey, userID)

	id := uuid.New()
	m.EXPECT().Delete(ctx, userID, domain.ScanID(id)).Return(nil)

	res, err := h.DeleteScan(ctx, v1specs.DeleteScanParams{ID: id})
	if err != nil {
		t.Fatalf("DeleteScan error: %v", err)
	}
	if _, ok := res.(*v1specs.DeleteScanNoContent); !ok {
		t.Fatalf("expected NoContent response, got %T", res)
	}
}

func TestHandler_GetScan(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	m := mockscanner.NewMockScanner(ctrl)
	h := v1handler.New(v1handler.Deps{Scanner: m})

	userID := domain.UserID(uuid.New())
	ctx := context.WithValue(context.Background(), v1handler.UserIDKey, userID)

	scan := sampleScan(userID, "https://abc.xyz")
	m.EXPECT().Result(ctx, userID, scan.ID).Return(&scan, nil)

	res, err := h.GetScan(ctx, v1specs.GetScanParams{ID: uuid.UUID(scan.ID)})
	if err != nil {
		t.Fatalf("GetScan error: %v", err)
	}
	got := res.(*v1specs.Scan)
	if got.URL.String() != "https://abc.xyz" {
		t.Fatalf("URL mismatch: %s", got.URL.String())
	}
}

func TestHandler_ListScans_DefaultLimitAndCursor(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	m := mockscanner.NewMockScanner(ctrl)
	h := v1handler.New(v1handler.Deps{Scanner: m})

	userID := domain.UserID(uuid.New())
	ctx := context.WithValue(context.Background(), v1handler.UserIDKey, userID)

	s1 := sampleScan(userID, "https://a")
	s2 := sampleScan(userID, "https://b")
	scans := []domain.Scan{s1, s2}
	next := "cursor123"

	params := v1specs.ListScansParams{}
	// default status zero-value and limit unset; expect DefaultLimit
	m.EXPECT().UserScans(ctx,
		userID,
		domain.ScanStatus(params.Status.Value),
		params.Cursor.Value,
		uint(v1handler.DefaultLimit),
	).Return(scans, next, nil)

	res, err := h.ListScans(ctx, params)
	if err != nil {
		t.Fatalf("ListScans error: %v", err)
	}
	lst := res.(*v1specs.ScanList)
	if len(lst.Items) != 2 {
		t.Fatalf("items len = %d", len(lst.Items))
	}
	if !lst.NextCursor.IsSet() || lst.NextCursor.Value != next {
		t.Fatalf("expected next cursor %q, got %#v", next, lst.NextCursor)
	}
}

func TestHandler_ListScans_CustomLimit_NoNextCursor(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	m := mockscanner.NewMockScanner(ctrl)
	h := v1handler.New(v1handler.Deps{Scanner: m})

	userID := domain.UserID(uuid.New())
	ctx := context.WithValue(context.Background(), v1handler.UserIDKey, userID)

	scans := []domain.Scan{}
	params := v1specs.ListScansParams{
		Limit:  v1specs.NewOptInt(5),
		Cursor: v1specs.NewOptNilString("c0"),
		Status: v1specs.NewOptScanStatus(v1specs.ScanStatus(domain.ScanStatusPending)),
	}
	m.EXPECT().UserScans(ctx, userID, domain.ScanStatusPending, "c0", uint(5)).Return(scans, "", nil)

	res, err := h.ListScans(ctx, params)
	if err != nil {
		t.Fatalf("ListScans error: %v", err)
	}
	lst := res.(*v1specs.ScanList)
	if len(lst.Items) != 0 {
		t.Fatalf("expected empty list")
	}
	if lst.NextCursor.IsSet() {
		t.Fatalf("next cursor should be unset when empty")
	}
}

// We cannot directly call unexported functions from another package.
// Define small exported wrappers in a test-only file within the package under test.

// sampleScan constructs a minimal domain.Scan for tests.
func sampleScan(userID domain.UserID, rawurl string) domain.Scan {
	id := uuid.New()

	return domain.Scan{
		ID:        domain.ScanID(id),
		UserID:    userID,
		URL:       rawurl,
		Status:    domain.ScanStatusCompleted,
		Attempts:  1,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
}
