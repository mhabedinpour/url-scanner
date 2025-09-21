package scanner

import (
	"fmt"
	"net"
	"net/url"
	"path"
	"sort"
	"strings"
)

// NormalizeURL returns a canonical, normalized representation of a URL string.
//
// The normalization rules are intentionally strict and opinionated to help with
// URL de-duplication in the scanner:
//   - Lower-case the scheme and host
//   - Ensure path is present; empty path becomes "/"
//   - Clean the path (resolve dot-segments, collapse duplicate slashes)
//   - Remove a trailing slash (except for the root path "/")
//   - Drop default ports (http:80, https:443), keep non-default ports
//   - Sort query parameters by key and by value for stable ordering
//   - Remove the fragment
//
// If the input cannot be parsed as a URL, an error is returned.
func NormalizeURL(raw string) (string, error) {
	u, err := url.Parse(raw)
	if err != nil {
		return "", fmt.Errorf("could not parse URL: %w", err)
	}

	// lowercase scheme
	u.Scheme = strings.ToLower(u.Scheme)

	// if no path, make it "/"
	if u.Path == "" {
		u.Path = "/"
	}

	// clean path (removes dot-segments, duplicate slashes)
	cleaned := path.Clean(u.Path)

	// keep a leading slash for absolute URLs
	if !strings.HasPrefix(cleaned, "/") {
		cleaned = "/" + cleaned
	}
	u.Path = cleaned

	// remove trailing slash (but not for root)
	if u.Path != "/" && strings.HasSuffix(u.Path, "/") {
		u.Path = strings.TrimRight(u.Path, "/")
	}

	// lowercase host and drop default ports
	host := strings.ToLower(u.Host)
	port := ""
	if ph, pp, err := net.SplitHostPort(host); err == nil {
		host, port = ph, pp
	} // else: might be a host without explicit port or IPv6 without port
	// remove default ports for common schemes
	if port != "" {
		if (u.Scheme == "http" && port == "80") || (u.Scheme == "https" && port == "443") {
			u.Host = host
		} else {
			u.Host = net.JoinHostPort(host, port)
		}
	} else {
		u.Host = host
	}

	// sort query params (keys and values)
	if u.RawQuery != "" {
		q := u.Query()
		// sort each value slice
		for k := range q {
			sort.Strings(q[k])
		}
		// url.Values.Encode() sorts keys lexicographically
		u.RawQuery = q.Encode()
	}

	// remove fragment
	u.Fragment = ""

	return u.String(), nil
}
