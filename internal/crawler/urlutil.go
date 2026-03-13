// internal/crawler/urlutil.go
package crawler

import (
	"net/url"
	"strings"
)

// normalizeAndFilterURL cleans a raw URL and checks it against exclusion rules.
// Returns ("", reason) when the URL should be skipped.
func (c *Crawler) normalizeAndFilterURL(rawURL string) (string, string) {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return "", "invalid URL"
	}

	// Only HTTP(S)
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return "", "non-HTTP(S) scheme"
	}

	// Strip fragment
	parsed.Fragment = ""

	// Lowercase host
	parsed.Host = strings.ToLower(parsed.Host)

	// Remove trailing slash (but keep bare "/")
	if len(parsed.Path) > 1 && strings.HasSuffix(parsed.Path, "/") {
		parsed.Path = strings.TrimRight(parsed.Path, "/")
	}

	// Sort query parameters for consistent dedup
	if parsed.RawQuery != "" {
		parsed.RawQuery = parsed.Query().Encode() // re-encodes in sorted key order
	}

	normalized := parsed.String()

	// Check compiled exclude patterns
	for _, pattern := range c.config.CompiledExcludePatterns {
		if pattern.MatchString(normalized) {
			return "", "matches exclude pattern: " + pattern.String()
		}
	}

	return normalized, ""
}
