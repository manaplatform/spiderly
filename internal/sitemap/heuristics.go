package sitemap

import (
	"net/url"
	"regexp"
	"strings"
)

// SitemapType classifies a sitemap URL by its likely content.
type SitemapType int

const (
	SitemapTypeUnknown SitemapType = iota
	SitemapTypeProduct
	SitemapTypeCategory
	SitemapTypeArticle
	SitemapTypePage
	SitemapTypeImage
	SitemapTypeVideo
	SitemapTypeNews
)

// String returns a human-readable label for the sitemap type.
func (t SitemapType) String() string {
	switch t {
	case SitemapTypeProduct:
		return "product"
	case SitemapTypeCategory:
		return "category"
	case SitemapTypeArticle:
		return "article"
	case SitemapTypePage:
		return "page"
	case SitemapTypeImage:
		return "image"
	case SitemapTypeVideo:
		return "video"
	case SitemapTypeNews:
		return "news"
	default:
		return "unknown"
	}
}

// ─────────────────────────────────────────────
//  Sitemap-Level Classification
// ─────────────────────────────────────────────

// sitemapTypePatterns maps SitemapType to patterns matched against the
// lowercased sitemap URL path and filename.
var sitemapTypePatterns = []struct {
	Type    SitemapType
	Matches []string
}{
	{SitemapTypeProduct, []string{"product", "item", "sku", "catalog", "merch", "shop-sitemap", "goods"}},
	{SitemapTypeCategory, []string{"category", "categor", "collection", "department", "browse", "taxonomy"}},
	{SitemapTypeArticle, []string{"article", "blog", "post", "story", "journal", "editorial"}},
	{SitemapTypePage, []string{"page", "static", "info", "about", "landing"}},
	{SitemapTypeImage, []string{"image", "img", "photo", "gallery", "media"}},
	{SitemapTypeVideo, []string{"video", "vid", "watch", "stream"}},
	{SitemapTypeNews, []string{"news", "press", "release", "headline"}},
}

// GetSitemapType classifies a sitemap URL based on its path/filename.
func GetSitemapType(sitemapURL string) SitemapType {
	parsed, err := url.Parse(strings.ToLower(sitemapURL))
	if err != nil {
		return SitemapTypeUnknown
	}

	path := parsed.Path

	for _, entry := range sitemapTypePatterns {
		for _, keyword := range entry.Matches {
			if strings.Contains(path, keyword) {
				return entry.Type
			}
		}
	}

	return SitemapTypeUnknown
}

// IsProductSitemap returns true if the sitemap URL likely contains product pages.
func IsProductSitemap(sitemapURL string) bool {
	return GetSitemapType(sitemapURL) == SitemapTypeProduct
}

// ─────────────────────────────────────────────
//  URL-Level Product Detection
// ─────────────────────────────────────────────

// productPathSegments are path segments that strongly indicate a product page.
// Matched as exact path segments (between slashes) to avoid false positives
// like "/production/" matching "product".
var productPathSegments = map[string]struct{}{
	"product":  {},
	"products": {},
	"item":     {},
	"items":    {},
	"p":        {},
	"pd":       {},
	"dp":       {}, // Amazon-style
	"gp":       {}, // Amazon-style
	"ip":       {}, // Walmart-style
	"sku":      {},
	"shop":     {},
	"buy":      {},
	"catalog":  {},
}

// productPathPrefixes are path prefixes that suggest product context.
// These require additional signals to confirm.
var productPathPrefixes = []string{
	"/product/",
	"/products/",
	"/item/",
	"/items/",
	"/shop/",
	"/catalog/",
	"/p/",
	"/pd/",
	"/dp/",
	"/gp/product/",
	"/ip/",
}

// antiProductPatterns are path segments that indicate non-product pages
// even when product-like segments are present.
var antiProductPatterns = []string{
	"/category/",
	"/categories/",
	"/collection/",
	"/collections/",
	"/tag/",
	"/tags/",
	"/blog/",
	"/article/",
	"/articles/",
	"/news/",
	"/help/",
	"/support/",
	"/faq/",
	"/about/",
	"/contact/",
	"/policy/",
	"/policies/",
	"/terms/",
	"/privacy/",
	"/account/",
	"/login/",
	"/signup/",
	"/register/",
	"/cart/",
	"/checkout/",
	"/search/",
	"/review/",
	"/reviews/",
	"/compare/",
	"/wishlist/",
	"/sitemap",
}

