// internal/extractor/price.go
package extractor

import (
	"regexp"
	"strconv"
	"strings"

	"github.com/PuerkitoBio/goquery"
)

var numericRe = regexp.MustCompile(`[\d.]+`)

// Persian/Arabic digit normalizer — built once.
var persianDigitReplacer = strings.NewReplacer(
	"۰", "0", "۱", "1", "۲", "2", "۳", "3", "۴", "4",
	"۵", "5", "۶", "6", "۷", "7", "۸", "8", "۹", "9",
	"٠", "0", "١", "1", "٢", "2", "٣", "3", "٤", "4",
	"٥", "5", "٦", "6", "٧", "7", "٨", "8", "٩", "9",
)

var priceSelectors = []string{
	"[itemprop='price']",
	".price",
	".product-price",
	".current-price",
	".sale-price",
	"[data-price]",
}

// currencyKeywords maps text fragments to ISO currency codes.
var currencyKeywords = map[string]string{
	"تومان": "IRR",
	"ریال":  "IRR",
	"$":     "USD",
	"usd":   "USD",
	"€":     "EUR",
	"eur":   "EUR",
	"£":     "GBP",
	"gbp":   "GBP",
}

// extractPrice returns the price and currency from DOM selectors.
func extractPrice(doc *goquery.Selection) (float64, string) {
	for _, sel := range priceSelectors {
		el := doc.Find(sel).First()
		if el.Length() == 0 {
			continue
		}

		// Try content attribute first (microdata)
		if priceStr, exists := el.Attr("content"); exists {
			if price := parsePrice(priceStr); price > 0 {
				return price, extractCurrency(doc)
			}
		}

		// Try data-price attribute
		if priceStr, exists := el.Attr("data-price"); exists {
			if price := parsePrice(priceStr); price > 0 {
				return price, extractCurrency(doc)
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

	return 0, ""
}

// extractCurrency gets the currency from itemprop or page text.
func extractCurrency(doc *goquery.Selection) string {
	if currency, exists := doc.Find("[itemprop='priceCurrency']").Attr("content"); exists {
		return strings.ToUpper(currency)
	}

	text := doc.Text()
	if strings.Contains(text, "تومان") || strings.Contains(text, "ریال") {
		return "IRR"
	}

	return "USD"
}

// extractCurrencyFromText detects currency from a price string.
func extractCurrencyFromText(text string) string {
	lower := strings.ToLower(text)
	for keyword, code := range currencyKeywords {
		if strings.Contains(lower, keyword) {
			return code
		}
	}
	return ""
}

// parsePrice normalizes and parses a price string into a float64.
func parsePrice(text string) float64 {
	// Normalize Persian/Arabic digits to Latin
	text = persianDigitReplacer.Replace(text)

	// Strip currency symbols and text
	for keyword := range currencyKeywords {
		text = strings.ReplaceAll(text, keyword, "")
	}
	text = strings.ReplaceAll(text, ",", "")
	text = strings.ReplaceAll(text, "٬", "") // Persian comma
	text = strings.TrimSpace(text)

	match := numericRe.FindString(text)
	if match == "" {
		return 0
	}

	price, err := strconv.ParseFloat(match, 64)
	if err != nil {
		return 0
	}
	return price
}
