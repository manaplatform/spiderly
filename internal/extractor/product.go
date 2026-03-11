package extractor

import (
	"regexp"
	"strconv"
	"strings"

	"github.com/PuerkitoBio/goquery"
)

// ProductData holds extracted product information
type ProductData struct {
	Name         string   `json:"name,omitempty"`
	Brand        string   `json:"brand,omitempty"`
	SKU          string   `json:"sku,omitempty"`
	GTIN         string   `json:"gtin,omitempty"`
	MPN          string   `json:"mpn,omitempty"`
	Price        float64  `json:"price,omitempty"`
	Currency     string   `json:"currency,omitempty"`
	OriginalPrice float64 `json:"original_price,omitempty"`
	Discount     float64  `json:"discount,omitempty"`
	Availability string   `json:"availability,omitempty"`
	InStock      bool     `json:"in_stock"`
	Rating       float64  `json:"rating,omitempty"`
	ReviewCount  int      `json:"review_count,omitempty"`
	Category     string   `json:"category,omitempty"`
	Categories   []string `json:"categories,omitempty"`
	Images       []string `json:"images,omitempty"`
	Description  string   `json:"description,omitempty"`
	Specs        map[string]string `json:"specs,omitempty"`
}

// ProductExtractor extracts product data from HTML
type ProductExtractor struct {
	doc *goquery.Document
}

// NewProductExtractor creates a new product extractor
func NewProductExtractor(doc *goquery.Document) *ProductExtractor {
	return &ProductExtractor{doc: doc}
}

// Extract extracts all product data from the document
func (pe *ProductExtractor) Extract() *ProductData {
	product := &ProductData{
		Specs: make(map[string]string),
	}

	// Try JSON-LD first (most reliable)
	pe.extractJSONLD(product)

	// Then try Open Graph
	pe.extractOpenGraph(product)

	// Then try meta tags
	pe.extractMetaTags(product)

	// Then try common selectors
	pe.extractFromSelectors(product)

	// Extract images
	pe.extractImages(product)

	// Extract specs/attributes
	pe.extractSpecs(product)

	// Calculate discount if we have both prices
	if product.OriginalPrice > 0 && product.Price > 0 && product.Price < product.OriginalPrice {
		product.Discount = ((product.OriginalPrice - product.Price) / product.OriginalPrice) * 100
	}

	return product
}

func (pe *ProductExtractor) extractJSONLD(product *ProductData) {
	pe.doc.Find(`script[type="application/ld+json"]`).Each(func(i int, s *goquery.Selection) {
		jsonText := s.Text()

		// Check if it's a Product schema
		if !strings.Contains(jsonText, `"@type"`) {
			return
		}

		if strings.Contains(jsonText, `"Product"`) || strings.Contains(jsonText, `"product"`) {
			// Extract name
			if product.Name == "" {
				product.Name = extractJSONValue(jsonText, "name")
			}

			// Extract brand
			if product.Brand == "" {
				product.Brand = extractJSONValue(jsonText, "brand")
				if product.Brand == "" {
					// Try nested brand object
					if strings.Contains(jsonText, `"brand":{`) {
						product.Brand = extractNestedJSONValue(jsonText, "brand", "name")
					}
				}
			}

			// Extract SKU
			if product.SKU == "" {
				product.SKU = extractJSONValue(jsonText, "sku")
			}

			// Extract GTIN
			if product.GTIN == "" {
				product.GTIN = extractJSONValue(jsonText, "gtin13")
				if product.GTIN == "" {
					product.GTIN = extractJSONValue(jsonText, "gtin")
				}
			}

			// Extract MPN
			if product.MPN == "" {
				product.MPN = extractJSONValue(jsonText, "mpn")
			}

			// Extract price from offers
			if product.Price == 0 {
				priceStr := extractJSONValue(jsonText, "price")
				if priceStr != "" {
					product.Price = parsePrice(priceStr)
				}
			}

			// Extract currency
			if product.Currency == "" {
				product.Currency = extractJSONValue(jsonText, "priceCurrency")
			}

			// Extract availability
			if product.Availability == "" {
				product.Availability = extractJSONValue(jsonText, "availability")
				product.InStock = isInStock(product.Availability)
			}

			// Extract rating
			if product.Rating == 0 {
				ratingStr := extractJSONValue(jsonText, "ratingValue")
				if ratingStr != "" {
					product.Rating, _ = strconv.ParseFloat(ratingStr, 64)
				}
			}

			// Extract review count
			if product.ReviewCount == 0 {
				reviewStr := extractJSONValue(jsonText, "reviewCount")
				if reviewStr != "" {
					product.ReviewCount, _ = strconv.Atoi(reviewStr)
				}
			}

			// Extract description
			if product.Description == "" {
				product.Description = extractJSONValue(jsonText, "description")
			}
		}
	})
}

