package models

type ProductInfo struct {
	Name          string            `json:"name,omitempty"`
	Brand         string            `json:"brand,omitempty"`
	SKU           string            `json:"sku,omitempty"`
	GTIN          string            `json:"gtin,omitempty"`
	MPN           string            `json:"mpn,omitempty"`
	Price         float64           `json:"price,omitempty"`
	Currency      string            `json:"currency,omitempty"`
	OriginalPrice float64           `json:"original_price,omitempty"`
	Discount      float64           `json:"discount,omitempty"`
	Availability  string            `json:"availability,omitempty"`
	InStock       bool              `json:"in_stock"`
	Rating        float64           `json:"rating,omitempty"`
	ReviewCount   int               `json:"review_count,omitempty"`
	Category      string            `json:"category,omitempty"`
	Categories    []string          `json:"categories,omitempty"`
	Images        []string          `json:"images,omitempty"`
	Description   string            `json:"description,omitempty"`
	Specs         map[string]string `json:"specs,omitempty"`
	
	// Multi-seller support
	Prices    []PriceInfo `json:"prices,omitempty"`
	SourceURL string      `json:"source_url,omitempty"`

	// ✅ NEW METADATA FIELDS
	Keywords       string
	CanonicalURL  string
}
