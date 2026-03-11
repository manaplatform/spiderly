package models

import (
	"time"
)

// News represents a scraped news article
type News struct {
	URL         string    `json:"url"`
	Title       string    `json:"title"`
	Content     string    `json:"content"`
	Author      string    `json:"author"`
	PublishDate string    `json:"publish_date"`
	Tags        []string  `json:"tags"`
	ScrapedAt   time.Time `json:"scraped_at"`
	Depth       int       `json:"depth"`
}

// LogEntry represents a log message
type LogEntry struct {
	Level     string    `json:"level"`
	Message   string    `json:"message"`
	Timestamp time.Time `json:"timestamp"`
}

// SitemapResult holds the result of sitemap parsing
type SitemapResult struct {
	URLs          []SitemapURL `json:"urls"`
	SitemapsFound int          `json:"sitemaps_found"`
	TotalURLs     int          `json:"total_urls"`
	Source        string       `json:"source"`
	ParsedAt      time.Time    `json:"parsed_at"`
}

type SitemapIndexEntry struct {
	Loc     string `xml:"loc"`
	LastMod string `xml:"lastmod,omitempty"`
}
