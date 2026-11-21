// internal/extractor/jsonld.go
package extractor

import (
	"encoding/json"
	"fmt"
	"math"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"spiderly/internal/models"

	"github.com/PuerkitoBio/goquery"
)

var currencySymbolMap = map[string]string{
	"$":   "USD",
	"€":   "EUR",
	"£":   "GBP",
	"¥":   "JPY",
	"₹":   "INR",
	"C$":  "CAD",
	"A$":  "AUD",
	"CHF": "CHF",
	"kr":  "SEK",
	"R$":  "BRL",
	"₽":   "RUB",
}

// ExtractJSONLD parses all script[type="application/ld+json"] tags in doc
// and extracts normalized structured data.
func ExtractJSONLD(doc *goquery.Selection) *models.StructuredData {
	all := ExtractAllJSONLD(doc)
	if len(all) == 0 {
		// Fallback to DOM breadcrumbs if no JSON-LD structured data
		domBreadcrumbs := ExtractDOMBreadcrumbs(doc)
		if len(domBreadcrumbs) > 0 {
			return &models.StructuredData{
				Breadcrumbs: domBreadcrumbs,
			}
		}
		return nil
	}

	// Prefer a Product entity if available, otherwise the first entity
	var primary *models.StructuredData
	for _, item := range all {
		if isProductType(item.Type) && item.Name != "" {
			primary = item
			break
		}
	}
	if primary == nil {
		primary = all[0]
	}

	// Merge breadcrumbs from other entities if missing
	if len(primary.Breadcrumbs) == 0 {
		for _, item := range all {
			if len(item.Breadcrumbs) > 0 {
				primary.Breadcrumbs = item.Breadcrumbs
				break
			}
		}
	}

	// If still no breadcrumbs, attempt DOM breadcrumb extraction
	if len(primary.Breadcrumbs) == 0 {
		primary.Breadcrumbs = ExtractDOMBreadcrumbs(doc)
	}

	return primary
}

// ExtractAllJSONLD parses all JSON-LD blocks and returns all recognized entities.
func ExtractAllJSONLD(doc *goquery.Selection) []*models.StructuredData {
	var results []*models.StructuredData
	var breadcrumbs []string

	doc.Find(`script[type="application/ld+json"]`).Each(func(_ int, s *goquery.Selection) {
		raw := strings.TrimSpace(s.Text())
		if raw == "" {
			return
		}

		parsedItems := parseJSONLDRaw(raw)
		for _, itemMap := range parsedItems {
			types := extractTypes(itemMap)

			// Check BreadcrumbList
			if containsType(types, "breadcrumblist") {
				crumbs := parseBreadcrumbList(itemMap)
				if len(crumbs) > 0 {
					breadcrumbs = crumbs
				}
			}

			// Check Product or ItemPage
			if containsAnyType(types, "product", "individualproduct", "productmodel", "vehicle", "book", "course") {
				sd := parseProductMap(itemMap, types)
				if sd != nil {
					results = append(results, sd)
				}
			} else if containsAnyType(types, "article", "newsarticle", "blogposting") {
				sd := parseArticleMap(itemMap, types)
				if sd != nil {
					results = append(results, sd)
				}
			}
		}
	})

	// Attach breadcrumbs to all products if found
	if len(breadcrumbs) > 0 {
		for _, r := range results {
			if len(r.Breadcrumbs) == 0 {
				r.Breadcrumbs = breadcrumbs
			}
		}
		if len(results) == 0 {
			results = append(results, &models.StructuredData{
				Type:        "BreadcrumbList",
				Breadcrumbs: breadcrumbs,
			})
		}
	}

	return results
}

// parseJSONLDRaw converts raw JSON string into a list of generic maps.
func parseJSONLDRaw(raw string) []map[string]interface{} {
	var out []map[string]interface{}

	// Could be an array of objects
	if strings.HasPrefix(raw, "[") {
		var arr []interface{}
		if err := json.Unmarshal([]byte(raw), &arr); err == nil {
			for _, item := range arr {
				if m, ok := item.(map[string]interface{}); ok {
					out = append(out, expandGraph(m)...)
				}
			}
			return out
		}
	}

	// Could be a single object
	var obj map[string]interface{}
	if err := json.Unmarshal([]byte(raw), &obj); err == nil {
		out = append(out, expandGraph(obj)...)
	}

	return out
}

// expandGraph flattens any "@graph" array inside a JSON-LD object.
func expandGraph(m map[string]interface{}) []map[string]interface{} {
	var list []map[string]interface{}

	if graph, ok := m["@graph"]; ok {
		if arr, ok := graph.([]interface{}); ok {
			for _, elem := range arr {
				if elemMap, ok := elem.(map[string]interface{}); ok {
					list = append(list, elemMap)
				}
			}
		}
	}

	list = append(list, m)
	return list
}

