package sitemap

import (
	"bufio"
	"compress/gzip"
	"context"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"sync"
	"time"

	"spiderly/internal/models"
)

// Parser handles sitemap discovery and parsing
type Parser struct {
	client      *http.Client
	userAgent   string
	maxSitemaps int
	timeout     time.Duration
	
	// Callbacks
	onLog       func(level, message string)
	onURLFound  func(sitemapURL models.SitemapURL, source string)
	onSitemapFound func(url string)
}

// ParserConfig holds configuration for the sitemap parser
type ParserConfig struct {
	UserAgent   string
	MaxSitemaps int
	Timeout     time.Duration
}

// NewParser creates a new sitemap parser
func NewParser(config ParserConfig) *Parser {
	if config.UserAgent == "" {
		config.UserAgent = "Spiderly/1.0 (Sitemap Crawler)"
	}
	if config.MaxSitemaps == 0 {
		config.MaxSitemaps = 50
	}
	if config.Timeout == 0 {
		config.Timeout = 30 * time.Second
	}

	return &Parser{
		client: &http.Client{
			Timeout: config.Timeout,
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				if len(via) >= 10 {
					return fmt.Errorf("too many redirects")
				}
				return nil
			},
		},
		userAgent:   config.UserAgent,
		maxSitemaps: config.MaxSitemaps,
		timeout:     config.Timeout,
	}
}

// SetLogCallback sets the logging callback
func (p *Parser) SetLogCallback(fn func(level, message string)) {
	p.onLog = fn
}

// SetURLFoundCallback sets the URL discovery callback
func (p *Parser) SetURLFoundCallback(fn func(sitemapURL models.SitemapURL, source string)) {
	p.onURLFound = fn
}

// SetSitemapFoundCallback sets the sitemap discovery callback
func (p *Parser) SetSitemapFoundCallback(fn func(url string)) {
	p.onSitemapFound = fn
}

func (p *Parser) log(level, message string) {
	if p.onLog != nil {
		p.onLog(level, message)
	}
}

// DiscoverAndParse discovers sitemaps and parses all URLs
func (p *Parser) DiscoverAndParse(ctx context.Context, baseURL string) (*models.SitemapResult, error) {
	parsedURL, err := url.Parse(baseURL)
	if err != nil {
		return nil, fmt.Errorf("invalid URL: %w", err)
	}

	result := &models.SitemapResult{
		URLs:     make([]models.SitemapURL, 0),
		ParsedAt: time.Now(),
		Source:   baseURL,
	}

	// Track visited sitemaps to avoid duplicates
	visited := make(map[string]bool)
	var mu sync.Mutex

	// Try to discover sitemaps
	sitemapURLs := p.discoverSitemaps(ctx, parsedURL)
	
	if len(sitemapURLs) == 0 {
		p.log("warn", "No sitemaps found for "+baseURL)
		return result, nil
	}

	p.log("info", fmt.Sprintf("🗺️ Found %d potential sitemap location(s)", len(sitemapURLs)))

	// Process each sitemap
	for _, smURL := range sitemapURLs {
		if ctx.Err() != nil {
			break
		}

		mu.Lock()
		if visited[smURL] {
			mu.Unlock()
			continue
		}
		visited[smURL] = true
		mu.Unlock()

		urls, nestedSitemaps, err := p.parseSitemap(ctx, smURL)
		if err != nil {
			p.log("warn", fmt.Sprintf("Failed to parse sitemap %s: %v", smURL, err))
			continue
		}

		result.SitemapsFound++
		if p.onSitemapFound != nil {
			p.onSitemapFound(smURL)
		}

		// Add discovered URLs
		for _, u := range urls {
			result.URLs = append(result.URLs, u)
			if p.onURLFound != nil {
				p.onURLFound(u, smURL)
			}
		}

		p.log("success", fmt.Sprintf("✅ Parsed sitemap: %s (%d URLs)", truncateURL(smURL, 50), len(urls)))

		// Process nested sitemaps (sitemap index)
		for _, nestedURL := range nestedSitemaps {
			if result.SitemapsFound >= p.maxSitemaps {
				p.log("warn", fmt.Sprintf("Reached maximum sitemap limit (%d)", p.maxSitemaps))
				break
			}

			mu.Lock()
			if visited[nestedURL] {
				mu.Unlock()
				continue
			}
			visited[nestedURL] = true
			mu.Unlock()

			nestedURLs, _, err := p.parseSitemap(ctx, nestedURL)
			if err != nil {
				p.log("warn", fmt.Sprintf("Failed to parse nested sitemap %s: %v", nestedURL, err))
				continue
			}

			result.SitemapsFound++
			if p.onSitemapFound != nil {
				p.onSitemapFound(nestedURL)
			}

			for _, u := range nestedURLs {
				result.URLs = append(result.URLs, u)
				if p.onURLFound != nil {
					p.onURLFound(u, nestedURL)
				}
			}

			p.log("success", fmt.Sprintf("✅ Parsed nested sitemap: %s (%d URLs)", truncateURL(nestedURL, 50), len(nestedURLs)))
		}
	}

	result.TotalURLs = len(result.URLs)
	return result, nil
}

