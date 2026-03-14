package sitemap

import (
	"encoding/xml"
	"fmt"
	"strings"
	"time"
)

// ─────────────────────────────────────────────
//  XML Types
// ─────────────────────────────────────────────

// URLSet represents a <urlset> sitemap document.
type URLSet struct {
	XMLName xml.Name `xml:"urlset"`
	URLs    []URL    `xml:"url"`
}

// URL is a single <url> entry inside a <urlset>.
type URL struct {
	Loc        string  `xml:"loc"`
	LastMod    string  `xml:"lastmod"`
	ChangeFreq string  `xml:"changefreq"`
	Priority   float32 `xml:"priority"`
}

// SitemapIndex represents a <sitemapindex> document.
type SitemapIndex struct {
	XMLName  xml.Name  `xml:"sitemapindex"`
	Sitemaps []Sitemap `xml:"sitemap"`
}

// Sitemap is a single <sitemap> entry inside a <sitemapindex>.
type Sitemap struct {
	Loc     string `xml:"loc"`
	LastMod string `xml:"lastmod"`
}

// SitemapData is the unified result of parsing any sitemap document.
type SitemapData struct {
	// Type is either "urlset" or "sitemapindex".
	Type string

	// URLs is populated when Type == "urlset".
	URLs []URL

	// Sitemaps is populated when Type == "sitemapindex".
	Sitemaps []Sitemap

	// SourceURL is the URL this data was fetched from.
	SourceURL string

	// FetchedAt records when the fetch completed.
	FetchedAt time.Time
}

// sitemapKind represents the detected root element type.
type sitemapKind int

const (
	kindUnknown sitemapKind = iota
	kindURLSet
	kindSitemapIndex
)

// ─────────────────────────────────────────────
//  Root Element Peeking
// ─────────────────────────────────────────────

// peekRootElement reads just enough XML tokens to identify the root element
// without unmarshalling the entire document. This replaces the old approach
// of unmarshalling twice (once as urlset, once as sitemapindex).
func peekRootElement(data []byte) sitemapKind {
	decoder := xml.NewDecoder(strings.NewReader(string(data)))

	for {
		tok, err := decoder.Token()
		if err != nil {
			return kindUnknown
		}

		start, ok := tok.(xml.StartElement)
		if !ok {
			continue // skip xml declarations, comments, processing instructions
		}

		// First start element is the root
		switch strings.ToLower(start.Name.Local) {
		case "urlset":
			return kindURLSet
		case "sitemapindex":
			return kindSitemapIndex
		default:
			return kindUnknown
		}
	}
}

// ─────────────────────────────────────────────
//  Decoding
// ─────────────────────────────────────────────

// decode parses raw XML bytes into a SitemapData struct.
// It peeks at the root element first, then unmarshals only once into the correct type.
func (p *Parser) decode(data []byte, sourceURL string) (*SitemapData, error) {
	kind := peekRootElement(data)

	switch kind {
	case kindURLSet:
		return p.decodeURLSet(data, sourceURL)
	case kindSitemapIndex:
		return p.decodeSitemapIndex(data, sourceURL)
	default:
		// Last resort: try both (handles edge cases like missing xmlns)
		return p.decodeFallback(data, sourceURL)
	}
}

// decodeURLSet unmarshals data as a <urlset> document.
func (p *Parser) decodeURLSet(data []byte, sourceURL string) (*SitemapData, error) {
	var urlset URLSet
	if err := xml.Unmarshal(data, &urlset); err != nil {
		return nil, fmt.Errorf("failed to parse urlset from %s: %w", sourceURL, err)
	}

	// Filter out entries with empty locations
	urls := make([]URL, 0, len(urlset.URLs))
	for i := range urlset.URLs {
		loc := strings.TrimSpace(urlset.URLs[i].Loc)
		if loc != "" {
			urlset.URLs[i].Loc = loc
			urls = append(urls, urlset.URLs[i])
		}
	}

	p.logVerbose("Decoded urlset with %d URLs from %s", len(urls), sourceURL)

	return &SitemapData{
		Type:      "urlset",
		URLs:      urls,
		SourceURL: sourceURL,
		FetchedAt: time.Now(),
	}, nil
}

// decodeSitemapIndex unmarshals data as a <sitemapindex> document.
func (p *Parser) decodeSitemapIndex(data []byte, sourceURL string) (*SitemapData, error) {
	var index SitemapIndex
	if err := xml.Unmarshal(data, &index); err != nil {
		return nil, fmt.Errorf("failed to parse sitemapindex from %s: %w", sourceURL, err)
	}

	// Filter out entries with empty locations
	sitemaps := make([]Sitemap, 0, len(index.Sitemaps))
	for i := range index.Sitemaps {
		loc := strings.TrimSpace(index.Sitemaps[i].Loc)
		if loc != "" {
			index.Sitemaps[i].Loc = loc
			sitemaps = append(sitemaps, index.Sitemaps[i])
		}
	}

	p.logVerbose("Decoded sitemapindex with %d child sitemaps from %s", len(sitemaps), sourceURL)

	return &SitemapData{
		Type:      "sitemapindex",
		Sitemaps:  sitemaps,
		SourceURL: sourceURL,
		FetchedAt: time.Now(),
	}, nil
}

// decodeFallback handles documents where peekRootElement returned kindUnknown.
// This covers malformed XML, missing namespaces, or non-standard root element names.
// Tries urlset first (more common), then sitemapindex.
func (p *Parser) decodeFallback(data []byte, sourceURL string) (*SitemapData, error) {
	p.logVerbose("Root element unknown for %s, trying fallback decode", sourceURL)

	// Try urlset first — it's the more common type
	var urlset URLSet
	if err := xml.Unmarshal(data, &urlset); err == nil && len(urlset.URLs) > 0 {
		return p.decodeURLSet(data, sourceURL)
	}

	// Try sitemapindex
	var index SitemapIndex
	if err := xml.Unmarshal(data, &index); err == nil && len(index.Sitemaps) > 0 {
		return p.decodeSitemapIndex(data, sourceURL)
	}

	return nil, fmt.Errorf("unrecognized sitemap format from %s", sourceURL)
}
