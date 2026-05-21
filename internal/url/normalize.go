// Package url provides URL normalization and an HTTP fetcher used by the
// deterministic pipeline commands.
//
// Behavior mirrors the Python prototype's normalize_url (in
// references/scripts/fetch-stub-content.py) and its YouTube video-ID
// extractor (in references/scripts/fetch-youtube-transcript.py).
package url

import "regexp"

// trackingParamRE matches tracking parameters in a URL query string,
// reproducing references/scripts/fetch-stub-content.py:normalize_url:
//
//	r'[?&](utm_\w+|ref|fbclid|gclid|mc_[a-z]+)=[^&]*'
//
// Each match removes a single `?key=value` or `&key=value` segment.
var trackingParamRE = regexp.MustCompile(`[?&](utm_\w+|ref|fbclid|gclid|mc_[a-z]+)=[^&]*`)

// leftoverAmpRE collapses `?&` left behind after the first tracking
// param is stripped from the front of the query string. Same regex as
// the prototype.
var leftoverAmpRE = regexp.MustCompile(`\?&`)

// trailingQuestionRE strips a trailing `?` left behind when all query
// parameters were tracking. Same regex as the prototype.
var trailingQuestionRE = regexp.MustCompile(`\?$`)

// Normalize returns a deduplication-friendly canonical form of url.
// Mirrors references/scripts/fetch-stub-content.py:normalize_url
// byte-for-byte:
//
//  1. Strip leading/trailing whitespace.
//  2. Remove tracking parameters: utm_*, ref, fbclid, gclid, mc_*.
//  3. Collapse `?&` (leftover after stripping the first parameter).
//  4. Strip a trailing `?`.
//  5. Strip the URL fragment (`#…`).
//  6. Strip a trailing slash.
func Normalize(url string) string {
	url = trimSpace(url)
	url = trackingParamRE.ReplaceAllString(url, "")
	url = leftoverAmpRE.ReplaceAllString(url, "?")
	url = trailingQuestionRE.ReplaceAllString(url, "")
	if i := indexByte(url, '#'); i >= 0 {
		url = url[:i]
	}
	for len(url) > 0 && url[len(url)-1] == '/' {
		url = url[:len(url)-1]
	}
	return url
}

// Small wrappers used in lieu of importing strings/bytes to keep the
// dependency surface visibly minimal here. Behavior is identical to
// strings.TrimSpace and strings.IndexByte.
func trimSpace(s string) string {
	start := 0
	for start < len(s) && isSpace(s[start]) {
		start++
	}
	end := len(s)
	for end > start && isSpace(s[end-1]) {
		end--
	}
	return s[start:end]
}

func isSpace(b byte) bool {
	switch b {
	case ' ', '\t', '\n', '\v', '\f', '\r':
		return true
	}
	return false
}

func indexByte(s string, c byte) int {
	for i := 0; i < len(s); i++ {
		if s[i] == c {
			return i
		}
	}
	return -1
}
