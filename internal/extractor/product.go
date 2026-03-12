// internal/extractor/extractor.go
package extractor

import (
	"regexp"
	"strings"

	"spiderly/internal/models"

	"github.com/PuerkitoBio/goquery"
)

// ProductOptions configures product extraction behavior
type ProductOptions struct {
	ExtractSpecs  bool
	ExtractImages bool
}

// ExtractProduct extracts product information from a page
func ExtractProduct(doc *goquery.Selection, pageURL string, opts ProductOptions) *models.ProductInfo {
	product := &models.ProductInfo{}

	// Extract product name
	product.Name = extractProductName(doc)
	if product.Name == "" {
		return nil // No product found
	}

	// Extract price
	product.Price, product.Currency = extractPrice(doc)

	// Extract description
	product.Description = extractDescription(doc)

	// Extract SKU
	product.SKU = extractSKU(doc)

	// Extract brand
	product.Brand = extractBrand(doc)

	// Extract availability
	product.Availability = extractAvailability(doc)

	// Optional: Extract specifications
	if opts.ExtractSpecs {
		product.Specs = extractSpecifications(doc)
	}

	// Optional: Extract images
	if opts.ExtractImages {
		product.Images = extractImages(doc, pageURL)
	}

	return product
}

// ExtractMainContent extracts the primary text content from a page
func ExtractMainContent(doc *goquery.Selection) string {
	// Clone to avoid modifying original
	clone := doc.Clone()

	// Remove script, style, nav, footer, header elements
	clone.Find("script, style, nav, footer, header, aside, .sidebar, .menu, .navigation").Remove()

	// Try to find main content area
	var content string

	// Priority order for content containers
	selectors := []string{
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

	for _, sel := range selectors {
		if found := clone.Find(sel); found.Length() > 0 {
			content = strings.TrimSpace(found.First().Text())
			if len(content) > 100 {
				break
			}
		}
	}

	// Fallback to body
	if content == "" {
		content = strings.TrimSpace(clone.Find("body").Text())
	}

	// Clean up whitespace
	content = cleanWhitespace(content)

	// Limit length
	if len(content) > 50000 {
		content = content[:50000]
	}

	return content
}

// ─────────────────────────────────────────────────────────────────────────────
//  Helper Functions
// ─────────────────────────────────────────────────────────────────────────────

func cleanWhitespace(s string) string {
	re := regexp.MustCompile(`\s+`)
	return strings.TrimSpace(re.ReplaceAllString(s, " "))
}

func extractProductName(doc *goquery.Selection) string {
	// Try common product name selectors
	selectors := []string{
		"h1.product-title",
		"h1.product_title",
		"h1.product-name",
		"[itemprop='name']",
		".product-title h1",
		".product_title",
		".product-name",
		"h1",
	}

	for _, sel := range selectors {
		if name := strings.TrimSpace(doc.Find(sel).First().Text()); name != "" {
			return name
		}
	}

	return ""
}

func extractPrice(doc *goquery.Selection) (float64, string) {
	// Common price selectors
	selectors := []string{
		"[itemprop='price']",
		".price",
		".product-price",
		".current-price",
		".sale-price",
		"[data-price]",
	}

	for _, sel := range selectors {
		el := doc.Find(sel).First()
		if el.Length() > 0 {
			// Try content attribute first
			if priceStr, exists := el.Attr("content"); exists {
				if price := parsePrice(priceStr); price > 0 {
					currency := extractCurrency(doc)
					return price, currency
				}
			}

			// Try text content
			priceText := el.Text()
			if price := parsePrice(priceText); price > 0 {
				currency := extractCurrencyFromText(priceText)
				if currency == "" {
					currency = extractCurrency(doc)
				}
				return price, currency
			}
		}
	}

	return 0, ""
}

func extractCurrency(doc *goquery.Selection) string {
	// Check meta tag
	if currency, exists := doc.Find("[itemprop='priceCurrency']").Attr("content"); exists {
		return currency
	}

	// Default currencies for Persian sites
	text := doc.Text()
	if strings.Contains(text, "تومان") || strings.Contains(text, "ریال") {
		return "IRR"
	}

	return "USD"
}

func extractCurrencyFromText(text string) string {
	text = strings.ToLower(text)
	if strings.Contains(text, "تومان") {
		return "IRR"
	}
	if strings.Contains(text, "ریال") {
		return "IRR"
	}
	if strings.Contains(text, "$") || strings.Contains(text, "usd") {
		return "USD"
	}
	if strings.Contains(text, "€") || strings.Contains(text, "eur") {
		return "EUR"
	}
	return ""
}

func parsePrice(text string) float64 {
	// Remove common currency symbols and text
	text = strings.ReplaceAll(text, ",", "")
	text = strings.ReplaceAll(text, "٬", "") // Persian comma
	text = strings.ReplaceAll(text, "تومان", "")
	text = strings.ReplaceAll(text, "ریال", "")
	text = strings.ReplaceAll(text, "$", "")
	text = strings.ReplaceAll(text, "€", "")
	text = strings.TrimSpace(text)

	// Extract numeric value
	re := regexp.MustCompile(`[\d.]+`)
	match := re.FindString(text)
	if match == "" {
		return 0
	}

	// Parse the number
	var price float64
	var decimalFound bool
	var decimalPlaces float64 = 1

	for _, r := range match {
		if r >= '0' && r <= '9' {
			if decimalFound {
				decimalPlaces *= 10
				price = price + float64(r-'0')/decimalPlaces
			} else {
				price = price*10 + float64(r-'0')
			}
		} else if r == '.' {
			decimalFound = true
		}
	}

	return price
}

func extractDescription(doc *goquery.Selection) string {
	selectors := []string{
		"[itemprop='description']",
		".product-description",
		".product-short-description",
		".description",
		"#product-description",
	}

	for _, sel := range selectors {
		if desc := strings.TrimSpace(doc.Find(sel).First().Text()); desc != "" {
			if len(desc) > 1000 {
				desc = desc[:1000]
			}
			return desc
		}
	}

	return ""
}

func extractSKU(doc *goquery.Selection) string {
	selectors := []string{
		"[itemprop='sku']",
		".sku",
		".product-sku",
		"[data-sku]",
	}

	for _, sel := range selectors {
		el := doc.Find(sel).First()
		if el.Length() > 0 {
			if sku, exists := el.Attr("content"); exists {
				return sku
			}
			if sku, exists := el.Attr("data-sku"); exists {
				return sku
			}
			return strings.TrimSpace(el.Text())
		}
	}

	return ""
}

func extractBrand(doc *goquery.Selection) string {
	selectors := []string{
		"[itemprop='brand']",
		".brand",
		".product-brand",
		"[data-brand]",
	}

	for _, sel := range selectors {
		el := doc.Find(sel).First()
		if el.Length() > 0 {
			if brand, exists := el.Attr("content"); exists {
				return brand
			}
			return strings.TrimSpace(el.Text())
		}
	}

	return ""
}

func extractAvailability(doc *goquery.Selection) string {
	selectors := []string{
		"[itemprop='availability']",
		".availability",
		".stock-status",
		".in-stock",
		".out-of-stock",
	}

	for _, sel := range selectors {
		el := doc.Find(sel).First()
		if el.Length() > 0 {
			if avail, exists := el.Attr("content"); exists {
				return avail
			}
			if avail, exists := el.Attr("href"); exists && strings.Contains(avail, "schema.org") {
				return avail
			}
			return strings.TrimSpace(el.Text())
		}
	}

	return ""
}

func extractSpecifications(doc *goquery.Selection) map[string]string {
	specs := make(map[string]string)

	// Common spec table patterns
	doc.Find("table.specifications tr, table.specs tr, .product-specs tr, .specifications-table tr").Each(func(i int, s *goquery.Selection) {
		key := strings.TrimSpace(s.Find("th, td:first-child").First().Text())
		value := strings.TrimSpace(s.Find("td:last-child").Text())
		if key != "" && value != "" && key != value {
			specs[key] = value
		}
	})

	// Definition list pattern
	doc.Find("dl.specifications, dl.specs, .product-attributes dl").Each(func(i int, s *goquery.Selection) {
		s.Find("dt").Each(func(j int, dt *goquery.Selection) {
			key := strings.TrimSpace(dt.Text())
			value := strings.TrimSpace(dt.Next().Text())
			if key != "" && value != "" {
				specs[key] = value
			}
		})
	})

	return specs
}

func extractImages(doc *goquery.Selection, baseURL string) []string {
	var images []string
	seen := make(map[string]bool)

	selectors := []string{
		".product-image img",
		".product-gallery img",
		"[itemprop='image']",
		".woocommerce-product-gallery img",
		".product-images img",
		"#product-images img",
	}

	for _, sel := range selectors {
		doc.Find(sel).Each(func(i int, s *goquery.Selection) {
			for _, attr := range []string{"src", "data-src", "data-lazy-src", "data-original"} {
				if src, exists := s.Attr(attr); exists && src != "" {
					src = resolveURL(src, baseURL)
					if !seen[src] && isValidImageURL(src) {
						images = append(images, src)
						seen[src] = true
					}
				}
			}
		})
	}

	return images
}

func resolveURL(href, baseURL string) string {
	if strings.HasPrefix(href, "http://") || strings.HasPrefix(href, "https://") {
		return href
	}
	if strings.HasPrefix(href, "//") {
		return "https:" + href
	}
	if strings.HasPrefix(href, "/") {
		// Parse base URL to get scheme and host
		re := regexp.MustCompile(`^(https?://[^/]+)`)
		if match := re.FindString(baseURL); match != "" {
			return match + href
		}
	}
	return href
}

func isValidImageURL(url string) bool {
	lower := strings.ToLower(url)
	validExts := []string{".jpg", ".jpeg", ".png", ".gif", ".webp", ".svg"}
	for _, ext := range validExts {
		if strings.Contains(lower, ext) {
			return true
		}
	}
	return strings.Contains(lower, "image") || strings.Contains(lower, "photo") || strings.Contains(lower, "product")
}