// discoverSitemaps finds sitemap URLs from common locations and robots.txt
func (p *Parser) discoverSitemaps(ctx context.Context, baseURL *url.URL) []string {
	var sitemaps []string
	seen := make(map[string]bool)

	addSitemap := func(u string) {
		if !seen[u] {
			seen[u] = true
			sitemaps = append(sitemaps, u)
		}
	}

	// Common sitemap locations
	commonPaths := []string{
		"/sitemap.xml",
		"/sitemap_index.xml",
		"/sitemap-index.xml",
		"/sitemapindex.xml",
		"/sitemap1.xml",
		"/sitemap-news.xml",
		"/news-sitemap.xml",
		"/post-sitemap.xml",
		"/page-sitemap.xml",
		"/wp-sitemap.xml",
	}

	baseURLStr := fmt.Sprintf("%s://%s", baseURL.Scheme, baseURL.Host)

	// Check robots.txt first
	robotsURL := baseURLStr + "/robots.txt"
	p.log("info", "🤖 Checking robots.txt for sitemap declarations...")
	
	robotsSitemaps := p.parseSitemapsFromRobots(ctx, robotsURL)
	for _, sm := range robotsSitemaps {
		addSitemap(sm)
		p.log("info", fmt.Sprintf("📍 Found in robots.txt: %s", truncateURL(sm, 60)))
	}

	// Check common paths
	p.log("info", "🔍 Checking common sitemap locations...")
	for _, path := range commonPaths {
		smURL := baseURLStr + path
		if p.checkSitemapExists(ctx, smURL) {
			addSitemap(smURL)
			p.log("info", fmt.Sprintf("📍 Found: %s", path))
		}
	}

	return sitemaps
}

// parseSitemapsFromRobots extracts sitemap URLs from robots.txt
func (p *Parser) parseSitemapsFromRobots(ctx context.Context, robotsURL string) []string {
	var sitemaps []string

	req, err := http.NewRequestWithContext(ctx, "GET", robotsURL, nil)
	if err != nil {
		return sitemaps
	}
	req.Header.Set("User-Agent", p.userAgent)

	resp, err := p.client.Do(req)
	if err != nil || resp.StatusCode != 200 {
		return sitemaps
	}
	defer resp.Body.Close()

	scanner := bufio.NewScanner(resp.Body)
	sitemapRegex := regexp.MustCompile(`(?i)^sitemap:\s*(.+)$`)

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if matches := sitemapRegex.FindStringSubmatch(line); len(matches) > 1 {
			smURL := strings.TrimSpace(matches[1])
			if smURL != "" {
				sitemaps = append(sitemaps, smURL)
			}
		}
	}

	return sitemaps
}

// checkSitemapExists verifies if a sitemap URL is accessible
func (p *Parser) checkSitemapExists(ctx context.Context, sitemapURL string) bool {
	req, err := http.NewRequestWithContext(ctx, "HEAD", sitemapURL, nil)
	if err != nil {
		return false
	}
	req.Header.Set("User-Agent", p.userAgent)

	resp, err := p.client.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()

	return resp.StatusCode == 200
}

