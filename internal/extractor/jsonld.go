// internal/extractor/jsonld.go
package extractor

import (
	"encoding/json"
	"strings"

	"github.com/PuerkitoBio/goquery"
)

// jsonLDProduct represents a Product in JSON-LD schema.org format.
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

// extractFromJSONLD scans all ld+json script blocks for a Product type.
func extractFromJSONLD(doc *goquery.Selection) *jsonLDProduct {
	var result *jsonLDProduct

	doc.Find(`script[type="application/ld+json"]`).Each(func(i int, s *goquery.Selection) {
		if result != nil {
			return
		}

		raw := strings.TrimSpace(s.Text())
		if raw == "" {
			return
		}

		result = tryParseProduct(raw)
		if result != nil {
			return
		}

		result = tryParseGraph(raw)
	})

	return result
}

func tryParseProduct(raw string) *jsonLDProduct {
	var obj jsonLDProduct
	if err := json.Unmarshal([]byte(raw), &obj); err != nil {
		return nil
	}
	if obj.Type == "Product" && obj.Name != "" {
		return &obj
	}
	return nil
}

func tryParseGraph(raw string) *jsonLDProduct {
	var wrapper struct {
		Graph []json.RawMessage `json:"@graph"`
	}
	if err := json.Unmarshal([]byte(raw), &wrapper); err != nil || len(wrapper.Graph) == 0 {
		return nil
	}

	for _, item := range wrapper.Graph {
		if p := tryParseProduct(string(item)); p != nil {
			return p
		}
	}
	return nil
}
