package models

import "time"

// ScrapedPage represents a crawled page with all extracted data
type ScrapedPage struct {
	// Basic info
	URL           string    `json:"url"`
	Title         string    `json:"title"`
	H1            string    `json:"h1,omitempty"`
	Description   string    `json:"description,omitempty"`
	Keywords      string    `json:"keywords,omitempty"`
	Author        string    `json:"author,omitempty"`
	PublishedDate string    `json:"published_date,omitempty"`
	OGImage       string    `json:"og_image,omitempty"`
	BodyText      string    `json:"body_text,omitempty"`
	Canonical     string    `json:"canonical,omitempty"`

	// Technical info
	StatusCode    int    `json:"status_code"`
	ContentType   string `json:"content_type,omitempty"`
	ContentLength int64  `json:"content_length"`
	LoadTimeMs    int64  `json:"load_time_ms"`
	LinksCount    int    `json:"links_count"`
	ImagesCount   int    `json:"images_count"`
	Depth         int    `json:"depth"`

	// Product data (when in product mode)
	Product *ProductInfo `json:"product,omitempty"`

	// Page type detection
	PageType string `json:"page_type,omitempty"` // product, category, article, etc.

	// Timestamps
	ScrapedAt time.Time `json:"scraped_at"`
}

// ProductInfo holds product-specific extracted data
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