func (pe *ProductExtractor) extractOpenGraph(product *ProductData) {
	// og:product:price:amount
	if product.Price == 0 {
		if price, exists := pe.doc.Find(`meta[property="og:product:price:amount"]`).Attr("content"); exists {
			product.Price = parsePrice(price)
		}
		if price, exists := pe.doc.Find(`meta[property="product:price:amount"]`).Attr("content"); exists {
			product.Price = parsePrice(price)
		}
	}

	// og:product:price:currency
	if product.Currency == "" {
		if currency, exists := pe.doc.Find(`meta[property="og:product:price:currency"]`).Attr("content"); exists {
			product.Currency = currency
		}
		if currency, exists := pe.doc.Find(`meta[property="product:price:currency"]`).Attr("content"); exists {
			product.Currency = currency
		}
	}

	// og:product:availability
	if product.Availability == "" {
		if avail, exists := pe.doc.Find(`meta[property="og:availability"]`).Attr("content"); exists {
			product.Availability = avail
			product.InStock = isInStock(avail)
		}
		if avail, exists := pe.doc.Find(`meta[property="product:availability"]`).Attr("content"); exists {
			product.Availability = avail
			product.InStock = isInStock(avail)
		}
	}

	// og:product:brand
	if product.Brand == "" {
		if brand, exists := pe.doc.Find(`meta[property="og:product:brand"]`).Attr("content"); exists {
			product.Brand = brand
		}
		if brand, exists := pe.doc.Find(`meta[property="product:brand"]`).Attr("content"); exists {
			product.Brand = brand
		}
	}
}

func (pe *ProductExtractor) extractMetaTags(product *ProductData) {
	// Twitter product cards
	if product.Price == 0 {
		if price, exists := pe.doc.Find(`meta[name="twitter:data1"]`).Attr("content"); exists {
			product.Price = parsePrice(price)
		}
	}
}

