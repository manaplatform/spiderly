package sitemap

import (
	"bufio"
	"bytes"
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

// Common product URL path segments used for heuristic detection
var productPathSegments = []string{
	"/product/", "/products/",
	"/p/", "/pd/", "/pdp/",
	"/item/", "/items/",
	"/goods/",
	"-p-",
	"/dp/",       // Amazon-style
	"/ip/",       // Walmart-style
	"/catalog/",  // Some e-commerce sites
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

// ─────────────────────────────────────────────
//  Discovery
// ─────────────────────────────────────────────

// DiscoverSitemaps finds all sitemaps for a given domain.
// It checks robots.txt, common paths, and recursively expands all sitemap indices.
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

	// 3. Recursively expand sitemap indices (handles nested indices and .gz files)
	var expandedSitemaps []string
	for _, sm := range sitemaps {
		children := p.expandSitemapIndexRecursive(sm, discovered, 0)
		if len(children) > 0 {
			expandedSitemaps = append(expandedSitemaps, children...)
		} else {
			// Not an index — it's a leaf sitemap itself
			expandedSitemaps = append(expandedSitemaps, sm)
		}
	}

	return expandedSitemaps, nil
}

// DiscoverSitemapsFiltered finds sitemaps matching specific type keywords.
// For product mode you'd pass typeFilters like: []string{"pdp", "product"}
func (p *Parser) DiscoverSitemapsFiltered(baseURL string, typeFilters []string) ([]string, error) {
	allSitemaps, err := p.DiscoverSitemaps(baseURL)
	if err != nil {
		return nil, err
	}

	if len(typeFilters) == 0 {
		return allSitemaps, nil
	}

	var filtered []string
	for _, sm := range allSitemaps {
		smLower := strings.ToLower(sm)
		for _, filter := range typeFilters {
			if strings.Contains(smLower, strings.ToLower(filter)) {
				filtered = append(filtered, sm)
				p.logVerbose("Sitemap matches filter '%s': %s", filter, sm)
				break
			}
		}
	}

	return filtered, nil
}

// ─────────────────────────────────────────────
//  Recursive Index Expansion
// ─────────────────────────────────────────────

// maxRecursionDepth prevents infinite loops in malformed sitemaps
const maxRecursionDepth = 5

// expandSitemapIndexRecursive fetches a sitemap, checks if it's an index,
// and recursively expands all children. Returns only leaf (non-index) sitemaps.
// The `seen` map prevents cycles, and `depth` caps recursion.
func (p *Parser) expandSitemapIndexRecursive(sitemapURL string, seen map[string]bool, depth int) []string {
	if depth > maxRecursionDepth {
		p.logVerbose("Max recursion depth reached for: %s", sitemapURL)
		return nil
	}

	data, err := p.fetchAndDecompress(sitemapURL)
	if err != nil {
		p.logVerbose("Failed to fetch sitemap for expansion: %s — %v", sitemapURL, err)
		return nil
	}

	// Try parsing as a sitemap index
	var index models.SitemapIndex
	if err := xml.Unmarshal(data, &index); err == nil && len(index.Sitemaps) > 0 {
		p.logVerbose("Expanded sitemap index %s: found %d child sitemaps (depth=%d)",
			sitemapURL, len(index.Sitemaps), depth)

		var leaves []string
		for _, sm := range index.Sitemaps {
			childURL := strings.TrimSpace(sm.Loc)
			if childURL == "" || seen[childURL] {
				continue
			}
			seen[childURL] = true
			p.logVerbose("  child: %s", childURL)

			// Recurse — the child could itself be another index
			grandChildren := p.expandSitemapIndexRecursive(childURL, seen, depth+1)
			if len(grandChildren) > 0 {
				leaves = append(leaves, grandChildren...)
			} else {
				// It's a leaf sitemap (contains <urlset>, not <sitemapindex>)
				leaves = append(leaves, childURL)
			}
		}
		return leaves
	}

	// Not an index — return nil so the caller knows this is a leaf
	return nil
}

// ─────────────────────────────────────────────
//  Parsing
// ─────────────────────────────────────────────

// ParseSitemap parses a sitemap and returns all URL entries.
// If the sitemap is actually an index, it recursively parses all children.
func (p *Parser) ParseSitemap(sitemapURL string) ([]models.SitemapEntry, error) {
	p.logVerbose("Parsing sitemap: %s", sitemapURL)

	data, err := p.fetchAndDecompress(sitemapURL)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch sitemap: %w", err)
	}

	p.logVerbose("Fetched %d bytes from %s", len(data), sitemapURL)

	// Try parsing as URL set first
	var urlset models.Sitemap
	if err := xml.Unmarshal(data, &urlset); err == nil && len(urlset.URLs) > 0 {
		sitemapType := GetSitemapType(sitemapURL)
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
				Type:       sitemapType,
			}
			entries = append(entries, entry)
		}
		p.logVerbose("Parsed sitemap %s: %d URLs (type=%s)", sitemapURL, len(entries), sitemapType)
		return entries, nil
	}

	// Try parsing as sitemap index — recursively parse each child
	var index models.SitemapIndex
	if xmlErr := xml.Unmarshal(data, &index); xmlErr == nil && len(index.Sitemaps) > 0 {
		p.logVerbose("Sitemap %s is an index with %d children — parsing recursively", sitemapURL, len(index.Sitemaps))
		var allEntries []models.SitemapEntry
		for _, sm := range index.Sitemaps {
			childURL := strings.TrimSpace(sm.Loc)
			if childURL == "" {
				continue
			}
			childEntries, childErr := p.ParseSitemap(childURL)
			if childErr != nil {
				p.logVerbose("Failed to parse child sitemap %s: %v", childURL, childErr)
				continue
			}
			allEntries = append(allEntries, childEntries...)
		}
		return allEntries, nil
	}

	// If urlset parse had an error and it's not an index, return the error
	if err != nil {
		return nil, fmt.Errorf("failed to parse sitemap XML from %s: %w", sitemapURL, err)
	}

	// Empty sitemap
	p.logVerbose("Sitemap %s parsed but contained 0 URLs", sitemapURL)
	return nil, nil
}

