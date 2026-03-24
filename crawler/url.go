package crawler

import (
	"net/url"
	"strings"
)

// resolveLink resolves href against baseURL and returns the absolute URL,
// or "" if the link is not an HTTP(S) page (e.g. mailto, #fragment-only).
func resolveLink(href, baseURL string) string {
	base, err := url.Parse(baseURL)
	if err != nil {
		return ""
	}
	ref, err := url.Parse(href)
	if err != nil {
		return ""
	}

	// fragment-only links add no new page
	if ref.Scheme == "" && ref.Host == "" && ref.Path == "" {
		return ""
	}

	abs := base.ResolveReference(ref)

	if abs.Scheme != "http" && abs.Scheme != "https" {
		return ""
	}

	// strip fragment — same page regardless of anchor
	abs.Fragment = ""

	return abs.String()
}

// sameHost reports whether u is on the same host as origin.
func sameHost(u, origin string) bool {
	pu, err := url.Parse(u)
	if err != nil {
		return false
	}
	po, err := url.Parse(origin)
	if err != nil {
		return false
	}
	return strings.EqualFold(pu.Host, po.Host)
}

// normalizeURL strips fragment, .html suffix, and trailing slash for deduplication.
func normalizeURL(u string) string {
	pu, err := url.Parse(u)
	if err != nil {
		return u
	}
	pu.Fragment = ""
	pu.Path = strings.TrimSuffix(pu.Path, ".html")
	return strings.TrimRight(pu.String(), "/")
}
