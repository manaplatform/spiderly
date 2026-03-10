package core

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"sync"
	"time"

	"spiderly/internal/crawler"
	"spiderly/internal/models"
	"spiderly/internal/sitemap"
	"spiderly/internal/web"
)

// ─────────────────────────────────────────────
//  Configuration
// ─────────────────────────────────────────────

// CoreConfig holds all configuration for the crawl core
type CoreConfig struct {
	// Target configuration
	TargetURL  string
	SitemapURL string // Direct sitemap URL (bypasses discovery)

	// Crawl limits
	MaxPages    int
	MaxDepth    int
	Concurrency int
	Delay       time.Duration
	Timeout     time.Duration

	// Sitemap filtering
	MinPriority float64
	URLPattern  string

	// Behavior flags
	ForceRecursive bool // Skip sitemap discovery, use traditional crawl
	Headless       bool
	Verbose        bool

	// Web server
	WebPort    int
	DisableWeb bool
}

// ─────────────────────────────────────────────
//  Core Struct
// ─────────────────────────────────────────────

// Core is the main orchestrator for Spiderly
type Core struct {
	config    CoreConfig
	webServer *web.Server
	crawler   *crawler.Crawler
	stats     *models.CrawlStats
	results   []models.ScrapedPage
	mu        sync.RWMutex
	ctx       context.Context
	cancel    context.CancelFunc
	startTime time.Time
}

// ─────────────────────────────────────────────
//  Exported Result Type
// ─────────────────────────────────────────────

// ScrapedPageResult is the exported result type used by cmd/main.go
type ScrapedPageResult struct {
	URL           string    `json:"url"`
	Title         string    `json:"title"`
	H1            string    `json:"h1,omitempty"`
	Description   string    `json:"description,omitempty"`
	Keywords      string    `json:"keywords,omitempty"`
	Author        string    `json:"author,omitempty"`
	PublishedDate string    `json:"published_date,omitempty"`
	OGImage       string    `json:"og_image,omitempty"`
	BodyText      string    `json:"body_text,omitempty"`
	StatusCode    int       `json:"status_code"`
	ContentType   string    `json:"content_type,omitempty"`
	ContentLength int64     `json:"content_length"`
	LoadTimeMs    int64     `json:"load_time_ms"`
	LinksCount    int       `json:"links_count"`
	ImagesCount   int       `json:"images_count"`
	Depth         int       `json:"depth"`
	ScrapedAt     time.Time `json:"scraped_at"`
}

// ToScrapedPageResults converts internal models.ScrapedPage slices to exported results
func ToScrapedPageResults(pages []models.ScrapedPage) []ScrapedPageResult {
	results := make([]ScrapedPageResult, len(pages))
	for i, p := range pages {
		results[i] = ScrapedPageResult{
			URL:           p.URL,
			Title:         p.Title,
			H1:            p.H1,
			Description:   p.Description,
			Keywords:      p.Keywords,
			Author:        p.Author,
			PublishedDate: p.PublishedDate,
			OGImage:       p.OGImage,
			BodyText:      p.BodyText,
			StatusCode:    p.StatusCode,
			ContentType:   p.ContentType,
			ContentLength: p.ContentLength,
			LoadTimeMs:    p.LoadTimeMs,
			LinksCount:    p.LinksCount,
			ImagesCount:   p.ImagesCount,
			Depth:         p.Depth,
			ScrapedAt:     p.ScrapedAt,
		}
	}
	return results
}

// ─────────────────────────────────────────────
//  Constructors
// ─────────────────────────────────────────────

// New creates a Core with sensible defaults using just URL and max pages.
// This is the shorthand constructor used by cmd/main.go:
//
//	e := core.New(target, *maxPages)
func New(targetURL string, maxPages int) *Core {
	return NewCore(CoreConfig{
		TargetURL:   targetURL,
		MaxPages:    maxPages,
		MaxDepth:    10,
		Concurrency: 5,
		Delay:       200 * time.Millisecond,
		Timeout:     30 * time.Second,
		WebPort:     8080,
		DisableWeb:  false,
		Verbose:     false,
	})
}

