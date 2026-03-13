// internal/crawler/product.go
package crawler

import (
	"regexp"
	"strings"
)

// Common URL path segments that indicate a product page.
var defaultProductIndicators = []string{
	"/product/",
	"/products/",
	"/item/",
	"/items/",
	"/p/",
	"/dp/",
	"/gp/product/",
	"/shop/",
	"/listing/",
}

// defaultProductPattern is a fallback regex when Config.ProductPattern is nil.
// It matches any URL whose path contains one of the indicator segments.
var defaultProductPattern = func() *regexp.Regexp {
	escaped := make([]string, len(defaultProductIndicators))
	for i, seg := range defaultProductIndicators {
		escaped[i] = regexp.QuoteMeta(seg)
	}
	return regexp.MustCompile("(?i)(" + strings.Join(escaped, "|") + ")")
}()

