package urlscanio_test

import (
	"context"
	"io"
	"net/http"
	"scanner/pkg/urlscanner/urlscanio"
	"strings"
	"testing"
	"time"

	"scanner/pkg/serrors"

	"github.com/stretchr/testify/require"
)

// rtFunc allows using a function as an http.RoundTripper.
type rtFunc func(*http.Request) (*http.Response, error)

func (f rtFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func newTestClient(fn rtFunc) *urlscanio.Client {
	return urlscanio.New(&http.Client{Transport: fn}, "test-token")
}

func Test_parseRateLimit_success(t *testing.T) {
	h := http.Header{}
	resetAt := time.Date(2025, 1, 2, 3, 4, 5, 678900000, time.UTC)
	h.Set("X-Rate-Limit-Limit", "120")
	h.Set("X-Rate-Limit-Remaining", "80")
	h.Set("X-Rate-Limit-Reset", resetAt.Format(time.RFC3339Nano))

	rl, err := urlscanio.ParseRateLimit(h)
	require.NoError(t, err)
	require.Equal(t, 120, rl.Limit)
	require.Equal(t, 80, rl.Remaining)
	require.True(t, rl.ResetAt.Equal(resetAt))
}

func Test_parseRateLimit_badTime(t *testing.T) {
	h := http.Header{}
	h.Set("X-Rate-Limit-Limit", "120")
	h.Set("X-Rate-Limit-Remaining", "80")
	h.Set("X-Rate-Limit-Reset", "not-a-time")

	_, err := urlscanio.ParseRateLimit(h)
	require.Error(t, err)
}

func TestClient_SubmitURL_success(t *testing.T) {
	resetAt := time.Now().Add(1 * time.Hour).UTC()
	c := newTestClient(func(r *http.Request) (*http.Response, error) {
		require.Equal(t, http.MethodPost, r.Method)
		require.Equal(t, "urlscan.io", r.URL.Host)
		require.Equal(t, "/api/v1/scan", r.URL.Path)
		require.Equal(t, "application/json", r.Header.Get("Content-Type"))
		require.Equal(t, "test-token", r.Header.Get("Api-Key"))

		h := http.Header{}
		h.Set("X-Rate-Limit-Limit", "100")
		h.Set("X-Rate-Limit-Remaining", "99")
		h.Set("X-Rate-Limit-Reset", resetAt.Format(time.RFC3339Nano))

		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     h,
			Body:       io.NopCloser(strings.NewReader(`{"uuid":"abc-123"}`)),
		}, nil
	})

	res, rl, err := c.SubmitURL(context.Background(), "https://example.com")
	require.NoError(t, err)
	require.Equal(t, "abc-123", res.ID)
	require.Equal(t, 100, rl.Limit)
	require.Equal(t, 99, rl.Remaining)
	require.True(t, rl.ResetAt.Equal(resetAt))
}

func TestClient_SubmitURL_rateLimited429(t *testing.T) {
	resetAt := time.Now().Add(5 * time.Minute).UTC()
	c := newTestClient(func(r *http.Request) (*http.Response, error) {
		h := http.Header{}
		h.Set("X-Rate-Limit-Limit", "100")
		h.Set("X-Rate-Limit-Remaining", "0")
		h.Set("X-Rate-Limit-Reset", resetAt.Format(time.RFC3339Nano))

		return &http.Response{
			StatusCode: http.StatusTooManyRequests,
			Header:     h,
			Body:       io.NopCloser(strings.NewReader("slow down")),
		}, nil
	})

	_, rl, err := c.SubmitURL(context.Background(), "https://example.com")
	require.Error(t, err)
	require.ErrorIs(t, err, serrors.ErrRateLimited, "expected ErrRateLimited kind: %v", err)
	require.Equal(t, 100, rl.Limit)
	require.Equal(t, 0, rl.Remaining)
	require.True(t, rl.ResetAt.Equal(resetAt))
}