// NewCore creates a new crawl core with full configuration
func NewCore(cfg CoreConfig) *Core {
	ctx, cancel := context.WithCancel(context.Background())

	return &Core{
		config:    cfg,
		stats:     &models.CrawlStats{},
		results:   make([]models.ScrapedPage, 0),
		ctx:       ctx,
		cancel:    cancel,
		startTime: time.Now(),
	}
}

// ApplyConfig overrides the current configuration.
// Only non-zero / non-empty values in cfg replace existing ones,
// so you can selectively override fields after calling New().
func (c *Core) ApplyConfig(cfg CoreConfig) {
	if cfg.TargetURL != "" {
		c.config.TargetURL = cfg.TargetURL
	}
	if cfg.SitemapURL != "" {
		c.config.SitemapURL = cfg.SitemapURL
	}
	if cfg.MaxPages > 0 {
		c.config.MaxPages = cfg.MaxPages
	}
	if cfg.MaxDepth > 0 {
		c.config.MaxDepth = cfg.MaxDepth
	}
	if cfg.Concurrency > 0 {
		c.config.Concurrency = cfg.Concurrency
	}
	if cfg.Delay > 0 {
		c.config.Delay = cfg.Delay
	}
	if cfg.Timeout > 0 {
		c.config.Timeout = cfg.Timeout
	}
	if cfg.MinPriority > 0 {
		c.config.MinPriority = cfg.MinPriority
	}
	if cfg.URLPattern != "" {
		c.config.URLPattern = cfg.URLPattern
	}
	if cfg.WebPort > 0 {
		c.config.WebPort = cfg.WebPort
	}

	// Boolean flags — always apply (they're intentional toggles)
	c.config.ForceRecursive = cfg.ForceRecursive
	c.config.Headless = cfg.Headless
	c.config.Verbose = cfg.Verbose
	c.config.DisableWeb = cfg.DisableWeb
}

// ─────────────────────────────────────────────
//  Run — Main Pipeline
// ─────────────────────────────────────────────

// Run executes the full crawl pipeline
func (c *Core) Run() ([]models.ScrapedPage, error) {
	c.startTime = time.Now()

	// Phase 1: Start web server in background (immediately)
	if !c.config.DisableWeb {
		c.startWebServer()
		// Give server a moment to bind
		time.Sleep(100 * time.Millisecond)
	}

	// Validate target URL
	targetURL, err := c.validateTargetURL()
	if err != nil {
		return nil, fmt.Errorf("invalid target URL: %w", err)
	}

	c.broadcast(models.WebSocketMessage{
		Type: "status",
		Data: map[string]interface{}{
			"phase":   "initializing",
			"message": fmt.Sprintf("Starting crawl for %s", targetURL),
		},
	})

	// Phase 2: Determine crawl strategy
	strategy, sitemapURLs, err := c.determineCrawlStrategy(targetURL)
	if err != nil {
		c.logVerbose("Strategy determination warning: %v", err)
	}

	c.broadcast(models.WebSocketMessage{
		Type: "strategy",
		Data: map[string]interface{}{
			"mode":         strategy,
			"sitemapCount": len(sitemapURLs),
		},
	})

	// Phase 3: Execute crawl based on strategy
	switch strategy {
	case "sitemap":
		return c.executeSitemapCrawl(targetURL, sitemapURLs)
	case "recursive":
		return c.executeRecursiveCrawl(targetURL)
	default:
		return c.executeRecursiveCrawl(targetURL)
	}
}

// ─────────────────────────────────────────────
//  Web Server
// ─────────────────────────────────────────────