func extractTypes(m map[string]interface{}) []string {
	var types []string
	val, ok := m["@type"]
	if !ok {
		return types
	}

	switch v := val.(type) {
	case string:
		clean := cleanType(v)
		if clean != "" {
			types = append(types, clean)
		}
	case []interface{}:
		for _, item := range v {
			if s, ok := item.(string); ok {
				clean := cleanType(s)
				if clean != "" {
					types = append(types, clean)
				}
			}
		}
	}
	return types
}

func cleanType(raw string) string {
	raw = strings.TrimSpace(raw)
	// Strip URI prefix
	if idx := strings.LastIndex(raw, "/"); idx != -1 {
		raw = raw[idx+1:]
	}
	if idx := strings.LastIndex(raw, "#"); idx != -1 {
		raw = raw[idx+1:]
	}
	return strings.ToLower(raw)
}

func containsType(types []string, target string) bool {
	for _, t := range types {
		if t == target {
			return true
		}
	}
	return false
}

func containsAnyType(types []string, targets ...string) bool {
	for _, target := range targets {
		if containsType(types, target) {
			return true
		}
	}
	return false
}

func isProductType(t string) bool {
	t = cleanType(t)
	return t == "product" || t == "individualproduct" || t == "productmodel" || t == "vehicle"
}

// parseProductMap extracts product data from a schema.org Product map.
func parseProductMap(m map[string]interface{}, types []string) *models.StructuredData {
	sd := &models.StructuredData{
		Type:     "Product",
		RawTypes: types,
		Specs:    make(map[string]string),
	}

	sd.Name = extractString(m, "name", "title")
	sd.Description = extractString(m, "description")
	sd.SKU = extractString(m, "sku")
	sd.GTIN = extractString(m, "gtin", "gtin13", "gtin14", "gtin12", "gtin8")
	sd.MPN = extractString(m, "mpn")

	// Brand
	sd.Brand = extractBrandName(m["brand"])

	// Category
	sd.Category = extractCategory(m["category"])

	// Images
	sd.Images = extractImagesList(m["image"])

	// Offers & Prices
	parseOffers(m["offers"], sd)

	// Aggregate Rating
	parseAggregateRating(m["aggregateRating"], sd)

	// Additional Property / Specs
	parseAdditionalProperties(m["additionalProperty"], sd)

	// Require at least a name or price or SKU to be a valid product structured data
	if sd.Name == "" && sd.Price == 0 && sd.SKU == "" {
		return nil
	}

	return sd
}

// parseArticleMap extracts article data from a schema.org Article map.
func parseArticleMap(m map[string]interface{}, types []string) *models.StructuredData {
	sd := &models.StructuredData{
		Type:     "Article",
		RawTypes: types,
	}

	sd.Name = extractString(m, "headline", "name")
	sd.Description = extractString(m, "description")
	sd.Category = extractString(m, "articleSection")
	sd.Images = extractImagesList(m["image"])

	if sd.Name == "" {
		return nil
	}
	return sd
}

// parseBreadcrumbList extracts breadcrumbs from a BreadcrumbList entity.
func parseBreadcrumbList(m map[string]interface{}) []string {
	rawList, ok := m["itemListElement"]
	if !ok {
		return nil
	}

	type crumbItem struct {
		position int
		name     string
	}

	var items []crumbItem

	processElement := func(elem interface{}) {
		elemMap, ok := elem.(map[string]interface{})
		if !ok {
			return
		}

		pos := 0
		if pVal, ok := elemMap["position"]; ok {
			switch p := pVal.(type) {
			case float64:
				pos = int(p)
			case int:
				pos = p
			case string:
				pos, _ = strconv.Atoi(p)
			}
		}

		name := extractString(elemMap, "name")
		if name == "" {
			if itemObj, ok := elemMap["item"].(map[string]interface{}); ok {
				name = extractString(itemObj, "name")
			}
		}

		name = strings.TrimSpace(name)
		if name != "" {
			items = append(items, crumbItem{position: pos, name: name})
		}
	}

	switch v := rawList.(type) {
	case []interface{}:
		for _, item := range v {
			processElement(item)
		}
	case map[string]interface{}:
		processElement(v)
	}

	sort.SliceStable(items, func(i, j int) bool {
		return items[i].position < items[j].position
	})

	var result []string
	for _, item := range items {
		result = append(result, item.name)
	}
	return result
}