func TestClient_SubmitURL_non2xx(t *testing.T) {
	resetAt := time.Now().Add(5 * time.Minute).UTC()
	c := newTestClient(func(r *http.Request) (*http.Response, error) {
		h := http.Header{}
		h.Set("X-Rate-Limit-Limit", "100")
		h.Set("X-Rate-Limit-Remaining", "98")
		h.Set("X-Rate-Limit-Reset", resetAt.Format(time.RFC3339Nano))

		return &http.Response{
			StatusCode: http.StatusBadGateway,
			Header:     h,
			Body:       io.NopCloser(strings.NewReader("upstream bad")),
		}, nil
	})

	_, rl, err := c.SubmitURL(context.Background(), "https://example.com")
	require.Error(t, err)
	require.Contains(t, err.Error(), "upstream bad")
	require.Equal(t, 100, rl.Limit)
	require.Equal(t, 98, rl.Remaining)
	require.True(t, rl.ResetAt.Equal(resetAt))
}

func TestClient_Result_success(t *testing.T) {
	sent := struct {
		Page struct {
			URL     string `json:"url"`
			Domain  string `json:"domain"`
			IP      string `json:"ip"`
			ASN     string `json:"asn"`
			Country string `json:"country"`
			Server  string `json:"server"`
		} `json:"page"`
		Verdicts struct {
			Overall struct {
				Malicious bool `json:"malicious"`
				Score     int  `json:"score"`
			} `json:"overall"`
		} `json:"verdicts"`
		Stats struct {
			Malicious int `json:"malicious"`
		} `json:"stats"`
	}{}
	sent.Page.URL = "https://evil.example"
	sent.Page.Domain = "evil.example"
	sent.Page.IP = "1.2.3.4"
	sent.Page.ASN = "AS12345"
	sent.Page.Country = "ZZ"
	sent.Page.Server = "nginx"
	sent.Verdicts.Overall.Malicious = true
	sent.Verdicts.Overall.Score = 42
	sent.Stats.Malicious = 7

	//nolint: lll
	body := `{"page":{"url":"` + sent.Page.URL + `","domain":"` + sent.Page.Domain + `","ip":"` + sent.Page.IP + `","asn":"` + sent.Page.ASN + `","country":"` + sent.Page.Country + `","server":"` + sent.Page.Server + `"},"verdicts":{"overall":{"malicious":true,"score":42}},"stats":{"malicious":7}}`

	c := newTestClient(func(r *http.Request) (*http.Response, error) {
		require.Equal(t, http.MethodGet, r.Method)
		require.Equal(t, "/api/v1/result/scan-123", r.URL.Path)
		require.Equal(t, "test-token", r.Header.Get("Api-Key"))

		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(body))}, nil
	})

	res, err := c.Result(context.Background(), "scan-123")
	require.NoError(t, err)
	require.NotNil(t, res)
	require.Equal(t, &struct {
		URL     string `json:"url"`
		Domain  string `json:"domain"`
		IP      string `json:"ip"`
		ASN     string `json:"asn"`
		Country string `json:"country"`
		Server  string `json:"server"`
	}{
		URL:     sent.Page.URL,
		Domain:  sent.Page.Domain,
		IP:      sent.Page.IP,
		ASN:     sent.Page.ASN,
		Country: sent.Page.Country,
		Server:  sent.Page.Server,
	}, res.Page)
	require.Equal(t, &struct {
		Malicious bool `json:"malicious"`
		Score     int  `json:"score"`
	}{
		Malicious: true,
		Score:     42,
	}, res.Verdict)
	require.Equal(t, &struct {
		Malicious int `json:"malicious"`
	}{Malicious: 7}, res.Stats)
}

func TestClient_Result_404(t *testing.T) {
	c := newTestClient(func(r *http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: http.StatusNotFound, Body: io.NopCloser(strings.NewReader("not found"))}, nil
	})

	res, err := c.Result(context.Background(), "scan-404")
	require.Error(t, err)
	require.Nil(t, res)
	require.ErrorIs(t, err, serrors.ErrNotFound)
}

func TestClient_Result_non2xx(t *testing.T) {
	c := newTestClient(func(r *http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: http.StatusBadGateway, Body: io.NopCloser(strings.NewReader("bad upstream"))}, nil
	})

	res, err := c.Result(context.Background(), "scan-500")
	require.Error(t, err)
	require.Nil(t, res)
	require.Contains(t, err.Error(), "bad upstream")
}
