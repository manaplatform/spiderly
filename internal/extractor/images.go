// internal/extractor/images.go
package extractor

import (
	"net/url"
	"strings"

	"github.com/PuerkitoBio/goquery"
)

var imageSelectors = []string{
	".product-image img",
	".product-gallery img",
	"[itemprop='image']",
	".woocommerce-product-gallery img",
	".product-images img",
	"#product-images img",
}

// imageAttrs are the attributes checked for image URLs, in priority order.
var imageAttrs = []string{"src", "data-src", "data-lazy-src", "data-original"}

// validImageExts used for URL validation.
var validImageExts = []string{".jpg", ".jpeg", ".png", ".gif", ".webp", ".svg", ".avif"}

// extractImages collects product image URLs from the DOM.
func extractImages(doc *goquery.Selection, baseURL string) []string {
	var images []string
	seen := make(map[string]bool)

	for _, sel := range imageSelectors {
		doc.Find(sel).Each(func(i int, s *goquery.Selection) {
			for _, attr := range imageAttrs {
				src, exists := s.Attr(attr)
				if !exists || src == "" {
					continue
				}
				resolved := resolveURL(src, baseURL)
				if resolved == "" || seen[resolved] {
					continue
				}
				if isValidImageURL(resolved) {
					images = append(images, resolved)
					seen[resolved] = true
				}
			}
		})
	}

	return images
}

// resolveURL resolves a possibly-relative href against a base URL.
func resolveURL(href, baseURL string) string {
	base, err := url.Parse(baseURL)
	if err != nil {
		return href
	}
	ref, err := url.Parse(href)
	if err != nil {
		return href
	}
	return base.ResolveReference(ref).String()
}

// isValidImageURL checks whether a URL looks like an image resource.
func isValidImageURL(rawURL string) bool {
	lower := strings.ToLower(rawURL)

	for _, ext := range validImageExts {
		if strings.Contains(lower, ext) {
			return true
		}
	}

	// Heuristic fallback for CDN paths without extensions
	return strings.Contains(lower, "image") ||
		strings.Contains(lower, "photo") ||
		strings.Contains(lower, "product") ||
		strings.Contains(lower, "cdn")
}
