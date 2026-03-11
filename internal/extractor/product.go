package extractor

import (
	"encoding/json"
	"regexp"
	"strconv"
	"strings"

	"spiderly/internal/models"

	"github.com/PuerkitoBio/goquery"
)

//
// ──────────────────────────────────────────
//   PRICE NORMALIZATION
// ──────────────────────────────────────────
//

func normalizePrimaryPrice(p *models.ProductInfo) {
	if p == nil || len(p.Prices) == 0 {
		return
	}

	best := p.Prices[0] // first price = canonical primary price
	p.Price = best.Amount
	p.Currency = best.Currency
	p.InStock = best.InStock

	if p.OriginalPrice > 0 && p.Price > 0 {
		p.Discount = (1 - (p.Price / p.OriginalPrice)) * 100
		if p.Discount < 0 {
			p.Discount = 0
		}
	}
}

//
// ──────────────────────────────────────────
//   MAIN EXTRACTION ROUTINE
// ──────────────────────────────────────────
//

func ExtractProduct(doc *goquery.Selection, pageURL string) *models.ProductInfo {
	final := &models.ProductInfo{
		Prices: []models.PriceInfo{},
	}

	// 1. METADATA FIRST (Title / Description / OG)
	extractMetadata(doc, final)

	// 2. JSON-LD (highest priority)
	if p := tryJSONLD(doc); p != nil {
		mergeProduct(final, p)
	}

	// 3. OpenGraph
	if p := tryOpenGraph(doc); p != nil {
		mergeProduct(final, p)
	}

	// 4. Microdata
	if p := tryMicrodata(doc); p != nil {
		mergeProduct(final, p)
	}

	// 5. Heuristics (last fallback)
	if p := tryHeuristics(doc); p != nil {
		mergeProduct(final, p)
	}

	// Nothing valid found
	if final.Name == "" && len(final.Prices) == 0 {
		return nil
	}

	final.SourceURL = pageURL
	normalizePrimaryPrice(final)

	return final
}

//
// ──────────────────────────────────────────
//   METADATA EXTRACTION (NEW)
// ──────────────────────────────────────────
//

func extractMetadata(doc *goquery.Selection, p *models.ProductInfo) {

	title := strings.TrimSpace(doc.Find("head title").Text())
	if p.Name == "" && title != "" {
		p.Name = title
	}

	doc.Find("meta").Each(func(_ int, s *goquery.Selection) {

		name := s.AttrOr("name", "")
		prop := s.AttrOr("property", "")
		content := s.AttrOr("content", "")

		switch name {
		case "description":
			if p.Description == "" {
				p.Description = content
			}
		case "keywords":
			if p.Keywords == "" {
				p.Keywords = content
			}
		}

		switch prop {
		case "og:title":
			if p.Name == "" {
				p.Name = content
			}
		case "og:description":
			if p.Description == "" {
				p.Description = content
			}
		case "og:url":
			if p.CanonicalURL == "" {
				p.CanonicalURL = content
			}
		case "og:image":
			if content != "" {
				p.Images = append(p.Images, content)
			}
		case "twitter:image":
			if content != "" {
				p.Images = append(p.Images, content)
			}
		}
	})
}

//
// ──────────────────────────────────────────
//   SMART MERGE LOGIC (NEW)
// ──────────────────────────────────────────
//

func mergeProduct(base *models.ProductInfo, incoming *models.ProductInfo) {
	if incoming == nil {
		return
	}

	// Basic fields
	if base.Name == "" && incoming.Name != "" {
		base.Name = incoming.Name
	}
	if base.Brand == "" && incoming.Brand != "" {
		base.Brand = incoming.Brand
	}
	if base.SKU == "" && incoming.SKU != "" {
		base.SKU = incoming.SKU
	}
	if base.Description == "" && incoming.Description != "" {
		base.Description = incoming.Description
	}
	if base.CanonicalURL == "" && incoming.CanonicalURL != "" {
		base.CanonicalURL = incoming.CanonicalURL
	}

	// Images (append but avoid duplicates)
	existing := map[string]bool{}
	for _, img := range base.Images {
		existing[img] = true
	}
	for _, img := range incoming.Images {
		if !existing[img] && img != "" {
			base.Images = append(base.Images, img)
		}
	}

	// Prices (append unique sellers)
	priceSeen := map[string]bool{}
	for _, pr := range base.Prices {
		key := pr.Currency + "_" + strconv.FormatFloat(pr.Amount, 'f', -1, 64)
		priceSeen[key] = true
	}
	for _, pr := range incoming.Prices {
		key := pr.Currency + "_" + strconv.FormatFloat(pr.Amount, 'f', -1, 64)
		if pr.Amount > 0 && !priceSeen[key] {
			base.Prices = append(base.Prices, pr)
		}
	}

	// Stock
	if !base.InStock && incoming.InStock {
		base.InStock = true
	}

	// Original Price
	if base.OriginalPrice == 0 && incoming.OriginalPrice > 0 {
		base.OriginalPrice = incoming.OriginalPrice
	}
}