// ExtractDOMBreadcrumbs extracts breadcrumb text from common HTML markup.
func ExtractDOMBreadcrumbs(doc *goquery.Selection) []string {
	if doc == nil {
		return nil
	}

	var crumbs []string
	selectors := []string{
		`nav[aria-label*="breadcrumb" i] li`,
		`ol.breadcrumb li`,
		`ul.breadcrumb li`,
		`ul.breadcrumbs li`,
		`.breadcrumb-item`,
		`[itemtype*="BreadcrumbList"] [itemprop="itemListElement"]`,
	}

	for _, sel := range selectors {
		elements := doc.Find(sel)
		if elements.Length() > 0 {
			elements.Each(func(_ int, s *goquery.Selection) {
				text := strings.TrimSpace(s.Text())
				// Remove common separators
				text = strings.Trim(text, "/>»-— \t\n\r")
				if text != "" && len(text) < 60 && !containsString(crumbs, text) {
					crumbs = append(crumbs, text)
				}
			})
			if len(crumbs) > 0 {
				return crumbs
			}
		}
	}

	return nil
}

func containsString(slice []string, s string) bool {
	for _, item := range slice {
		if strings.EqualFold(item, s) {
			return true
		}
	}
	return false
}

// parseOffers extracts price, currency, availability, and seller info.
func parseOffers(raw interface{}, sd *models.StructuredData) {
	if raw == nil {
		return
	}

	var offerList []map[string]interface{}
	switch v := raw.(type) {
	case map[string]interface{}:
		offerList = append(offerList, v)
	case []interface{}:
		for _, item := range v {
			if m, ok := item.(map[string]interface{}); ok {
				offerList = append(offerList, m)
			}
		}
	}

	for _, offer := range offerList {
		// Price
		if sd.Price == 0 {
			price, curr := parsePriceValue(offer["price"])
			if price > 0 {
				sd.Price = price
			}
			if curr != "" && sd.Currency == "" {
				sd.Currency = curr
			}
			// Check AggregateOffer lowPrice / highPrice
			if sd.Price == 0 {
				lowPrice, _ := parsePriceValue(offer["lowPrice"])
				if lowPrice > 0 {
					sd.Price = lowPrice
				}
			}
		}

		// Currency
		if sd.Currency == "" {
			if curr, ok := offer["priceCurrency"].(string); ok && strings.TrimSpace(curr) != "" {
				sd.Currency = strings.ToUpper(strings.TrimSpace(curr))
			}
		}

		// Availability
		if sd.Availability == "" {
			if avail, ok := offer["availability"]; ok {
				normAvail, inStock := normalizeAvailability(avail)
				sd.Availability = normAvail
				sd.InStock = inStock
			}
		}

		// Stop once we have price & availability
		if sd.Price > 0 && sd.Availability != "" {
			break
		}
	}
}

// parsePriceValue parses price number or string into a float64 and currency.
func parsePriceValue(val interface{}) (float64, string) {
	if val == nil {
		return 0, ""
	}

	switch v := val.(type) {
	case float64:
		return v, ""
	case int:
		return float64(v), ""
	case int64:
		return float64(v), ""
	case string:
		clean := strings.TrimSpace(v)
		if clean == "" {
			return 0, ""
		}

		// Check for currency symbols
		var foundCurrency string
		for symbol, code := range currencySymbolMap {
			if strings.Contains(clean, symbol) {
				foundCurrency = code
				clean = strings.ReplaceAll(clean, symbol, "")
				break
			}
		}

		// Extract numbers, comma, dot
		re := regexp.MustCompile(`[0-9]+([.,][0-9]+)?`)
		match := re.FindString(clean)
		if match == "" {
			return 0, foundCurrency
		}

		// Handle comma as decimal separator if followed by 2 digits at end
		if strings.Contains(match, ",") && !strings.Contains(match, ".") {
			match = strings.ReplaceAll(match, ",", ".")
		} else if strings.Contains(match, ",") && strings.Contains(match, ".") {
			// e.g. 1,299.99
			match = strings.ReplaceAll(match, ",", "")
		}

		parsed, err := strconv.ParseFloat(match, 64)
		if err == nil && !math.IsNaN(parsed) && !math.IsInf(parsed, 0) {
			return parsed, foundCurrency
		}
		return 0, foundCurrency
	}
	return 0, ""
}

// normalizeAvailability cleans schema.org availability values.
func normalizeAvailability(val interface{}) (string, bool) {
	if val == nil {
		return "", false
	}
	s := fmt.Sprintf("%v", val)
	sLower := strings.ToLower(s)

	if strings.Contains(sLower, "instock") || strings.Contains(sLower, "available") || strings.Contains(sLower, "in_stock") {
		return "InStock", true
	}
	if strings.Contains(sLower, "preorder") || strings.Contains(sLower, "pre_order") {
		return "PreOrder", true
	}
	if strings.Contains(sLower, "backorder") {
		return "BackOrder", true
	}
	if strings.Contains(sLower, "limitedavailability") {
		return "LimitedAvailability", true
	}
	if strings.Contains(sLower, "outofstock") || strings.Contains(sLower, "soldout") || strings.Contains(sLower, "out_of_stock") {
		return "OutOfStock", false
	}
	if strings.Contains(sLower, "discontinued") {
		return "Discontinued", false
	}

	return s, false
}