// ─────────────────────────────────────────────
//  Robots.txt
// ─────────────────────────────────────────────

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

		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		// Handle "Sitemap: <url>" lines (case-insensitive prefix)
		if strings.HasPrefix(strings.ToLower(line), "sitemap:") {
			// Split on first colon only would break http:// — split on "Sitemap:" prefix
			idx := strings.Index(strings.ToLower(line), "sitemap:")
			sitemapURL := strings.TrimSpace(line[idx+len("sitemap:"):])
			if sitemapURL != "" {
				sitemaps = append(sitemaps, sitemapURL)
			}
		}
	}

	return sitemaps
}

// ─────────────────────────────────────────────
//  Existence Check
// ─────────────────────────────────────────────

// checkSitemapExists verifies if a sitemap URL is accessible
func (p *Parser) checkSitemapExists(sitemapURL string) bool {
	req, err := http.NewRequest(http.MethodHead, sitemapURL, nil)
	if err != nil {
		return false
	}

	req.Header.Set("User-Agent", "Spiderly/1.0 (+https://github.com/spiderly)")

	resp, err := p.client.Do(req)
	if err != nil {
		// HEAD failed, try GET as fallback
		resp, err = p.client.Get(sitemapURL)
		if err != nil {
			return false
		}
		defer resp.Body.Close()
	} else {
		defer resp.Body.Close()
	}

	if resp.StatusCode != http.StatusOK {
		return false
	}

	contentType := resp.Header.Get("Content-Type")
	return strings.Contains(contentType, "xml") ||
		strings.Contains(contentType, "text/plain") ||
		strings.Contains(contentType, "application/gzip") ||
		strings.Contains(contentType, "application/x-gzip") ||
		strings.HasSuffix(sitemapURL, ".xml") ||
		strings.HasSuffix(sitemapURL, ".xml.gz") ||
		strings.HasSuffix(sitemapURL, ".gz")
}

// ─────────────────────────────────────────────
//  Fetch & Decompress (gzip-aware)
// ─────────────────────────────────────────────