// parseSitemap parses a single sitemap XML file
func (p *Parser) parseSitemap(ctx context.Context, sitemapURL string) ([]models.SitemapURL, []string, error) {
	var urls []models.SitemapURL
	var nestedSitemaps []string

	req, err := http.NewRequestWithContext(ctx, "GET", sitemapURL, nil)
	if err != nil {
		return nil, nil, err
	}
	req.Header.Set("User-Agent", p.userAgent)
	req.Header.Set("Accept", "application/xml, text/xml, */*")
	req.Header.Set("Accept-Encoding", "gzip, deflate")

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, nil, fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	var reader io.Reader = resp.Body

	// Handle gzip-compressed sitemaps
	if strings.Contains(resp.Header.Get("Content-Encoding"), "gzip") || 
	   strings.HasSuffix(sitemapURL, ".gz") {
		gzReader, err := gzip.NewReader(resp.Body)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to decompress gzip: %w", err)
		}
		defer gzReader.Close()
		reader = gzReader
	}

	// Read the body
	body, err := io.ReadAll(reader)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to read body: %w", err)
	}

	// Try parsing as sitemap index first
	var sitemapIndex models.SitemapIndex
	if err := xml.Unmarshal(body, &sitemapIndex); err == nil && len(sitemapIndex.Sitemaps) > 0 {
		p.log("info", fmt.Sprintf("📂 Found sitemap index with %d sitemaps", len(sitemapIndex.Sitemaps)))
		for _, entry := range sitemapIndex.Sitemaps {
			nestedSitemaps = append(nestedSitemaps, entry.Loc)
		}
		return urls, nestedSitemaps, nil
	}

	// Try parsing as regular sitemap
	var sitemap models.Sitemap
	if err := xml.Unmarshal(body, &sitemap); err == nil {
		for _, u := range sitemap.URLs {
			if u.Loc != "" {
				urls = append(urls, u)
			}
		}
		return urls, nestedSitemaps, nil
	}

	return nil, nil, fmt.Errorf("failed to parse XML structure")
}

// ParseSingleSitemap parses a specific sitemap URL directly
func (p *Parser) ParseSingleSitemap(ctx context.Context, sitemapURL string) (*models.SitemapResult, error) {
	result := &models.SitemapResult{
		URLs:     make([]models.SitemapURL, 0),
		ParsedAt: time.Now(),
		Source:   sitemapURL,
	}

	visited := make(map[string]bool)
	toProcess := []string{sitemapURL}

	for len(toProcess) > 0 && result.SitemapsFound < p.maxSitemaps {
		if ctx.Err() != nil {
			break
		}

		currentURL := toProcess[0]
		toProcess = toProcess[1:]

		if visited[currentURL] {
			continue
		}
		visited[currentURL] = true

		urls, nestedSitemaps, err := p.parseSitemap(ctx, currentURL)
		if err != nil {
			p.log("warn", fmt.Sprintf("Failed to parse %s: %v", currentURL, err))
			continue
		}

		result.SitemapsFound++
		if p.onSitemapFound != nil {
			p.onSitemapFound(currentURL)
		}

		for _, u := range urls {
			result.URLs = append(result.URLs, u)
			if p.onURLFound != nil {
				p.onURLFound(u, currentURL)
			}
		}

		toProcess = append(toProcess, nestedSitemaps...)
		p.log("success", fmt.Sprintf("✅ Parsed: %s (%d URLs, %d nested)", truncateURL(currentURL, 40), len(urls), len(nestedSitemaps)))
	}

	result.TotalURLs = len(result.URLs)
	return result, nil
}

// FilterURLs filters sitemap URLs based on criteria
func FilterURLs(urls []models.SitemapURL, filter func(models.SitemapURL) bool) []models.SitemapURL {
	var filtered []models.SitemapURL
	for _, u := range urls {
		if filter(u) {
			filtered = append(filtered, u)
		}
	}
	return filtered
}

// FilterByPriority returns URLs with priority >= minPriority
func FilterByPriority(urls []models.SitemapURL, minPriority float64) []models.SitemapURL {
	return FilterURLs(urls, func(u models.SitemapURL) bool {
		return u.Priority >= minPriority
	})
}

// FilterByPattern returns URLs matching the regex pattern
func FilterByPattern(urls []models.SitemapURL, pattern string) []models.SitemapURL {
	re, err := regexp.Compile(pattern)
	if err != nil {
		return urls
	}
	return FilterURLs(urls, func(u models.SitemapURL) bool {
		return re.MatchString(u.Loc)
	})
}

// Helper function to truncate long URLs for display
func truncateURL(u string, maxLen int) string {
	if len(u) <= maxLen {
		return u
	}
	return u[:maxLen-3] + "..."
}
