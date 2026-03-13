// internal/extractor/product_fields.go
package extractor

import (
	"strings"

	"github.com/PuerkitoBio/goquery"
)

// Default selectors for each product field, ordered by reliability.
var (
	nameSelectors = []string{
		"h1.product-title",
		"h1.product_title",
		"h1.product-name",
		"[itemprop='name']",
		".product-title h1",
		".product_title",
		".product-name",
		"h1",
	}

	descriptionSelectors = []string{
		"[itemprop='description']",
		".product-description",
		".product-short-description",
		".description",
		"#product-description",
	}

	skuSelectors = []string{
		"[itemprop='sku']",
		".sku",
		".product-sku",
		"[data-sku]",
	}

	brandSelectors = []string{
		"[itemprop='brand']",
		".brand",
		".product-brand",
		"[data-brand]",
	}

	availabilitySelectors = []string{
		"[itemprop='availability']",
		".availability",
		".stock-status",
		".in-stock",
		".out-of-stock",
	}
)

// extractProductName returns the product name from DOM selectors.
func extractProductName(doc *goquery.Selection) string {
	return firstMatchText(doc, nameSelectors)
}

// extractDescription returns the product description, capped at 1000 chars.
func extractDescription(doc *goquery.Selection) string {
	desc := firstMatchText(doc, descriptionSelectors)
	if len(desc) > 1000 {
		desc = desc[:1000]
	}
	return desc
}

// extractSKU returns the product SKU from DOM selectors.
func extractSKU(doc *goquery.Selection) string {
	for _, sel := range skuSelectors {
		el := doc.Find(sel).First()
		if el.Length() == 0 {
			continue
		}
		// Prefer content or data-sku attributes
		if sku, exists := el.Attr("content"); exists && sku != "" {
			return sku
		}
		if sku, exists := el.Attr("data-sku"); exists && sku != "" {
			return sku
		}
		if text := strings.TrimSpace(el.Text()); text != "" {
			return text
		}
	}
	return ""
}

// extractBrand returns the product brand from DOM selectors.
func extractBrand(doc *goquery.Selection) string {
	for _, sel := range brandSelectors {
		el := doc.Find(sel).First()
		if el.Length() == 0 {
			continue
		}
		if brand, exists := el.Attr("content"); exists && brand != "" {
			return brand
		}
		if text := strings.TrimSpace(el.Text()); text != "" {
			return text
		}
	}
	return ""
}

// extractAvailability returns the product availability status.
func extractAvailability(doc *goquery.Selection) string {
	for _, sel := range availabilitySelectors {
		el := doc.Find(sel).First()
		if el.Length() == 0 {
			continue
		}
		if avail, exists := el.Attr("content"); exists && avail != "" {
			return avail
		}
		if avail, exists := el.Attr("href"); exists && strings.Contains(avail, "schema.org") {
			return avail
		}
		if text := strings.TrimSpace(el.Text()); text != "" {
			return text
		}
	}
	return ""
}