// fetchAndDecompress fetches a sitemap URL and transparently handles gzip decompression.
// Detection uses four methods: Content-Encoding header, Content-Type header, URL suffix, and magic bytes.
func (p *Parser) fetchAndDecompress(sitemapURL string) ([]byte, error) {
	req, err := http.NewRequest(http.MethodGet, sitemapURL, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("User-Agent", "Spiderly/1.0 (+https://github.com/spiderly)")
	req.Header.Set("Accept", "application/xml, text/xml, application/gzip, */*")
	// Note: We intentionally do NOT set Accept-Encoding here.
	// If we set "Accept-Encoding: gzip", the HTTP transport may auto-decompress
	// and we'd lose the ability to detect .gz files that are gzipped at the content level
	// (not just transfer encoding). Instead we handle decompression ourselves.

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch sitemap: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("sitemap returned status %d", resp.StatusCode)
	}

	// Read all body data
	bodyData, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	// Determine if decompression is needed
	isGzipped := false

	// Method 1: Content-Encoding header
	contentEncoding := resp.Header.Get("Content-Encoding")
	if strings.Contains(strings.ToLower(contentEncoding), "gzip") {
		isGzipped = true
		p.logVerbose("Detected gzip from Content-Encoding header")
	}

	// Method 2: Content-Type header
	contentType := resp.Header.Get("Content-Type")
	if strings.Contains(contentType, "gzip") || strings.Contains(contentType, "x-gzip") {
		isGzipped = true
		p.logVerbose("Detected gzip from Content-Type header")
	}

	// Method 3: URL suffix
	if strings.HasSuffix(strings.ToLower(sitemapURL), ".gz") {
		isGzipped = true
		p.logVerbose("Detected gzip from URL suffix (.gz)")
	}

	// Method 4: Magic bytes (gzip always starts with 0x1f 0x8b)
	if len(bodyData) >= 2 && bodyData[0] == 0x1f && bodyData[1] == 0x8b {
		isGzipped = true
		p.logVerbose("Detected gzip from magic bytes")
	}

	// Decompress if needed
	if isGzipped {
		p.logVerbose("Decompressing gzip data (%d bytes)", len(bodyData))
		gzReader, err := gzip.NewReader(bytes.NewReader(bodyData))
		if err != nil {
			return nil, fmt.Errorf("failed to create gzip reader: %w", err)
		}
		defer gzReader.Close()

		decompressed, err := io.ReadAll(gzReader)
		if err != nil {
			return nil, fmt.Errorf("failed to decompress gzip: %w", err)
		}
		p.logVerbose("Decompressed to %d bytes", len(decompressed))
		return decompressed, nil
	}

	return bodyData, nil
}

// ─────────────────────────────────────────────
//  Product URL Heuristics
// ─────────────────────────────────────────────

// IsProductSitemap checks if a sitemap URL looks like it contains product pages.
// E.g., "sitemap-pdp-1.xml.gz", "product-sitemap.xml", etc.
func IsProductSitemap(sitemapURL string) bool {
	lower := strings.ToLower(sitemapURL)
	productKeywords := []string{"pdp", "product", "item", "goods", "catalog"}
	for _, kw := range productKeywords {
		if strings.Contains(lower, kw) {
			return true
		}
	}
	return false
}

// IsLikelyProductURL checks if a page URL looks like a product detail page.
// Uses common e-commerce URL patterns.
func IsLikelyProductURL(pageURL string) bool {
	lower := strings.ToLower(pageURL)
	for _, seg := range productPathSegments {
		if strings.Contains(lower, seg) {
			return true
		}
	}

	// Additional heuristic: URLs ending with a numeric/alphanumeric SKU-like segment
	// e.g., /some-product-name-12345 or /some-product-name/12345
	parts := strings.Split(strings.TrimRight(lower, "/"), "/")
	if len(parts) > 0 {
		last := parts[len(parts)-1]
		// If the last path segment contains a mix of letters, digits, and hyphens
		// and is reasonably long, it might be a product slug
		if len(last) > 10 && strings.ContainsAny(last, "0123456789") && strings.Contains(last, "-") {
			return true
		}
	}

	return false
}

// GetSitemapType extracts the type from sitemap URL (e.g., "pdp", "plp", "static")
func GetSitemapType(sitemapURL string) string {
	urlLower := strings.ToLower(sitemapURL)

	// WordPress-style compound types: product_cat, product_tag → treat as "product"
	if strings.Contains(urlLower, "product_cat") || strings.Contains(urlLower, "product_tag") {
		return "product"
	}

	types := []string{"pdp", "plp", "product", "category", "static", "landing", "blog", "news", "video", "image"}
	for _, t := range types {
		if strings.Contains(urlLower, "-"+t+"-") || strings.Contains(urlLower, "_"+t+"_") ||
			strings.Contains(urlLower, "/"+t+"-") || strings.Contains(urlLower, "-"+t+".") ||
			strings.Contains(urlLower, "/"+t+"/") || strings.Contains(urlLower, "_"+t+".") ||
			strings.Contains(urlLower, "/"+t+"_") || strings.Contains(urlLower, "-"+t+"_") {
			return t
		}
	}

	return "unknown"
}


// ─────────────────────────────────────────────
//  Logging
// ─────────────────────────────────────────────

func (p *Parser) logVerbose(format string, args ...interface{}) {
	if p.verbose {
		log.Printf("[SITEMAP] "+format, args...)
	}
}