func (pe *ProductExtractor) extractFromSelectors(product *ProductData) {
	// Common product name selectors
	if product.Name == "" {
		nameSelectors := []string{
			`h1[itemprop="name"]`,
			`.product-title h1`,
			`.product-name h1`,
			`.product-name`,
			`.product-title`,
			`h1.title`,
			`[data-testid="product-title"]`,
			`.pdp-title`,
			`#product-name`,
		}
		for _, sel := range nameSelectors {
			if name := strings.TrimSpace(pe.doc.Find(sel).First().Text()); name != "" {
				product.Name = name
				break
			}
		}
	}

	// Common price selectors
	if product.Price == 0 {
		priceSelectors := []string{
			`[itemprop="price"]`,
			`.product-price`,
			`.price-current`,
			`.current-price`,
			`.sale-price`,
			`.final-price`,
			`[data-testid="product-price"]`,
			`.pdp-price`,
			`#product-price`,
			`.price`,
		}
		for _, sel := range priceSelectors {
			elem := pe.doc.Find(sel).First()
			if priceStr := elem.AttrOr("content", ""); priceStr != "" {
				product.Price = parsePrice(priceStr)
				break
			}
			if priceStr := strings.TrimSpace(elem.Text()); priceStr != "" {
				product.Price = parsePrice(priceStr)
				if product.Price > 0 {
					break
				}
			}
		}
	}

	// Original price selectors
	if product.OriginalPrice == 0 {
		origPriceSelectors := []string{
			`.original-price`,
			`.old-price`,
			`.was-price`,
			`.list-price`,
			`.regular-price`,
			`[data-testid="original-price"]`,
			`.price-old`,
			`del.price`,
			`s.price`,
		}
		for _, sel := range origPriceSelectors {
			if priceStr := strings.TrimSpace(pe.doc.Find(sel).First().Text()); priceStr != "" {
				product.OriginalPrice = parsePrice(priceStr)
				if product.OriginalPrice > 0 {
					break
				}
			}
		}
	}

	// Brand selectors
	if product.Brand == "" {
		brandSelectors := []string{
			`[itemprop="brand"]`,
			`.product-brand`,
			`.brand-name`,
			`[data-testid="product-brand"]`,
			`.pdp-brand`,
		}
		for _, sel := range brandSelectors {
			elem := pe.doc.Find(sel).First()
			if brand := elem.AttrOr("content", ""); brand != "" {
				product.Brand = brand
				break
			}
			if brand := strings.TrimSpace(elem.Text()); brand != "" {
				product.Brand = brand
				break
			}
		}
	}

	// SKU selectors
	if product.SKU == "" {
		skuSelectors := []string{
			`[itemprop="sku"]`,
			`.product-sku`,
			`.sku`,
			`[data-sku]`,
		}
		for _, sel := range skuSelectors {
			elem := pe.doc.Find(sel).First()
			if sku := elem.AttrOr("content", ""); sku != "" {
				product.SKU = sku
				break
			}
			if sku := elem.AttrOr("data-sku", ""); sku != "" {
				product.SKU = sku
				break
			}
			if sku := strings.TrimSpace(elem.Text()); sku != "" {
				product.SKU = sku
				break
			}
		}
	}

	// Availability selectors
	if product.Availability == "" {
		availSelectors := []string{
			`[itemprop="availability"]`,
			`.product-availability`,
			`.stock-status`,
			`.availability`,
		}
		for _, sel := range availSelectors {
			elem := pe.doc.Find(sel).First()
			if avail := elem.AttrOr("content", ""); avail != "" {
				product.Availability = avail
				product.InStock = isInStock(avail)
				break
			}
			if avail := elem.AttrOr("href", ""); avail != "" {
				product.Availability = avail
				product.InStock = isInStock(avail)
				break
			}
			if avail := strings.TrimSpace(elem.Text()); avail != "" {
				product.Availability = avail
				product.InStock = isInStock(avail)
				break
			}
		}
	}

	// Rating selectors
	if product.Rating == 0 {
		ratingSelectors := []string{
			`[itemprop="ratingValue"]`,
			`.product-rating`,
			`.rating-value`,
		}
		for _, sel := range ratingSelectors {
			elem := pe.doc.Find(sel).First()
			if rating := elem.AttrOr("content", ""); rating != "" {
				product.Rating, _ = strconv.ParseFloat(rating, 64)
				break
			}
		}
	}

	// Categories
	pe.doc.Find(`[itemprop="itemListElement"]`).Each(func(i int, s *goquery.Selection) {
		if name := strings.TrimSpace(s.Find(`[itemprop="name"]`).Text()); name != "" {
			product.Categories = append(product.Categories, name)
		}
	})

	if len(product.Categories) > 0 {
		product.Category = strings.Join(product.Categories, " > ")
	}
}

func (pe *ProductExtractor) extractImages(product *ProductData) {
	seen := make(map[string]bool)

	// Main product image
	mainImageSelectors := []string{
		`[itemprop="image"]`,
		`.product-image img`,
		`.main-image img`,
		`.gallery-main img`,
		`[data-testid="product-image"]`,
		`.pdp-image img`,
	}

	for _, sel := range mainImageSelectors {
		pe.doc.Find(sel).Each(func(i int, s *goquery.Selection) {
			if src := getImageSrc(s); src != "" && !seen[src] {
				seen[src] = true
				product.Images = append(product.Images, src)
			}
		})
	}

	// Gallery images
	gallerySelectors := []string{
		`.product-gallery img`,
		`.gallery-thumbs img`,
		`.product-thumbs img`,
		`[data-gallery] img`,
	}

	for _, sel := range gallerySelectors {
		pe.doc.Find(sel).Each(func(i int, s *goquery.Selection) {
			if src := getImageSrc(s); src != "" && !seen[src] {
				seen[src] = true
				product.Images = append(product.Images, src)
			}
		})
	}
}

