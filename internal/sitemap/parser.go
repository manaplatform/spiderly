package sitemap

import (
	"bufio"
	"compress/gzip"
	"encoding/xml"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strings"
	"time"

	"spiderly/internal/models"
)

// Common sitemap locations to check
var commonSitemapPaths = []string{
	"/sitemap.xml",
	"/sitemap_index.xml",
	"/sitemap-index.xml",
	"/sitemaps.xml",
	"/sitemap1.xml",
	"/sitemap-0.xml",
	"/post-sitemap.xml",
	"/page-sitemap.xml",
	"/news-sitemap.xml",
	"/sitemap/sitemap.xml",
	"/sitemaps/sitemap.xml",
}

// Parser handles sitemap discovery and parsing
type Parser struct {
	client  *http.Client
	timeout time.Duration
	verbose bool
}

// NewParser creates a new sitemap parser
func NewParser(timeout time.Duration, verbose bool) *Parser {
	if timeout == 0 {
		timeout = 30 * time.Second
	}
	
	return &Parser{
		client: &http.Client{
			Timeout: timeout,
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				if len(via) >= 5 {
					return fmt.Errorf("too many redirects")
				}
				return nil
			},
		},
		timeout: timeout,
		verbose: verbose,
	}
}

// DiscoverSitemaps finds all sitemaps for a given domain
func (p *Parser) DiscoverSitemaps(baseURL string) ([]string, error) {
	parsed, err := url.Parse(baseURL)
	if err != nil {
		return nil, err
	}
	
	baseHost := fmt.Sprintf("%s://%s", parsed.Scheme, parsed.Host)
	discovered := make(map[string]bool)
	var sitemaps []string
	
	// 1. Check robots.txt first (highest priority)
	robotsSitemaps := p.parseRobotsTxt(baseHost + "/robots.txt")
	for _, sm := range robotsSitemaps {
		if !discovered[sm] {
			discovered[sm] = true
			sitemaps = append(sitemaps, sm)
			p.logVerbose("Found sitemap in robots.txt: %s", sm)
		}
	}
	
	// 2. Check common sitemap locations
	for _, path := range commonSitemapPaths {
		sitemapURL := baseHost + path
		if discovered[sitemapURL] {
			continue
		}
		
		if p.checkSitemapExists(sitemapURL) {
			discovered[sitemapURL] = true
			sitemaps = append(sitemaps, sitemapURL)
			p.logVerbose("Found sitemap at common path: %s", sitemapURL)
		}
	}
	
	// 3. Expand sitemap indices
	var expandedSitemaps []string
	for _, sm := range sitemaps {
		children, err := p.expandSitemapIndex(sm)
		if err != nil {
			p.logVerbose("Error expanding sitemap index %s: %v", sm, err)
			// Still include the original sitemap
			expandedSitemaps = append(expandedSitemaps, sm)
			continue
		}
		
		if len(children) > 0 {
			for _, child := range children {
				if !discovered[child] {
					discovered[child] = true
					expandedSitemaps = append(expandedSitemaps, child)
				}
			}
		} else {
			expandedSitemaps = append(expandedSitemaps, sm)
		}
	}
	
	return expandedSitemaps, nil
}

// parseRobotsTxt extracts sitemap URLs from robots.txt
func (p *Parser) parseRobotsTxt(robotsURL string) []string {
	var sitemaps []string
	
	resp, err := p.client.Get(robotsURL)
	if err != nil {
		p.logVerbose("Failed to fetch robots.txt: %v", err)
		return sitemaps
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		p.logVerbose("robots.txt returned status %d", resp.StatusCode)
		return sitemaps
	}
	
	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		
		// Skip comments and empty lines
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		
		// Look for Sitemap: directive (case-insensitive)
		if strings.HasPrefix(strings.ToLower(line), "sitemap:") {
			parts := strings.SplitN(line, ":", 2)
			if len(parts) == 2 {
				sitemapURL := strings.TrimSpace(parts[1])
				if sitemapURL != "" {
					sitemaps = append(sitemaps, sitemapURL)
				}
			}
		}
	}
	
	return sitemaps
}