// parseAggregateRating extracts ratingValue and reviewCount.
func parseAggregateRating(raw interface{}, sd *models.StructuredData) {
	if raw == nil {
		return
	}
	m, ok := raw.(map[string]interface{})
	if !ok {
		return
	}

	if val, ok := m["ratingValue"]; ok {
		switch v := val.(type) {
		case float64:
			sd.Rating = v
		case int:
			sd.Rating = float64(v)
		case string:
			if f, err := strconv.ParseFloat(strings.TrimSpace(v), 64); err == nil {
				sd.Rating = f
			}
		}
	}

	countVal := m["reviewCount"]
	if countVal == nil {
		countVal = m["ratingCount"]
	}
	if countVal != nil {
		switch v := countVal.(type) {
		case float64:
			sd.ReviewCount = int(v)
		case int:
			sd.ReviewCount = v
		case string:
			if c, err := strconv.Atoi(strings.TrimSpace(v)); err == nil {
				sd.ReviewCount = c
			}
		}
	}
}

// parseAdditionalProperties extracts name-value specification pairs.
func parseAdditionalProperties(raw interface{}, sd *models.StructuredData) {
	if raw == nil {
		return
	}

	processItem := func(item interface{}) {
		m, ok := item.(map[string]interface{})
		if !ok {
			return
		}
		name := extractString(m, "name")
		value := extractString(m, "value")
		if name != "" && value != "" {
			sd.Specs[name] = value
		}
	}

	switch v := raw.(type) {
	case []interface{}:
		for _, item := range v {
			processItem(item)
		}
	case map[string]interface{}:
		processItem(v)
	}
}

func extractString(m map[string]interface{}, keys ...string) string {
	for _, key := range keys {
		if val, ok := m[key]; ok && val != nil {
			switch v := val.(type) {
			case string:
				s := strings.TrimSpace(v)
				if s != "" {
					return s
				}
			case float64:
				return strconv.FormatFloat(v, 'f', -1, 64)
			case int:
				return strconv.Itoa(v)
			}
		}
	}
	return ""
}

func extractBrandName(raw interface{}) string {
	if raw == nil {
		return ""
	}
	switch v := raw.(type) {
	case string:
		return strings.TrimSpace(v)
	case map[string]interface{}:
		return extractString(v, "name")
	}
	return ""
}

func extractCategory(raw interface{}) string {
	if raw == nil {
		return ""
	}
	switch v := raw.(type) {
	case string:
		return strings.TrimSpace(v)
	case []interface{}:
		var parts []string
		for _, item := range v {
			if s, ok := item.(string); ok && strings.TrimSpace(s) != "" {
				parts = append(parts, strings.TrimSpace(s))
			}
		}
		return strings.Join(parts, " > ")
	}
	return ""
}

func extractImagesList(raw interface{}) []string {
	if raw == nil {
		return nil
	}
	var images []string

	addURL := func(u string) {
		u = strings.TrimSpace(u)
		if u != "" && !containsString(images, u) {
			images = append(images, u)
		}
	}

	switch v := raw.(type) {
	case string:
		addURL(v)
	case []interface{}:
		for _, item := range v {
			switch img := item.(type) {
			case string:
				addURL(img)
			case map[string]interface{}:
				if u := extractString(img, "url", "contentUrl"); u != "" {
					addURL(u)
				}
			}
		}
	case map[string]interface{}:
		if u := extractString(v, "url", "contentUrl"); u != "" {
			addURL(u)
		}
	}
	return images
}

// jsonLDProduct is maintained for backward compatibility.
type jsonLDProduct struct {
	Type        string `json:"@type"`
	Name        string `json:"name"`
	Description string `json:"description"`
	SKU         string `json:"sku"`
	Brand       *struct {
		Name string `json:"name"`
	} `json:"brand"`
	Offers *struct {
		Price         interface{} `json:"price"`
		PriceCurrency string      `json:"priceCurrency"`
		Availability  string      `json:"availability"`
	} `json:"offers"`
	Image           interface{} `json:"image"`
	AggregateRating *struct {
		RatingValue interface{} `json:"ratingValue"`
		ReviewCount interface{} `json:"reviewCount"`
	} `json:"aggregateRating"`
}

func extractFromJSONLD(doc *goquery.Selection) *jsonLDProduct {
	sd := ExtractJSONLD(doc)
	if sd == nil || sd.Name == "" {
		return nil
	}
	return &jsonLDProduct{
		Type:        sd.Type,
		Name:        sd.Name,
		Description: sd.Description,
		SKU:         sd.SKU,
	}
}