func (pe *ProductExtractor) extractSpecs(product *ProductData) {
	// Common spec table patterns
	specSelectors := []string{
		`.product-specs tr`,
		`.specifications tr`,
		`.tech-specs tr`,
		`table.specs tr`,
		`[itemprop="additionalProperty"]`,
	}

	for _, sel := range specSelectors {
		pe.doc.Find(sel).Each(func(i int, s *goquery.Selection) {
			var key, value string

			// Try th/td pattern
			if th := s.Find("th").First(); th.Length() > 0 {
				key = strings.TrimSpace(th.Text())
				value = strings.TrimSpace(s.Find("td").First().Text())
			}

			// Try itemprop pattern
			if key == "" {
				key = s.AttrOr("itemprop", "")
				if key == "" {
					key = strings.TrimSpace(s.Find(`[itemprop="name"]`).Text())
				}
				value = strings.TrimSpace(s.Find(`[itemprop="value"]`).Text())
			}

			// Try two td pattern
			if key == "" {
				tds := s.Find("td")
				if tds.Length() >= 2 {
					key = strings.TrimSpace(tds.Eq(0).Text())
					value = strings.TrimSpace(tds.Eq(1).Text())
				}
			}

			if key != "" && value != "" {
				key = strings.TrimSuffix(key, ":")
				product.Specs[key] = value
			}
		})
	}

	// Definition list pattern
	pe.doc.Find(".product-specs dl, .specifications dl").Each(func(i int, s *goquery.Selection) {
		dts := s.Find("dt")
		dds := s.Find("dd")
		for j := 0; j < dts.Length() && j < dds.Length(); j++ {
			key := strings.TrimSpace(dts.Eq(j).Text())
			value := strings.TrimSpace(dds.Eq(j).Text())
			if key != "" && value != "" {
				key = strings.TrimSuffix(key, ":")
				product.Specs[key] = value
			}
		}
	})
}

// Helper functions

func extractJSONValue(json, key string) string {
	patterns := []string{
		`"` + key + `"\s*:\s*"([^"]*)"`,
		`"` + key + `"\s*:\s*(\d+\.?\d*)`,
	}

	for _, pattern := range patterns {
		re := regexp.MustCompile(pattern)
		if matches := re.FindStringSubmatch(json); len(matches) > 1 {
			return matches[1]
		}
	}
	return ""
}

func extractNestedJSONValue(json, parent, key string) string {
	// Simple nested extraction
	pattern := `"` + parent + `"\s*:\s*\{[^}]*"` + key + `"\s*:\s*"([^"]*)"`
	re := regexp.MustCompile(pattern)
	if matches := re.FindStringSubmatch(json); len(matches) > 1 {
		return matches[1]
	}
	return ""
}

func parsePrice(s string) float64 {
	// Remove currency symbols and text
	s = strings.TrimSpace(s)

	// Remove common currency symbols
	currencySymbols := []string{"$", "€", "£", "¥", "₹", "﷼", "تومان", "ریال", "IRR", "USD", "EUR", "GBP"}
	for _, sym := range currencySymbols {
		s = strings.ReplaceAll(s, sym, "")
	}

	// Remove spaces and commas (but keep Persian/Arabic digits)
	s = strings.ReplaceAll(s, " ", "")
	s = strings.ReplaceAll(s, ",", "")
	s = strings.ReplaceAll(s, "٬", "") // Persian thousands separator

	// Convert Persian digits to ASCII
	persianDigits := map[rune]rune{
		'۰': '0', '۱': '1', '۲': '2', '۳': '3', '۴': '4',
		'۵': '5', '۶': '6', '۷': '7', '۸': '8', '۹': '9',
	}
	var converted strings.Builder
	for _, r := range s {
		if ascii, ok := persianDigits[r]; ok {
			converted.WriteRune(ascii)
		} else {
			converted.WriteRune(r)
		}
	}
	s = converted.String()

	// Extract number
	re := regexp.MustCompile(`[\d.]+`)
	matches := re.FindString(s)
	if matches == "" {
		return 0
	}

	price, _ := strconv.ParseFloat(matches, 64)
	return price
}

func isInStock(availability string) bool {
	availability = strings.ToLower(availability)
	inStockIndicators := []string{
		"instock",
		"in_stock",
		"in stock",
		"available",
		"موجود",
		"https://schema.org/instock",
	}
	for _, indicator := range inStockIndicators {
		if strings.Contains(availability, indicator) {
			return true
		}
	}
	return false
}

func getImageSrc(s *goquery.Selection) string {
	// Try various image source attributes
	attrs := []string{"src", "data-src", "data-lazy-src", "data-original", "srcset"}
	for _, attr := range attrs {
		if src, exists := s.Attr(attr); exists && src != "" {
			// For srcset, get the first URL
			if attr == "srcset" {
				parts := strings.Split(src, ",")
				if len(parts) > 0 {
					src = strings.Fields(parts[0])[0]
				}
			}
			return strings.TrimSpace(src)
		}
	}
	return ""
}