// startWebServer initializes and runs the web dashboard in background
func (c *Core) startWebServer() {
	c.webServer = web.NewServer(c.config.WebPort)

	go func() {
		addr := fmt.Sprintf(":%d", c.config.WebPort)
		c.logVerbose("Starting web dashboard on http://localhost%s", addr)

		if err := c.webServer.Start(); err != nil && err != http.ErrServerClosed {
			log.Printf("Web server error: %v", err)
		}
	}()

	// Log dashboard URL
	fmt.Printf("\n🌐 Dashboard: http://localhost:%d\n\n", c.config.WebPort)
}

// ─────────────────────────────────────────────
//  URL Validation
// ─────────────────────────────────────────────

// validateTargetURL validates and normalizes the target URL
func (c *Core) validateTargetURL() (string, error) {
	// If direct sitemap URL provided, extract base domain
	if c.config.SitemapURL != "" {
		parsed, err := url.Parse(c.config.SitemapURL)
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("%s://%s", parsed.Scheme, parsed.Host), nil
	}

	if c.config.TargetURL == "" {
		return "", fmt.Errorf("no target URL specified")
	}

	// Ensure URL has scheme
	targetURL := c.config.TargetURL
	if !strings.HasPrefix(targetURL, "http://") && !strings.HasPrefix(targetURL, "https://") {
		targetURL = "https://" + targetURL
	}

	parsed, err := url.Parse(targetURL)
	if err != nil {
		return "", err
	}

	if parsed.Host == "" {
		return "", fmt.Errorf("URL must have a valid host")
	}

	return targetURL, nil
}

// ─────────────────────────────────────────────
//  Crawl Strategy
// ─────────────────────────────────────────────

// determineCrawlStrategy decides between sitemap and recursive crawling
func (c *Core) determineCrawlStrategy(targetURL string) (string, []models.SitemapEntry, error) {
	// If force recursive mode, skip sitemap discovery
	if c.config.ForceRecursive {
		c.logVerbose("Forced recursive mode - skipping sitemap discovery")
		return "recursive", nil, nil
	}

	// If direct sitemap URL provided, use it
	if c.config.SitemapURL != "" {
		c.logVerbose("Direct sitemap URL provided: %s", c.config.SitemapURL)
		entries, err := c.fetchSitemapEntries(c.config.SitemapURL)
		if err != nil {
			return "recursive", nil, fmt.Errorf("failed to fetch provided sitemap: %w", err)
		}
		if len(entries) > 0 {
			return "sitemap", entries, nil
		}
		return "recursive", nil, fmt.Errorf("provided sitemap was empty")
	}

	// Auto-discovery: Try to find sitemaps (DEFAULT BEHAVIOR)
	c.broadcast(models.WebSocketMessage{
		Type: "status",
		Data: map[string]interface{}{
			"phase":   "discovery",
			"message": "Searching for sitemaps...",
		},
	})

	parser := sitemap.NewParser(c.config.Timeout, c.config.Verbose)

	// Discover sitemaps from robots.txt and common locations
	sitemapURLs, err := parser.DiscoverSitemaps(targetURL)
	if err != nil {
		c.logVerbose("Sitemap discovery error: %v", err)
	}

	if len(sitemapURLs) == 0 {
		c.logVerbose("No sitemaps found - falling back to recursive crawl")
		c.broadcast(models.WebSocketMessage{
			Type: "status",
			Data: map[string]interface{}{
				"phase":   "discovery",
				"message": "No sitemaps found - using recursive crawl",
			},
		})
		return "recursive", nil, nil
	}

	c.logVerbose("Found %d sitemap(s)", len(sitemapURLs))

	// Parse all discovered sitemaps
	var allEntries []models.SitemapEntry
	for _, sitemapURL := range sitemapURLs {
		entries, err := c.fetchSitemapEntries(sitemapURL)
		if err != nil {
			c.logVerbose("Failed to parse sitemap %s: %v", sitemapURL, err)
			continue
		}
		allEntries = append(allEntries, entries...)
	}

	if len(allEntries) == 0 {
		c.logVerbose("All sitemaps were empty - falling back to recursive crawl")
		return "recursive", nil, nil
	}

	// Apply filters
	filteredEntries := c.filterSitemapEntries(allEntries)

	c.broadcast(models.WebSocketMessage{
		Type: "sitemap_stats",
		Data: map[string]interface{}{
			"totalUrls":    len(allEntries),
			"filteredUrls": len(filteredEntries),
			"sitemaps":     len(sitemapURLs),
		},
	})

	fmt.Printf("📍 Found %d URLs in sitemap(s), %d after filtering\n", len(allEntries), len(filteredEntries))

	return "sitemap", filteredEntries, nil
}

