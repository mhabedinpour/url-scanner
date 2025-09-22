package v1handler_test

import (
	"context"
	"net/url"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"scanner/internal/api/handler/v1handler"
	"scanner/internal/api/specs/v1specs"
	mockscanner "scanner/internal/scanner/mock"
	"scanner/pkg/domain"
)

func Test_toV1Result_Mapping(t *testing.T) {
	in := &domain.ScanResult{
		Page: &struct {
			URL     string `json:"url"`
			Domain  string `json:"domain"`
			IP      string `json:"ip"`
			ASN     string `json:"asn"`
			Country string `json:"country"`
			Server  string `json:"server"`
		}{
			URL:     "https://example.com/index.html",
			Domain:  "example.com",
			IP:      "93.184.216.34",
			ASN:     "AS15133",
			Country: "US",
			Server:  "nginx",
		},
		Verdict: &struct {
			Malicious bool `json:"malicious"`
			Score     int  `json:"score"`
		}{
			Malicious: true,
			Score:     42,
		},
		Stats: &struct {
			Malicious int `json:"malicious"`
		}{
			Malicious: 3,
		},
	}

	out := v1handler.DomainScanResultToV1Specs(in)

	require.True(t, out.Page.URL.IsSet(), "expected page.url to be set")
	require.Equal(t, "https://example.com/index.html", out.Page.URL.Value.String())
	require.Equal(t, "example.com", out.Page.Domain.Value)
	require.Equal(t, "93.184.216.34", out.Page.IP.Value)
	require.Equal(t, "AS15133", out.Page.Asn.Value)
	require.Equal(t, "US", out.Page.Country.Value)
	require.Equal(t, "nginx", out.Page.Server.Value)

	require.True(t, out.Verdicts.Malicious.Value, "malicious expected true")
	require.Equal(t, 42, out.Verdicts.Score.Value)

	require.Equal(t, 3, out.Stats.Malicious.Value)
}

func Test_toV1Result_OptionalFieldsUnset_WhenEmpty(t *testing.T) {
	// Supply empty result to ensure no optional fields are set
	in := &domain.ScanResult{}
	out := v1handler.DomainScanResultToV1Specs(in)
	// page is zero; Page fields should be zero-values
	require.Equal(t, v1specs.ScanResultPage{}, out.Page, "expected empty page struct")
	require.False(t, out.Verdicts.Malicious.IsSet(), "malicious should not be set by default")
	require.False(t, out.Stats.Malicious.IsSet(), "stats.malicious should not be set by default")
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
	require.NoError(t, err)
	require.Equal(t, id, out.ID)
	require.Equal(t, "https://example.org/x", out.URL.String())
	require.Equal(t, v1specs.ScanStatus(domain.ScanStatusCompleted), out.Status)
	require.Equal(t, 2, out.Attempts)
	require.True(t, out.CreatedAt.Equal(now), "createdAt mismatch")
	require.True(t, out.UpdatedAt.IsSet(), "updatedAt should be set")
}

func Test_toV1Specs_InvalidURL_Error(t *testing.T) {
	in := &domain.Scan{URL: "://bad url"}
	_, err := v1handler.DomainScanToV1Specs(in)
	require.Error(t, err)
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
	require.NoError(t, err)
	require.NotNil(t, res)
	got := res.(*v1specs.Scan)
	require.Equal(t, "https://e.com", got.URL.String())
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
	require.NoError(t, err)
	require.IsType(t, &v1specs.DeleteScanNoContent{}, res)
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
	require.NoError(t, err)
	got := res.(*v1specs.Scan)
	require.Equal(t, "https://abc.xyz", got.URL.String())
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
	require.NoError(t, err)
	lst := res.(*v1specs.ScanList)
	require.Len(t, lst.Items, 2)
	require.True(t, lst.NextCursor.IsSet())
	require.Equal(t, next, lst.NextCursor.Value)
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
	require.NoError(t, err)
	lst := res.(*v1specs.ScanList)
	require.Empty(t, lst.Items)
	require.False(t, lst.NextCursor.IsSet(), "next cursor should be unset when empty")
}

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
