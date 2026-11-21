package models

import "time"

// ScrapedPage represents a crawled page with all extracted data
type ScrapedPage struct {
	// Basic info
	URL           string `json:"url"`
	Title         string `json:"title"`
	H1            string `json:"h1,omitempty"`
	Description   string `json:"description,omitempty"`
	Keywords      string `json:"keywords,omitempty"`
	Author        string `json:"author,omitempty"`
	PublishedDate string `json:"published_date,omitempty"`
	OGImage       string `json:"og_image,omitempty"`
	BodyText      string `json:"body_text,omitempty"`
	Canonical     string `json:"canonical,omitempty"`

	// Technical info
	StatusCode    int    `json:"status_code"`
	ContentType   string `json:"content_type,omitempty"`
	ContentLength int64  `json:"content_length"`
	LoadTimeMs    int64  `json:"load_time_ms"`
	LinksCount    int    `json:"links_count"`
	ImagesCount   int    `json:"images_count"`
	Depth         int    `json:"depth"`
	ParentURL     string `json:"parent_url,omitempty"`
	ChildURLs     []string `json:"child_urls,omitempty"`
	Category      string `json:"category,omitempty"`
	Breadcrumbs   []string `json:"breadcrumbs,omitempty"`
	Content       string `json:"content,omitempty"`
	// Product data (when in product mode or detected)
	Product *ProductInfo `json:"product,omitempty"`
	News    *NewsData    `json:"news,omitempty"`

	// Structured data extracted from JSON-LD / schema.org
	StructuredData *StructuredData `json:"structured_data,omitempty"`

	// Page type detection
	PageType string `json:"page_type,omitempty"` // product, category, article, etc.

	// Timestamps
	ScrapedAt time.Time `json:"scraped_at"`
}

// PriceInfo holds price information from a seller/offer
type PriceInfo struct {
	Amount   float64 `json:"amount,omitempty"`
	Currency string  `json:"currency,omitempty"`
	Seller   string  `json:"seller,omitempty"`
	InStock  bool    `json:"in_stock"`
}

// StructuredData represents normalized structured data extracted from JSON-LD / schema.org.
type StructuredData struct {
	Type          string            `json:"type,omitempty"`
	Name          string            `json:"name,omitempty"`
	Description   string            `json:"description,omitempty"`
	Brand         string            `json:"brand,omitempty"`
	SKU           string            `json:"sku,omitempty"`
	GTIN          string            `json:"gtin,omitempty"`
	MPN           string            `json:"mpn,omitempty"`
	Price         float64           `json:"price,omitempty"`
	OriginalPrice float64           `json:"original_price,omitempty"`
	Discount      float64           `json:"discount,omitempty"`
	Currency      string            `json:"currency,omitempty"`
	Availability  string            `json:"availability,omitempty"`
	InStock       bool              `json:"in_stock"`
	Rating        float64           `json:"rating,omitempty"`
	ReviewCount   int               `json:"review_count,omitempty"`
	Category      string            `json:"category,omitempty"`
	Breadcrumbs   []string          `json:"breadcrumbs,omitempty"`
	Images        []string          `json:"images,omitempty"`
	Specs         map[string]string `json:"specs,omitempty"`
	RawTypes      []string          `json:"raw_types,omitempty"`
}

// ToProductInfo converts StructuredData to a ProductInfo model.
func (sd *StructuredData) ToProductInfo() *ProductInfo {
	if sd == nil {
		return nil
	}
	return &ProductInfo{
		Name:          sd.Name,
		Price:         sd.Price,
		OriginalPrice: sd.OriginalPrice,
		Discount:      sd.Discount,
		Currency:      sd.Currency,
		Description:   sd.Description,
		SKU:           sd.SKU,
		GTIN:          sd.GTIN,
		MPN:           sd.MPN,
		Brand:         sd.Brand,
		Availability:  sd.Availability,
		InStock:       sd.InStock,
		Rating:        sd.Rating,
		ReviewCount:   sd.ReviewCount,
		Category:      sd.Category,
		Categories:    sd.Breadcrumbs,
		Specs:         sd.Specs,
		Images:        sd.Images,
	}
}

// CrawlStats tracks crawl progress
type CrawlStats struct {
	PagesScraped int
	PagesQueued  int
	Errors       int
	StartTime    time.Time
}

// DiscoveredLink represents a link found during crawling.
//
// This struct is used by the crawler when it discovers new links on a page.
// Fields:
//   - URL:        The discovered link's absolute URL.
//   - Depth:      The crawl depth at which this link was found.
//   - Source:     A short label for where the link came from (e.g., "sitemap", "page").
//   - SourceURL:  The full URL of the page where this link was discovered.
//   - AnchorText: The visible text of the <a> tag that contained this link.
type DiscoveredLink struct {
	URL        string // The discovered URL
	Depth      int    // Crawl depth at which this link exists
	Source     string // Origin label: "sitemap", "page", "redirect", etc.
	SourceURL  string // The page URL where this link was found
	AnchorText string // The anchor text of the <a> element
}