// fetchSitemapEntries fetches and parses a single sitemap
func (c *Core) fetchSitemapEntries(sitemapURL string) ([]models.SitemapEntry, error) {
	parser := sitemap.NewParser(c.config.Timeout, c.config.Verbose)
	return parser.ParseSitemap(sitemapURL)
}

// filterSitemapEntries applies priority and regex filters
func (c *Core) filterSitemapEntries(entries []models.SitemapEntry) []models.SitemapEntry {
	var filtered []models.SitemapEntry

	var urlRegex *regexp.Regexp
	if c.config.URLPattern != "" {
		var err error
		urlRegex, err = regexp.Compile(c.config.URLPattern)
		if err != nil {
			c.logVerbose("Invalid URL pattern regex: %v", err)
			urlRegex = nil
		}
	}

	for _, entry := range entries {
		// Priority filter
		if c.config.MinPriority > 0 && entry.Priority < c.config.MinPriority {
			continue
		}

		// URL pattern filter
		if urlRegex != nil && !urlRegex.MatchString(entry.URL) {
			continue
		}

		filtered = append(filtered, entry)
	}

	return filtered
}

// ─────────────────────────────────────────────
//  Crawl Execution
// ─────────────────────────────────────────────

// executeSitemapCrawl crawls URLs discovered from sitemaps
func (c *Core) executeSitemapCrawl(baseURL string, entries []models.SitemapEntry) ([]models.ScrapedPage, error) {
	c.logVerbose("Starting sitemap-based crawl with %d URLs", len(entries))

	c.broadcast(models.WebSocketMessage{
		Type: "status",
		Data: map[string]interface{}{
			"phase":   "crawling",
			"message": fmt.Sprintf("Crawling %d URLs from sitemap", len(entries)),
			"mode":    "sitemap",
		},
	})

	// Convert entries to URL list
	urls := make([]string, 0, len(entries))
	for _, entry := range entries {
		urls = append(urls, entry.URL)
	}

	// Limit to MaxPages
	if c.config.MaxPages > 0 && len(urls) > c.config.MaxPages {
		urls = urls[:c.config.MaxPages]
	}

	// Create and configure crawler
	c.crawler = crawler.NewCrawler(crawler.Config{
		MaxPages:    c.config.MaxPages,
		MaxDepth:    1, // Sitemap URLs are flat
		Concurrency: c.config.Concurrency,
		Delay:       c.config.Delay,
		Timeout:     c.config.Timeout,
		Headless:    c.config.Headless,
		SitemapMode: true,
	})

	// Set up callbacks
	c.setupCrawlerCallbacks()

	// Queue all sitemap URLs
	for _, u := range urls {
		c.crawler.QueueURL(u, 0)
	}

	// Execute crawl
	results, err := c.crawler.Crawl(baseURL)
	if err != nil {
		return nil, err
	}

	c.mu.Lock()
	c.results = results
	c.mu.Unlock()

	c.finalizeCrawl()

	return results, nil
}