//
// ──────────────────────────────────────────
//   JSON-LD
// ──────────────────────────────────────────
//

func tryJSONLD(doc *goquery.Selection) *models.ProductInfo {
	var result *models.ProductInfo

	doc.Find(`script[type="application/ld+json"]`).Each(func(_ int, s *goquery.Selection) {
		if result != nil {
			return
		}

		raw := strings.TrimSpace(s.Text())
		if raw == "" {
			return
		}

		var data interface{}
		if err := json.Unmarshal([]byte(raw), &data); err != nil {
			return
		}

		switch v := data.(type) {
		case map[string]interface{}:
			if isProductJSONLD(v) {
				result = parseJSONLDProduct(v)
			}
		case []interface{}:
			for _, item := range v {
				if m, ok := item.(map[string]interface{}); ok && isProductJSONLD(m) {
					result = parseJSONLDProduct(m)
					return
				}
			}
		}
	})

	return result
}

func isProductJSONLD(m map[string]interface{}) bool {
	t, ok := m["@type"].(string)
	return ok && strings.ToLower(t) == "product"
}

//
// ──────────────────────────────────────────
//   JSON-LD Parsing
// ──────────────────────────────────────────
//

func parseJSONLDProduct(data map[string]interface{}) *models.ProductInfo {
	p := &models.ProductInfo{
		Prices: []models.PriceInfo{},
	}

	if name, ok := data["name"].(string); ok {
		p.Name = name
	}
	if sku, ok := data["sku"].(string); ok {
		p.SKU = sku
	}

	// Brand
	switch b := data["brand"].(type) {
	case string:
		p.Brand = b
	case map[string]interface{}:
		if bn, ok := b["name"].(string); ok {
			p.Brand = bn
		}
	}

	// Images
	switch img := data["image"].(type) {
	case string:
		p.Images = append(p.Images, img)
	case []interface{}:
		for _, i := range img {
			if s, ok := i.(string); ok {
				p.Images = append(p.Images, s)
			}
		}
	}

	// Offers
	switch offers := data["offers"].(type) {
	case map[string]interface{}:
		if pr := extractPriceFromOffer(offers); pr != nil {
			p.Prices = append(p.Prices, *pr)
		}
	case []interface{}:
		for _, of := range offers {
			if m, ok := of.(map[string]interface{}); ok {
				if pr := extractPriceFromOffer(m); pr != nil {
					p.Prices = append(p.Prices, *pr)
				}
			}
		}
	}

	return p
}

//
// ──────────────────────────────────────────
//   Extract price block from JSON-LD offer
// ──────────────────────────────────────────
//

func extractPriceFromOffer(m map[string]interface{}) *models.PriceInfo {
	pr := &models.PriceInfo{}

	// Price
	switch raw := m["price"].(type) {
	case string:
		pr.Amount = parsePrice(raw)
	case float64:
		pr.Amount = raw
	}

	// Currency
	if c, ok := m["priceCurrency"].(string); ok {
		pr.Currency = c
	}

	// Seller
	switch seller := m["seller"].(type) {
	case string:
		pr.Seller = seller
	case map[string]interface{}:
		if n, ok := seller["name"].(string); ok {
			pr.Seller = n
		}
	}

	// Availability
	if a, ok := m["availability"].(string); ok {
		pr.InStock = strings.Contains(strings.ToLower(a), "instock")
	}

	if pr.Amount > 0 {
		return pr
	}
	return nil
}

//
// ──────────────────────────────────────────
//   OpenGraph
// ──────────────────────────────────────────
//

