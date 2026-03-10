package models

import (
	"encoding/xml"
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

// CrawlStats holds crawling statistics
type CrawlStats struct {
	TotalURLs     int     `json:"total_urls"`
	ProcessedURLs int     `json:"processed_urls"`
	SuccessCount  int     `json:"success_count"`
	ErrorCount    int     `json:"error_count"`
	CurrentURL    string  `json:"current_url"`
	ElapsedTime   string  `json:"elapsed_time"`
	Progress      float64 `json:"progress"`
	SitemapMode   bool    `json:"sitemap_mode"`
	SitemapURLs   int     `json:"sitemap_urls"`
	SitemapsFound int     `json:"sitemaps_found"`
}

// WebSocketMessage represents a message sent to the dashboard
type WebSocketMessage struct {
	Type    string      `json:"type"`
	Payload interface{} `json:"payload"`
}

// LogEntry represents a log message
type LogEntry struct {
	Level     string    `json:"level"`
	Message   string    `json:"message"`
	Timestamp time.Time `json:"timestamp"`
}

// DiscoveredLink represents a discovered URL
type DiscoveredLink struct {
	URL      string `json:"url"`
	Source   string `json:"source"`
	Depth    int    `json:"depth"`
	Priority string `json:"priority,omitempty"`
	LastMod  string `json:"lastmod,omitempty"`
}

// SitemapURL represents a single URL entry in a sitemap
type SitemapURL struct {
	Loc        string  `xml:"loc" json:"loc"`
	LastMod    string  `xml:"lastmod,omitempty" json:"lastmod,omitempty"`
	ChangeFreq string  `xml:"changefreq,omitempty" json:"changefreq,omitempty"`
	Priority   float64 `xml:"priority,omitempty" json:"priority,omitempty"`
}

// Sitemap represents a standard sitemap.xml structure
type Sitemap struct {
	XMLName xml.Name     `xml:"urlset"`
	URLs    []SitemapURL `xml:"url"`
}

// SitemapIndex represents a sitemap index file
type SitemapIndex struct {
	XMLName  xml.Name       `xml:"sitemapindex"`
	Sitemaps []SitemapEntry `xml:"sitemap"`
}

// SitemapEntry represents an entry in a sitemap index
type SitemapEntry struct {
	Loc     string `xml:"loc" json:"loc"`
	LastMod string `xml:"lastmod,omitempty" json:"lastmod,omitempty"`
}

// SitemapResult holds the result of sitemap parsing
type SitemapResult struct {
	URLs          []SitemapURL `json:"urls"`
	SitemapsFound int          `json:"sitemaps_found"`
	TotalURLs     int          `json:"total_urls"`
	Source        string       `json:"source"`
	ParsedAt      time.Time    `json:"parsed_at"`
}
