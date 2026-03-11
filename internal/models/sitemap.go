package models

import "encoding/xml"

// SitemapIndex represents a sitemap index file
type SitemapIndex struct {
	XMLName  xml.Name         `xml:"sitemapindex"`
	Sitemaps []SitemapLocation `xml:"sitemap"`
}

// SitemapLocation represents a sitemap reference in an index
type SitemapLocation struct {
	Loc     string `xml:"loc"`
	LastMod string `xml:"lastmod,omitempty"`
}

// Sitemap represents a URL set sitemap
type Sitemap struct {
	XMLName xml.Name     `xml:"urlset"`
	URLs    []SitemapURL `xml:"url"`
}

// SitemapURL represents a single URL entry in a sitemap
type SitemapURL struct {
	Loc        string  `xml:"loc"`
	LastMod    string  `xml:"lastmod,omitempty"`
	ChangeFreq string  `xml:"changefreq,omitempty"`
	Priority   float64 `xml:"priority,omitempty"`
}

// SitemapEntry is the parsed result for internal use
type SitemapEntry struct {
	URL        string
	LastMod    string
	ChangeFreq string
	Priority   float64
	Type       string // pdp, plp, static, etc.
}

// SitemapStats holds statistics about discovered sitemaps
type SitemapStats struct {
	TotalSitemaps int
	TotalURLs     int
	ByType        map[string]int
}
