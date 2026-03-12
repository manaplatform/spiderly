package models

type ProductInfo struct {
	Name          string            `json:"name,omitempty"`
	Price         float64           `json:"price,omitempty"`
	OriginalPrice float64           `json:"original_price,omitempty"`
	Discount      float64           `json:"discount,omitempty"`
	Currency      string            `json:"currency,omitempty"`
	Description   string            `json:"description,omitempty"`
	SKU           string            `json:"sku,omitempty"`
	GTIN          string            `json:"gtin,omitempty"`
	MPN           string            `json:"mpn,omitempty"`
	Brand         string            `json:"brand,omitempty"`
	Availability  string            `json:"availability,omitempty"`
	InStock       bool              `json:"in_stock,omitempty"`
	Rating        float64           `json:"rating,omitempty"`
	ReviewCount   int               `json:"review_count,omitempty"`
	Category      string            `json:"category,omitempty"`
	Categories    []string          `json:"categories,omitempty"`
	Specs         map[string]string `json:"specs,omitempty"`
	Images        []string          `json:"images,omitempty"`
}