// executeRecursiveCrawl performs traditional link-following crawl
func (c *Core) executeRecursiveCrawl(targetURL string) ([]models.ScrapedPage, error) {
	c.logVerbose("Starting recursive crawl from %s", targetURL)

	c.broadcast(models.WebSocketMessage{
		Type: "status",
		Data: map[string]interface{}{
			"phase":   "crawling",
			"message": fmt.Sprintf("Recursive crawl starting from %s", targetURL),
			"mode":    "recursive",
		},
	})

	// Create and configure crawler
	c.crawler = crawler.NewCrawler(crawler.Config{
		MaxPages:    c.config.MaxPages,
		MaxDepth:    c.config.MaxDepth,
		Concurrency: c.config.Concurrency,
		Delay:       c.config.Delay,
		Timeout:     c.config.Timeout,
		Headless:    c.config.Headless,
		SitemapMode: false,
	})

	// Set up callbacks
	c.setupCrawlerCallbacks()

	// Execute crawl
	results, err := c.crawler.Crawl(targetURL)
	if err != nil {
		return nil, err
	}

	c.mu.Lock()
	c.results = results
	c.mu.Unlock()

	c.finalizeCrawl()

	return results, nil
}

// ─────────────────────────────────────────────
//  Callbacks
// ─────────────────────────────────────────────

// setupCrawlerCallbacks configures real-time reporting callbacks
func (c *Core) setupCrawlerCallbacks() {
	if c.crawler == nil {
		return
	}

	// Page scraped callback
	c.crawler.OnPageScraped(func(page models.ScrapedPage) {
		c.mu.Lock()
		c.stats.PagesScraped++
		c.mu.Unlock()

		c.broadcast(models.WebSocketMessage{
			Type: "page",
			Data: map[string]interface{}{
				"url":         page.URL,
				"title":       page.Title,
				"statusCode":  page.StatusCode,
				"contentType": page.ContentType,
				"scraped":     c.stats.PagesScraped,
			},
		})
	})

	// Error callback
	c.crawler.OnError(func(url string, err error) {
		c.mu.Lock()
		c.stats.Errors++
		c.mu.Unlock()

		c.broadcast(models.WebSocketMessage{
			Type: "error",
			Data: map[string]interface{}{
				"url":   url,
				"error": err.Error(),
			},
		})
	})

	// Link discovered callback
	c.crawler.OnLinkDiscovered(func(link models.DiscoveredLink) {
		c.broadcast(models.WebSocketMessage{
			Type: "link",
			Data: map[string]interface{}{
				"url":    link.URL,
				"source": link.SourceURL,
				"depth":  link.Depth,
			},
		})
	})
}

// ─────────────────────────────────────────────
//  Finalization & Utilities
// ─────────────────────────────────────────────

// finalizeCrawl sends completion message and stats
func (c *Core) finalizeCrawl() {
	duration := time.Since(c.startTime)

	c.mu.RLock()
	stats := *c.stats
	resultCount := len(c.results)
	c.mu.RUnlock()

	c.broadcast(models.WebSocketMessage{
		Type: "complete",
		Data: map[string]interface{}{
			"pagesScraped": stats.PagesScraped,
			"errors":       stats.Errors,
			"duration":     duration.String(),
			"durationMs":   duration.Milliseconds(),
			"totalResults": resultCount,
		},
	})

	fmt.Printf("\n✅ Crawl complete: %d pages in %s\n", resultCount, duration.Round(time.Millisecond))
}

// broadcast sends a message to all connected WebSocket clients
func (c *Core) broadcast(msg models.WebSocketMessage) {
	if c.webServer != nil {
		c.webServer.Broadcast(msg)
	}
}

// logVerbose logs a message if verbose mode is enabled
func (c *Core) logVerbose(format string, args ...interface{}) {
	if c.config.Verbose {
		log.Printf("[CORE] "+format, args...)
	}
}

// ─────────────────────────────────────────────
//  Public Accessors & Lifecycle
// ─────────────────────────────────────────────

// Stop gracefully shuts down the core
func (c *Core) Stop() {
	c.cancel()
	if c.crawler != nil {
		c.crawler.Stop()
	}
	if c.webServer != nil {
		c.webServer.Stop()
	}
}

// GetResults returns the crawl results
func (c *Core) GetResults() []models.ScrapedPage {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.results
}

// GetStats returns current crawl statistics
func (c *Core) GetStats() models.CrawlStats {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return *c.stats
}
