// internal/extractor/result.go
package extractor

import (
	"fmt"
	"spiderly/internal/models"

	"github.com/PuerkitoBio/goquery"
)

// ExtractionResult wraps a ProductInfo with metadata about how it was extracted.
type ExtractionResult struct {
	Product    *models.ProductInfo `json:"product"`
	Source     string              `json:"source"`     // "json-ld", "opengraph", "dom", "mixed"
	Confidence float64            `json:"confidence"` // 0.0 – 1.0
}

// ─── Apply methods (layered fill — never overwrite non-empty fields) ─────────

// applyJSONLD fills product fields from JSON-LD structured data.
func (r *ExtractionResult) applyJSONLD(ld *jsonLDProduct) {
	if ld == nil {
		return
	}
	p := r.Product

	if p.Name == "" {
		p.Name = ld.Name
	}
	if p.Description == "" {
		p.Description = truncate(ld.Description, 1000)
	}
	if p.SKU == "" {
		p.SKU = ld.SKU
	}
	if p.Brand == "" && ld.Brand != nil {
		p.Brand = ld.Brand.Name
	}

	// Price and currency live inside Offers
	if ld.Offers != nil {
		if p.Price == 0 {
			p.Price = toFloat64(ld.Offers.Price)
		}
		if p.Currency == "" {
			p.Currency = ld.Offers.PriceCurrency
		}
		if p.Availability == "" {
			p.Availability = ld.Offers.Availability
		}
	}

	// Image can be a string or []string in JSON-LD
	if len(p.Images) == 0 {
		p.Images = extractJSONLDImages(ld.Image)
	}
}

// applyOpenGraph fills product fields from Open Graph meta tags.
func (r *ExtractionResult) applyOpenGraph(og *ogData) {
	if og == nil {
		return
	}
	p := r.Product

	if p.Name == "" && og.Title != "" {
		p.Name = og.Title
	}
	if p.Description == "" && og.Description != "" {
		p.Description = truncate(og.Description, 1000)
	}
	if len(p.Images) == 0 && og.Image != "" {
		p.Images = []string{og.Image}
	}

	// Track mixed source
	if r.Source != "" && r.Source != "opengraph" {
		r.Source = "mixed"
	} else if r.Source == "" {
		r.Source = "opengraph"
	}
}

// applyDOM fills remaining empty fields using DOM selectors.
func (r *ExtractionResult) applyDOM(doc *goquery.Selection, pageURL string, opts ProductOptions) {
	p := r.Product

	if p.Name == "" {
		p.Name = firstMatchText(doc, DefaultNameSelectors)
	}
	if p.Price == 0 {
		p.Price, p.Currency = extractPrice(doc)
	}
	if p.Description == "" {
		p.Description = extractDescription(doc)
	}
	if p.SKU == "" {
		p.SKU = extractSKU(doc)
	}
	if p.Brand == "" {
		p.Brand = extractBrand(doc)
	}
	if p.Availability == "" {
		p.Availability = extractAvailability(doc)
	}
	if opts.ExtractSpecs && len(p.Specs) == 0 {
		p.Specs = extractSpecifications(doc)
	}
	if opts.ExtractImages && len(p.Images) == 0 {
		p.Images = extractImages(doc, pageURL)
	}

	// Track source
	if r.Source != "" && r.Source != "dom" {
		r.Source = "mixed"
	} else if r.Source == "" {
		r.Source = "dom"
	}
}

// ─── Confidence scoring ──────────────────────────────────────────────────────

// calculateConfidence assigns a 0.0–1.0 score based on how many fields
// were successfully extracted and the data source quality.
func (r *ExtractionResult) calculateConfidence() {
	p := r.Product
	score := 0.0
	total := 0.0

	fields := []struct {
		filled bool
		weight float64
	}{
		{p.Name != "", 0.25},
		{p.Price > 0, 0.20},
		{p.Currency != "", 0.05},
		{p.Description != "", 0.15},
		{p.SKU != "", 0.05},
		{p.Brand != "", 0.05},
		{p.Availability != "", 0.05},
		{len(p.Images) > 0, 0.10},
		{len(p.Specs) > 0, 0.10},
	}

	for _, f := range fields {
		total += f.weight
		if f.filled {
			score += f.weight
		}
	}

	if total > 0 {
		score = score / total
	}

	switch r.Source {
	case "json-ld":
		score = clampF(score+0.10, 0, 1)
	case "mixed":
		score = clampF(score+0.05, 0, 1)
	}

	r.Confidence = score
}

// ─── Utilities ───────────────────────────────────────────────────────────────

// extractJSONLDImages handles the JSON-LD image field which can be a string,
// a []string, or a []interface{}.
func extractJSONLDImages(v interface{}) []string {
	if v == nil {
		return nil
	}
	switch img := v.(type) {
	case string:
		if img != "" {
			return []string{img}
		}
	case []interface{}:
		var out []string
		for _, item := range img {
			if s, ok := item.(string); ok && s != "" {
				out = append(out, s)
			}
		}
		return out
	case []string:
		return img
	}
	return nil
}

// toFloat64 converts an interface{} (string or number) to float64.
func toFloat64(v interface{}) float64 {
	if v == nil {
		return 0
	}
	switch n := v.(type) {
	case float64:
		return n
	case int:
		return float64(n)
	case string:
		return parsePrice(n)
	default:
		s := fmt.Sprintf("%v", n)
		return parsePrice(s)
	}
}

func truncate(s string, maxLen int) string {
	runes := []rune(s)
	if len(runes) > maxLen {
		return string(runes[:maxLen])
	}
	return s
}

func clampF(v, min, max float64) float64 {
	if v < min {
		return min
	}
	if v > max {
		return max
	}
	return v
}