// reProductID matches common product identifier patterns in URL paths:
//   - Pure numeric IDs: /12345 or -12345
//   - Alphanumeric SKUs: /ABC-12345, /B08N5WRWNW
//   - Slug-with-ID: /blue-widget-12345
var reProductID = regexp.MustCompile(
	`(?:^|[/\-_])(?:[A-Z]{1,3}\d{5,}|\d{5,}|[A-Z0-9]{10,})(?:$|[/?\-_])`,
)

// reSlugPattern matches URL-friendly slugs typical of product pages:
//   - /blue-running-shoes
//   - /mens-cotton-t-shirt-xl
// Requires at least 2 hyphenated words to reduce false positives.
var reSlugPattern = regexp.MustCompile(
	`/[a-z0-9]+(?:-[a-z0-9]+){2,}(?:/|$)`,
)

// IsLikelyProductURL returns true if the URL looks like a product detail page.
// It uses a scoring system combining multiple signals to reduce false positives.
//
// Scoring:
//
//	+3  path contains a known product segment
//	+2  path matches a product prefix
//	+1  URL contains a product-like ID pattern
//	+1  URL has a multi-word slug pattern
//	+1  URL has commerce query params (variant, size, color, etc.)
//	-5  path matches an anti-product pattern (hard veto)
//
// Threshold: score >= 3 is classified as product.
func IsLikelyProductURL(rawURL string) bool {
	parsed, err := url.Parse(strings.ToLower(rawURL))
	if err != nil {
		return false
	}

	path := parsed.Path
	if path == "" || path == "/" {
		return false
	}

	// Hard veto: anti-product patterns
	for _, anti := range antiProductPatterns {
		if strings.Contains(path, anti) {
			return false
		}
	}

	score := 0

	// Signal 1: Exact product path segment match (+3)
	segments := strings.Split(strings.Trim(path, "/"), "/")
	for _, seg := range segments {
		if _, ok := productPathSegments[seg]; ok {
			score += 3
			break
		}
	}

	// Signal 2: Product path prefix match (+2)
	for _, prefix := range productPathPrefixes {
		if strings.HasPrefix(path, prefix) {
			score += 2
			break
		}
	}

	// Signal 3: Product ID pattern in path (+1)
	if reProductID.MatchString(strings.ToUpper(path)) {
		score++
	}

	// Signal 4: Multi-word slug pattern (+1)
	if reSlugPattern.MatchString(path) {
		score++
	}

	// Signal 5: Commerce-related query parameters (+1)
	if hasCommerceParams(parsed.Query()) {
		score++
	}

	return score >= 3
}

// commerceParams are query parameter keys that suggest a product page variant.
var commerceParams = map[string]struct{}{
	"variant":    {},
	"variant_id": {},
	"size":       {},
	"color":      {},
	"colour":     {},
	"sku":        {},
	"model":      {},
	"style":      {},
	"option":     {},
	"qty":        {},
	"quantity":    {},
}

// hasCommerceParams returns true if the query string contains any commerce-related parameters.
func hasCommerceParams(query url.Values) bool {
	for key := range query {
		if _, ok := commerceParams[strings.ToLower(key)]; ok {
			return true
		}
	}
	return false
}

// ─────────────────────────────────────────────
//  Batch Classification
// ─────────────────────────────────────────────

// ClassifiedURL pairs a URL with its classification result.
type ClassifiedURL struct {
	URL       string
	IsProduct bool
	Score     int
}

// ClassifyURLs classifies a batch of URLs, returning results for each.
// This is more efficient than calling IsLikelyProductURL in a loop
// because it reuses the parsed anti-pattern check.
func ClassifyURLs(urls []URL) []ClassifiedURL {
	results := make([]ClassifiedURL, len(urls))

	for i, u := range urls {
		results[i] = ClassifiedURL{
			URL:       u.Loc,
			IsProduct: IsLikelyProductURL(u.Loc),
		}
	}

	return results
}

// FilterProductURLs returns only the URLs classified as likely product pages.
func FilterProductURLs(urls []URL) []URL {
	products := make([]URL, 0, len(urls)/2) // reasonable pre-alloc guess

	for _, u := range urls {
		if IsLikelyProductURL(u.Loc) {
			products = append(products, u)
		}
	}

	return products
}
