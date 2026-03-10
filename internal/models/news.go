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


// LogEntry represents a log message
type LogEntry struct {
	Level     string    `json:"level"`
	Message   string    `json:"message"`
	Timestamp time.Time `json:"timestamp"`
}




// Sitemap represents a standard sitemap.xml structure
type Sitemap struct {
	XMLName xml.Name     `xml:"urlset"`
	URLs    []SitemapURL `xml:"url"`
}




// SitemapResult holds the result of sitemap parsing
type SitemapResult struct {
	URLs          []SitemapURL `json:"urls"`
	SitemapsFound int          `json:"sitemaps_found"`
	TotalURLs     int          `json:"total_urls"`
	Source        string       `json:"source"`
	ParsedAt      time.Time    `json:"parsed_at"`
}


// Sitemap XML structures
type SitemapURL struct {
	XMLName    xml.Name `xml:"url"`
	Loc        string   `xml:"loc"`
	LastMod    string   `xml:"lastmod,omitempty"`
	ChangeFreq string   `xml:"changefreq,omitempty"`
	Priority   float64  `xml:"priority,omitempty"`
}



type SitemapIndexEntry struct {
	Loc     string `xml:"loc"`
	LastMod string `xml:"lastmod,omitempty"`
}

type SitemapIndex struct {
	XMLName  xml.Name            `xml:"sitemapindex"`
	Sitemaps []SitemapIndexEntry `xml:"sitemap"`
}

// SitemapEntry represents a parsed sitemap URL with metadata
type SitemapEntry struct {
	URL        string
	LastMod    string
	ChangeFreq string
	Priority   float64
}

// DiscoveredLink represents a link found during crawling
type DiscoveredLink struct {
	URL       string
	SourceURL string
	Depth     int
	AnchorText string
}

// CrawlStats holds real-time crawling statistics
type CrawlStats struct {
	PagesScraped   int
	PagesQueued    int
	Errors         int
	SitemapURLs    int
	StartTime      time.Time
	CurrentURL     string
	Mode           string
}

// WebSocketMessage is the structure for real-time dashboard updates
type WebSocketMessage struct {
	Type string                 `json:"type"`
	Data map[string]interface{} `json:"data,omitempty"`
}