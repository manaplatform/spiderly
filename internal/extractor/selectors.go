// internal/extractor/selectors.go
package extractor

import (
	"strings"

	"github.com/PuerkitoBio/goquery"
)

// ─── Default selector lists used across extraction files ─────────────────────

// DefaultNameSelectors for product name extraction (priority order).
var DefaultNameSelectors = []string{
	"h1.product-title",
	"h1.product_title",
	"h1.product-name",
	"[itemprop='name']",
	".product-title h1",
	".product_title",
	".product-name",
	// Persian e-commerce
	"h1.product__title",
	"h1.pd-name",
	// Generic fallback
	"h1",
}

// DefaultPriceSelectors for price extraction.
var DefaultPriceSelectors = []string{
	"[itemprop='price']",
	".price .amount",
	".price",
	".product-price",
	".current-price",
	".sale-price",
	"[data-price]",
	// WooCommerce
	".woocommerce-Price-amount",
	// Persian e-commerce
	".product-price-value",
	".pd-price",
}

// DefaultDescriptionSelectors for product description.
var DefaultDescriptionSelectors = []string{
	"[itemprop='description']",
	".product-description",
	".product-short-description",
	".description",
	"#product-description",
	// WooCommerce
	".woocommerce-product-details__short-description",
}

// DefaultSKUSelectors for SKU extraction.
var DefaultSKUSelectors = []string{
	"[itemprop='sku']",
	".sku",
	".product-sku",
	"[data-sku]",
}

// DefaultBrandSelectors for brand extraction.
var DefaultBrandSelectors = []string{
	"[itemprop='brand']",
	"[itemprop='brand'] [itemprop='name']",
	".brand",
	".product-brand",
	"[data-brand]",
}

// DefaultAvailabilitySelectors for stock/availability.
var DefaultAvailabilitySelectors = []string{
	"[itemprop='availability']",
	".availability",
	".stock-status",
	".in-stock",
	".out-of-stock",
	// WooCommerce
	".stock",
}

// DefaultImageSelectors for product images.
var DefaultImageSelectors = []string{
	".product-image img",
	".product-gallery img",
	"[itemprop='image']",
	".woocommerce-product-gallery img",
	".product-images img",
	"#product-images img",
	// Persian e-commerce
	".product-gallery-image img",
	".pd-gallery img",
}

// DefaultContentSelectors for main content area (priority order).
var DefaultContentSelectors = []string{
	"main",
	"article",
	"[role='main']",
	".content",
	".post-content",
	".entry-content",
	".article-content",
	"#content",
	"#main",
	".main-content",
}

// ─── Shared helpers ──────────────────────────────────────────────────────────

// firstMatchText iterates selectors and returns the trimmed text of the first
// non-empty match. Returns "" if nothing is found.
func firstMatchText(doc *goquery.Selection, selectors []string) string {
	for _, sel := range selectors {
		if text := strings.TrimSpace(doc.Find(sel).First().Text()); text != "" {
			return text
		}
	}
	return ""
}

// firstMatchElement returns the first *goquery.Selection that matches any of
// the provided selectors, or nil if none match.
func firstMatchElement(doc *goquery.Selection, selectors []string) *goquery.Selection {
	for _, sel := range selectors {
		el := doc.Find(sel).First()
		if el.Length() > 0 {
			return el
		}
	}
	return nil
}

// firstMatchAttr returns the value of the given attribute from the first
// matching element across selectors. Returns ("", false) if not found.
func firstMatchAttr(doc *goquery.Selection, selectors []string, attr string) (string, bool) {
	for _, sel := range selectors {
		el := doc.Find(sel).First()
		if el.Length() > 0 {
			if val, exists := el.Attr(attr); exists && val != "" {
				return val, true
			}
		}
	}
	return "", false
}