func tryOpenGraph(doc *goquery.Selection) *models.ProductInfo {
	p := &models.ProductInfo{Prices: []models.PriceInfo{}}
	found := false

	doc.Find("meta").Each(func(_ int, s *goquery.Selection) {
		prop := s.AttrOr("property", "")
		content := s.AttrOr("content", "")

		switch prop {
		case "og:title":
			p.Name = content
			found = true
		case "og:description":
			p.Description = content
			found = true
		case "og:image":
			p.Images = append(p.Images, content)
			found = true
		case "product:brand":
			p.Brand = content
			found = true
		case "product:price:amount":
			amount := parsePrice(content)
			if amount > 0 {
				p.Prices = append(p.Prices, models.PriceInfo{Amount: amount})
				found = true
			}
		case "product:price:currency":
			if len(p.Prices) > 0 {
				p.Prices[0].Currency = content
			}
		case "product:availability":
			p.InStock = strings.Contains(strings.ToLower(content), "instock")
			found = true
		}
	})

	if found {
		return p
	}
	return nil
}

//
// ──────────────────────────────────────────
//   Microdata
// ──────────────────────────────────────────
//

func tryMicrodata(doc *goquery.Selection) *models.ProductInfo {
	p := &models.ProductInfo{Prices: []models.PriceInfo{}}
	found := false

	doc.Find("[itemprop]").Each(func(_ int, s *goquery.Selection) {
		prop := s.AttrOr("itemprop", "")
		val := s.AttrOr("content", "")
		if val == "" {
			val = strings.TrimSpace(s.Text())
		}

		switch prop {
		case "name":
			p.Name = val
			found = true
		case "price":
			amount := parsePrice(val)
			if amount > 0 {
				p.Prices = append(p.Prices, models.PriceInfo{Amount: amount})
				found = true
			}
		case "priceCurrency":
			if len(p.Prices) > 0 {
				p.Prices[0].Currency = val
			}
		case "brand":
			p.Brand = val
			found = true
		case "sku":
			p.SKU = val
			found = true
		case "image":
			src := s.AttrOr("src", "")
			if src == "" {
				src = val
			}
			p.Images = append(p.Images, src)
			found = true
		case "availability":
			p.InStock = strings.Contains(strings.ToLower(val), "instock")
			found = true
		}
	})

	if found {
		return p
	}
	return nil
}

//
// ──────────────────────────────────────────
//   Heuristic fallback
// ──────────────────────────────────────────
//

func tryHeuristics(doc *goquery.Selection) *models.ProductInfo {
	p := &models.ProductInfo{Prices: []models.PriceInfo{}}
	found := false

	titleSelectors := []string{
		"h1.product-title",
		"h1.product-name",
		"h1",
	}

	for _, sel := range titleSelectors {
		if t := strings.TrimSpace(doc.Find(sel).First().Text()); t != "" {
			p.Name = t
			found = true
			break
		}
	}

	priceSelectors := []string{
		".price", "[class*='price']", "[id*='price']",
	}

	for _, sel := range priceSelectors {
		doc.Find(sel).Each(func(_ int, s *goquery.Selection) {
			text := strings.TrimSpace(s.Text())
			amount := parsePrice(text)
			if amount > 0 {
				pr := models.PriceInfo{Amount: amount}

				if strings.Contains(text, "$") {
					pr.Currency = "USD"
				} else if strings.Contains(text, "€") {
					pr.Currency = "EUR"
				} else if strings.Contains(text, "£") {
					pr.Currency = "GBP"
				} else if strings.Contains(text, "تومان") {
					pr.Currency = "IRR"
				}

				p.Prices = append(p.Prices, pr)
				found = true
			}
		})
	}

	imageSelectors := []string{
		".product-image img",
		"[class*='product'] img",
	}

	for _, sel := range imageSelectors {
		doc.Find(sel).Each(func(_ int, s *goquery.Selection) {
			src := s.AttrOr("src", "")
			if src != "" {
				p.Images = append(p.Images, src)
				found = true
			}
		})
	}

	if found {
		return p
	}
	return nil
}

//
// ──────────────────────────────────────────
//   Price parsing
// ──────────────────────────────────────────
//

func parsePrice(v string) float64 {
	v = strings.ReplaceAll(v, ",", "")
	v = strings.ReplaceAll(v, "$", "")
	v = strings.ReplaceAll(v, "€", "")
	v = strings.ReplaceAll(v, "£", "")
	v = strings.ReplaceAll(v, "تومان", "")
	v = strings.ReplaceAll(v, "ریال", "")
	v = strings.TrimSpace(v)

	r := regexp.MustCompile(`[\d.]+`)
	n := r.FindString(v)

	if n == "" {
		return 0
	}

	val, _ := strconv.ParseFloat(n, 64)
	return val
}
