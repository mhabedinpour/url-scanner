// Package urlscanio provides a urlscanner.Client implementation backed by the
// public urlscan.io API.
package urlscanio

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"scanner/pkg/domain"
	"scanner/pkg/serrors"
	"scanner/pkg/urlscanner"
	"strconv"
	"strings"
	"time"
)

// Client talks to the urlscan.io REST API and fulfills the urlscanner.Client
// interface. It is safe for concurrent use.
type Client struct {
	httpClient *http.Client // httpClient performs HTTP requests to urlscan.io
	token      string       // token is the API key for urlscan.io
}

// ParseRateLimit extracts urlscan.io rate‑limit information from the HTTP
// response headers and converts it into a urlscanner.RateLimitStatus.
func ParseRateLimit(h http.Header) (urlscanner.RateLimitStatus, error) {
	atoi := func(s string) int {
		if s == "" {
			return 0
		}
		if n, err := strconv.Atoi(s); err == nil {
			return n
		}

		return 0
	}
	limit := atoi(h.Get("X-Rate-Limit-Limit"))
	remaining := atoi(h.Get("X-Rate-Limit-Remaining"))

	resetStr := h.Get("X-Rate-Limit-Reset")
	resetAt, err := time.Parse(time.RFC3339Nano, resetStr)
	if err != nil {
		return urlscanner.RateLimitStatus{}, fmt.Errorf("could not parse reset at: %w", err)
	}

	return urlscanner.RateLimitStatus{Limit: limit, Remaining: remaining, ResetAt: resetAt}, nil
}

// SubmitURL submits the provided URL to urlscan.io for scanning.
// It returns the provider job identifier, the parsed rate‑limit status from
// the response headers, and an error if the submission failed.
func (c *Client) SubmitURL(ctx context.Context, URL string) (urlscanner.SubmitRes, urlscanner.RateLimitStatus, error) {
	// https://docs.urlscan.io/apis/urlscan-openapi/scanning/submitscan
	type submitReq struct {
		URL        string `json:"url"`
		Visibility string `json:"visibility,omitempty"`
	}
	bodyBytes, err := json.Marshal(submitReq{URL: URL, Visibility: "public"})
	if err != nil {
		return urlscanner.SubmitRes{}, urlscanner.RateLimitStatus{}, fmt.Errorf("could not marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx,
		http.MethodPost,
		"https://urlscan.io/api/v1/scan",
		strings.NewReader(string(bodyBytes)))
	if err != nil {
		return urlscanner.SubmitRes{}, urlscanner.RateLimitStatus{}, fmt.Errorf("could not create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Api-Key", c.token)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return urlscanner.SubmitRes{}, urlscanner.RateLimitStatus{}, fmt.Errorf("could not send request: %w", err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	rl, err := ParseRateLimit(resp.Header)
	if err != nil {
		return urlscanner.SubmitRes{}, rl, fmt.Errorf("could not parse rate limit: %w", err)
	}
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		return urlscanner.SubmitRes{}, rl, fmt.Errorf("could not read response body: %w", err)
	}
	if resp.StatusCode == http.StatusTooManyRequests {
		return urlscanner.SubmitRes{},
			rl,
			serrors.With(serrors.ErrRateLimited, "rate limited: %s", strings.TrimSpace(string(b)))
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return urlscanner.SubmitRes{}, rl, fmt.Errorf("submit failed: %s", strings.TrimSpace(string(b)))
	}

	// successful
	var submitResp struct {
		UUID string `json:"uuid"`
	}
	if err := json.Unmarshal(b, &submitResp); err != nil {
		return urlscanner.SubmitRes{}, rl, fmt.Errorf("could not decode response: %w", err)
	}

	return urlscanner.SubmitRes{ID: submitResp.UUID}, rl, nil
}

// Result fetches and decodes the scan result for the given scanID from
// urlscan.io. It returns domain.ScanResult when available, ErrNotFound when the
// scan is not yet available or does not exist, or another error on failure.
func (c *Client) Result(ctx context.Context, scanID string) (*domain.ScanResult, error) {
	// https://docs.urlscan.io/apis/urlscan-openapi/scanning/resultapi
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://urlscan.io/api/v1/result/"+scanID, nil)
	if err != nil {
		return nil, fmt.Errorf("could not create request: %w", err)
	}
	req.Header.Set("Api-Key", c.token)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("could not send request: %w", err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	b, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("could not read response body: %w", err)
	}
	if resp.StatusCode == http.StatusNotFound {
		return nil, serrors.With(serrors.ErrNotFound, "result not found")
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("get result failed: %s", strings.TrimSpace(string(b)))
	}

	// successful
	var rs struct {
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
	}
	if err := json.Unmarshal(b, &rs); err != nil {
		return nil, fmt.Errorf("could not decode response: %w", err)
	}
	out := &domain.ScanResult{}
	out.Page = &struct {
		URL     string `json:"url"`
		Domain  string `json:"domain"`
		IP      string `json:"ip"`
		ASN     string `json:"asn"`
		Country string `json:"country"`
		Server  string `json:"server"`
	}{
		URL:     rs.Page.URL,
		Domain:  rs.Page.Domain,
		IP:      rs.Page.IP,
		ASN:     rs.Page.ASN,
		Country: rs.Page.Country,
		Server:  rs.Page.Server,
	}
	out.Verdict = &struct {
		Malicious bool `json:"malicious"`
		Score     int  `json:"score"`
	}{
		Malicious: rs.Verdicts.Overall.Malicious,
		Score:     rs.Verdicts.Overall.Score,
	}
	out.Stats = &struct {
		Malicious int `json:"malicious"`
	}{
		Malicious: rs.Stats.Malicious,
	}

	return out, nil
}

// Ensure Client conforms to the urlscanner.Client interface at compile time.
var _ urlscanner.Client = (*Client)(nil)

// New constructs a Client that uses the provided http.Client and API token
// to interact with the urlscan.io API.
func New(httpClient *http.Client, token string) *Client {
	return &Client{
		httpClient: httpClient,
		token:      token,
	}
}