// checkSitemapExists verifies if a sitemap URL is accessible
func (p *Parser) checkSitemapExists(sitemapURL string) bool {
	req, err := http.NewRequest(http.MethodHead, sitemapURL, nil)
	if err != nil {
		return false
	}
	
	req.Header.Set("User-Agent", "Spiderly/1.0 (+https://github.com/spiderly)")
	
	resp, err := p.client.Do(req)
	if err != nil {
		// Try GET as fallback (some servers don't support HEAD)
		resp, err = p.client.Get(sitemapURL)
		if err != nil {
			return false
		}
		defer resp.Body.Close()
	} else {
		defer resp.Body.Close()
	}
	
	// Check for success status and XML content type
	if resp.StatusCode != http.StatusOK {
		return false
	}
	
	contentType := resp.Header.Get("Content-Type")
	return strings.Contains(contentType, "xml") || 
	       strings.Contains(contentType, "text/plain") ||
	       strings.HasSuffix(sitemapURL, ".xml")
}

// expandSitemapIndex checks if a sitemap is an index and returns child sitemaps
func (p *Parser) expandSitemapIndex(sitemapURL string) ([]string, error) {
	body, err := p.fetchSitemap(sitemapURL)
	if err != nil {
		return nil, err
	}
	defer body.Close()
	
	data, err := io.ReadAll(body)
	if err != nil {
		return nil, err
	}
	
	// Try parsing as sitemap index
	var index models.SitemapIndex
	if err := xml.Unmarshal(data, &index); err == nil && len(index.Sitemaps) > 0 {
		var children []string
		for _, sm := range index.Sitemaps {
			if sm.Loc != "" {
				children = append(children, sm.Loc)
			}
		}
		p.logVerbose("Expanded sitemap index %s: found %d child sitemaps", sitemapURL, len(children))
		return children, nil
	}
	
	// Not an index, return empty
	return nil, nil
}

// ParseSitemap parses a sitemap and returns all URL entries
func (p *Parser) ParseSitemap(sitemapURL string) ([]models.SitemapEntry, error) {
	body, err := p.fetchSitemap(sitemapURL)
	if err != nil {
		return nil, err
	}
	defer body.Close()
	
	data, err := io.ReadAll(body)
	if err != nil {
		return nil, fmt.Errorf("failed to read sitemap body: %w", err)
	}
	
	// Try parsing as URL set
	var urlset models.Sitemap
	if err := xml.Unmarshal(data, &urlset); err != nil {
		return nil, fmt.Errorf("failed to parse sitemap XML: %w", err)
	}
	
	entries := make([]models.SitemapEntry, 0, len(urlset.URLs))
	for _, u := range urlset.URLs {
		if u.Loc == "" {
			continue
		}
		
		entry := models.SitemapEntry{
			URL:        u.Loc,
			LastMod:    u.LastMod,
			ChangeFreq: u.ChangeFreq,
			Priority:   u.Priority,
		}
		entries = append(entries, entry)
	}
	
	p.logVerbose("Parsed sitemap %s: %d URLs", sitemapURL, len(entries))
	return entries, nil
}

// fetchSitemap retrieves a sitemap with gzip support
func (p *Parser) fetchSitemap(sitemapURL string) (io.ReadCloser, error) {
	req, err := http.NewRequest(http.MethodGet, sitemapURL, nil)
	if err != nil {
		return nil, err
	}
	
	req.Header.Set("User-Agent", "Spiderly/1.0 (+https://github.com/spiderly)")
	req.Header.Set("Accept", "application/xml, text/xml, */*")
	req.Header.Set("Accept-Encoding", "gzip")
	
	resp, err := p.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch sitemap: %w", err)
	}
	
	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		return nil, fmt.Errorf("sitemap returned status %d", resp.StatusCode)
	}
	
	// Handle gzip compression
	var reader io.ReadCloser = resp.Body
	if resp.Header.Get("Content-Encoding") == "gzip" || strings.HasSuffix(sitemapURL, ".gz") {
		gzReader, err := gzip.NewReader(resp.Body)
		if err != nil {
			resp.Body.Close()
			return nil, fmt.Errorf("failed to create gzip reader: %w", err)
		}
		reader = &gzipReadCloser{gzReader, resp.Body}
	}
	
	return reader, nil
}

// gzipReadCloser wraps a gzip reader to close both readers
type gzipReadCloser struct {
	*gzip.Reader
	underlying io.ReadCloser
}

func (g *gzipReadCloser) Close() error {
	g.Reader.Close()
	return g.underlying.Close()
}

func (p *Parser) logVerbose(format string, args ...interface{}) {
	if p.verbose {
		log.Printf("[SITEMAP] "+format, args...)
	}
}
