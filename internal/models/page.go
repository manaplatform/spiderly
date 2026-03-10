package models

import "time"

// ScrapedPage represents a fully scraped web page with all extracted data
type ScrapedPage struct {
	URL           string    `json:"url"`
	Title         string    `json:"title,omitempty"`
	H1            string    `json:"h1,omitempty"`
	Description   string    `json:"description,omitempty"`
	Keywords      string    `json:"keywords,omitempty"`
	Author        string    `json:"author,omitempty"`
	OGImage       string    `json:"og_image,omitempty"`
	PublishedDate string    `json:"published_date,omitempty"`
	ContentType   string    `json:"content_type,omitempty"`
	BodyText      string    `json:"body_text,omitempty"`
	StatusCode    int       `json:"status_code"`
	ContentLength int64     `json:"content_length,omitempty"`
	LoadTimeMs    int64     `json:"load_time_ms,omitempty"`
	Depth         int       `json:"depth"`
	LinksCount    int       `json:"links_count,omitempty"`
	ImagesCount   int       `json:"images_count,omitempty"`
	ScrapedAt     time.Time `json:"scraped_at"`
}
