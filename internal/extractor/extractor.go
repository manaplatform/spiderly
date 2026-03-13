// internal/extractor/extractor.go
package extractor

import (
	"spiderly/internal/models"

	"github.com/PuerkitoBio/goquery"
)

// ProductOptions configures product extraction behavior.
type ProductOptions struct {
	ExtractSpecs  bool
	ExtractImages bool
}

// ExtractProduct extracts product information from a page.
// It tries JSON-LD first, then falls back to DOM scraping.
func ExtractProduct(doc *goquery.Selection, pageURL string, opts ProductOptions) *ExtractionResult {
	result := &ExtractionResult{
		Product: &models.ProductInfo{},
	}

	// 1. Try JSON-LD (highest confidence)
	if ld := extractFromJSONLD(doc); ld != nil {
		result.Source = "json-ld"
		result.applyJSONLD(ld)
	}

	// 2. Fill missing fields from Open Graph
	og := extractFromOpenGraph(doc)
	result.applyOpenGraph(og)

	// 3. Fill remaining gaps from DOM selectors
	result.applyDOM(doc, pageURL, opts)

	// No product name at all — nothing useful found
	if result.Product.Name == "" {
		return nil
	}

	result.calculateConfidence()
	return result
}

// ExtractMainContent extracts the primary text content from a page.
func ExtractMainContent(doc *goquery.Selection) string {
	return extractMainContent(doc)
}
