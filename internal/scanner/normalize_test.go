package scanner_test

import (
	"scanner/internal/scanner"
	"testing"
)

func TestNormalizeURL(t *testing.T) {
	cases := []struct {
		name string
		in   string
		out  string
		ok   bool
	}{
		{
			name: "lowercase scheme and host; add root path",
			in:   "HTTP://Example.COM",
			out:  "http://example.com/",
			ok:   true,
		},
		{
			name: "remove default http port",
			in:   "http://example.com:80/path",
			out:  "http://example.com/path",
			ok:   true,
		},
		{
			name: "remove default https port",
			in:   "https://example.com:443/",
			out:  "https://example.com/",
			ok:   true,
		},
		{
			name: "keep non-default port",
			in:   "http://example.com:8080/",
			out:  "http://example.com:8080/",
			ok:   true,
		},
		{
			name: "clean path and drop trailing slash",
			in:   "http://example.com//a/./b/../c/",
			out:  "http://example.com/a/c",
			ok:   true,
		},
		{
			name: "sort query keys and values",
			in:   "http://EXAMPLE.com/path?b=2&a=2&a=1",
			// note: entire URL lowercased, values and keys too
			out: "http://example.com/path?a=1&a=2&b=2",
			ok:  true,
		},
		{
			name: "remove fragment",
			in:   "https://example.com/path?x=1#Section-2",
			out:  "https://example.com/path?x=1",
			ok:   true,
		},
		{
			name: "ipv6 host with port (non-default kept)",
			in:   "http://[2001:db8::1]:8080/a",
			out:  "http://[2001:db8::1]:8080/a",
			ok:   true,
		},
		{
			name: "already normalized",
			in:   "https://example.com/foo?bar=1&baz=2",
			out:  "https://example.com/foo?bar=1&baz=2",
			ok:   true,
		},
		{
			name: "invalid url returns error",
			in:   "http://exa mple.com",
			out:  "",
			ok:   false,
		},
	}

	for _, tc := range cases {
		got, err := scanner.NormalizeURL(tc.in)
		if tc.ok {
			if err != nil {
				t.Fatalf("%s: unexpected error: %v", tc.name, err)
			}
			if got != tc.out {
				t.Errorf("%s: got %q, want %q", tc.name, got, tc.out)
			}
		} else if err == nil {
			t.Errorf("%s: expected error, got none (result %q)", tc.name, got)
		}
	}
}
