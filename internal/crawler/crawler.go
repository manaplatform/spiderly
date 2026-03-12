// internal/crawler/crawler.go
package crawler

import (
	"context"
	"fmt"
	"net/url"
	"regexp"
	"strings"
	"sync"
	"time"
	"spiderly/internal/extractor"
	"spiderly/internal/exclude"
	"spiderly/internal/models"
	"log"
	"github.com/gocolly/colly/v2"
)

// ─────────────────────────────────────────────────────────────────────────────
//  Configuration
// ─────────────────────────────────────────────────────────────────────────────

// Config holds crawler configuration
type Config struct {
	MaxPages    int
	MaxDepth    int
	Concurrency int
	Delay       time.Duration
	Timeout     time.Duration
	UserAgent   string
	Headless    bool
	Verbose     bool
	SitemapMode bool

	// Product extraction settings
	ProductMode    bool
	ProductPattern *regexp.Regexp
	ExtractSpecs   bool
	ExtractImages  bool
	ExcludePatterns []string
	CompiledExcludePatterns []*regexp.Regexp
}

// DefaultConfig returns sensible defaults
func DefaultConfig() Config {
	return Config{
		MaxPages:    100,
		MaxDepth:    3,
		Concurrency: 5,
		Delay:       200 * time.Millisecond,
		Timeout:     30 * time.Second,
		UserAgent:   "Spiderly/1.0",
		Headless:    false,
		Verbose:     false,
		SitemapMode: false,
	}
}

// ─────────────────────────────────────────────────────────────────────────────
//  Crawler Struct
// ─────────────────────────────────────────────────────────────────────────────

// Crawler handles web scraping operations
type Crawler struct {
	config    Config
	collector *colly.Collector
	results   []models.ScrapedPage
	visited   map[string]bool
	queue     []models.DiscoveredLink
	mu        sync.Mutex
	scraped   int // Counter for scraped pages
	ctx       context.Context
	cancel    context.CancelFunc

	// Callbacks
	onPageScraped func(page models.ScrapedPage)
	onError       func(url string, err error)
	onLinkFound   func(link models.DiscoveredLink)
	onLinkDiscovered func(from, to string)
}

// ─────────────────────────────────────────────────────────────────────────────
//  Constructor
// ─────────────────────────────────────────────────────────────────────────────

// NewCrawler creates a new Crawler instance
func NewCrawler(cfg Config) *Crawler {
	ctx, cancel := context.WithCancel(context.Background())

	// Precompile exclude patterns if provided
	if len(cfg.ExcludePatterns) > 0 && len(cfg.CompiledExcludePatterns) == 0 {
		if res, err := exclude.CompilePatterns(cfg.ExcludePatterns); err == nil {
			cfg.CompiledExcludePatterns = res
		}
	}
	return &Crawler{
		config:  cfg,
		results: make([]models.ScrapedPage, 0),
		visited: make(map[string]bool),
		queue:   make([]models.DiscoveredLink, 0),
		ctx:     ctx,
		cancel:  cancel,
	}
}

// ─────────────────────────────────────────────────────────────────────────────
//  Callback Setters
// ─────────────────────────────────────────────────────────────────────────────

// OnPageScraped sets callback for each successfully scraped page
func (c *Crawler) OnPageScraped(fn func(page models.ScrapedPage)) {
	c.onPageScraped = fn
}

// OnError sets callback for crawl errors
func (c *Crawler) OnError(fn func(url string, err error)) {
	c.onError = fn
}

// OnLinkFound sets callback for discovered links
func (c *Crawler) OnLinkFound(fn func(link models.DiscoveredLink)) {
	c.onLinkFound = fn
}

// ─────────────────────────────────────────────────────────────────────────────
//  Public Methods
// ─────────────────────────────────────────────────────────────────────────────

// QueueURL adds a URL to the crawl queue
func (c *Crawler) QueueURL(rawURL string, depth int) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.visited[rawURL] {
		return
	}

	c.queue = append(c.queue, models.DiscoveredLink{
		URL:   rawURL,
		Depth: depth,
	})
}

// Crawl starts the crawling process
func (c *Crawler) Crawl(startURL string) ([]models.ScrapedPage, error) {
	// Setup collector with domain restriction based on startURL
	if err := c.setupCollector(startURL); err != nil {
		return nil, fmt.Errorf("failed to setup collector: %w", err)
	}

	// If sitemap mode with queued URLs, visit them directly
	if c.config.SitemapMode && len(c.queue) > 0 {
		return c.crawlQueued()
	}

	// Otherwise do recursive crawl starting from startURL
	c.QueueURL(startURL, 0)
	return c.crawlQueued()
}

// Stop cancels the crawl operation
func (c *Crawler) Stop() {
	c.cancel()
}

// GetResults returns all scraped pages
func (c *Crawler) GetResults() []models.ScrapedPage {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.results
}

// ─────────────────────────────────────────────────────────────────────────────
//  Internal: Setup
// ─────────────────────────────────────────────────────────────────────────────

func (c *Crawler) setupCollector(startURL string) error {
	// Parse start URL to get domain
	parsedURL, err := url.Parse(startURL)
	if err != nil {
		return fmt.Errorf("invalid start URL: %w", err)
	}

	// Create collector with options
	opts := []colly.CollectorOption{
		colly.Async(true),
		colly.MaxDepth(c.config.MaxDepth),
		colly.UserAgent(c.config.UserAgent),
		colly.AllowedDomains(parsedURL.Host), // Restrict to same domain
	}

	c.collector = colly.NewCollector(opts...)

	// Set limits
	c.collector.Limit(&colly.LimitRule{
		DomainGlob:  "*",
		Parallelism: c.config.Concurrency,
		Delay:       c.config.Delay,
	})

	c.collector.SetRequestTimeout(c.config.Timeout)

	// Register handlers
	c.collector.OnHTML("html", c.handlePage)
	c.collector.OnHTML("a[href]", c.handleLink)
	c.collector.OnError(c.handleError)

	return nil
}

// ─────────────────────────────────────────────────────────────────────────────
//  Internal: Crawl Loop
// ─────────────────────────────────────────────────────────────────────────────

func (c *Crawler) crawlQueued() ([]models.ScrapedPage, error) {
	for {
		// Check context cancellation
		select {
		case <-c.ctx.Done():
			c.collector.Wait()
			return c.results, c.ctx.Err()
		default:
		}

		// Get next URL from queue
		c.mu.Lock()
		if len(c.queue) == 0 {
			c.mu.Unlock()
			break
		}

		// Check if we've hit max pages
		if c.config.MaxPages > 0 && c.scraped >= c.config.MaxPages {
			c.mu.Unlock()
			break
		}

		link := c.queue[0]
		c.queue = c.queue[1:]

		// Skip if already visited
		if c.visited[link.URL] {
			c.mu.Unlock()
			continue
		}

		c.visited[link.URL] = true
		c.mu.Unlock()

		// Visit URL
		if err := c.collector.Visit(link.URL); err != nil {
			if c.onError != nil {
				c.onError(link.URL, err)
			}
		}
	}

	c.collector.Wait()
	return c.results, nil
}

// ─────────────────────────────────────────────────────────────────────────────
//  Internal: Handlers
// ─────────────────────────────────────────────────────────────────────────────

func (c *Crawler) handlePage(e *colly.HTMLElement) {
	// Check context
	select {
	case <-c.ctx.Done():
		return
	default:
	}

	pageURL := e.Request.URL.String()

	// Thread-safe increment and max check
	c.mu.Lock()
	if c.config.MaxPages > 0 && c.scraped >= c.config.MaxPages {
		c.mu.Unlock()
		return
	}
	c.scraped++
	c.mu.Unlock()

	// Build page result
	page := models.ScrapedPage{
		URL:        pageURL,
		Title:      strings.TrimSpace(e.ChildText("title")),
		StatusCode: e.Response.StatusCode,
	}

	// Extract content length if available
	if e.Response != nil {
		page.ContentLength = int64(len(e.Response.Body))
	}

	// Extract main content
	page.Content = extractor.ExtractMainContent(e.DOM)

	// Conditionally extract product data
	if c.shouldExtractProduct(pageURL) {
		// DEBUG: Log that extraction is starting for this specific URL
		log.Printf("[DEBUG] Starting product extraction for URL: %s", pageURL)

		opts := extractor.ProductOptions{
			ExtractSpecs:  c.config.ExtractSpecs,
			ExtractImages: c.config.ExtractImages,
		}
		product := extractor.ExtractProduct(e.DOM, pageURL, opts)
		
		if product != nil && product.Name != "" {
			// DEBUG: Log a successful extraction and print the product name
			log.Printf("[DEBUG] Successfully extracted product: '%s' from URL: %s", product.Name, pageURL)
			page.Product = product
		} else {
			// DEBUG: Log if the extraction returned nil or an empty product name
			log.Printf("[DEBUG] Extraction failed or product name was empty for URL: %s", pageURL)
		}
	} else {
		// DEBUG (Optional): Log when a page is skipped based on your rules
		log.Printf("[DEBUG] Skipping product extraction (condition not met) for URL: %s", pageURL)
	}

	// Store result
	c.mu.Lock()
	c.results = append(c.results, page)
	c.mu.Unlock()

	// Trigger callback
	if c.onPageScraped != nil {
		c.onPageScraped(page)
	}
}

func (c *Crawler) handleLink(e *colly.HTMLElement) {
	// Check context
	select {
	case <-c.ctx.Done():
		return
	default:
	}

	// Skip in sitemap mode - we only crawl queued URLs
	if c.config.SitemapMode {
		return
	}

	href := e.Attr("href")
	if href == "" {
		return
	}

	// Resolve relative URLs
	absoluteURL := e.Request.AbsoluteURL(href)
	if absoluteURL == "" {
		return
	}

	// Skip non-http(s) URLs
	if !strings.HasPrefix(absoluteURL, "http://") && !strings.HasPrefix(absoluteURL, "https://") {
		return
	}

	// Clean URL (remove fragments)
	if idx := strings.Index(absoluteURL, "#"); idx != -1 {
		absoluteURL = absoluteURL[:idx]
	}

	// Calculate depth
	currentDepth := e.Request.Depth

	c.mu.Lock()
	alreadyVisited := c.visited[absoluteURL]
	if !alreadyVisited && (c.config.MaxDepth == 0 || currentDepth < c.config.MaxDepth) {
		c.queue = append(c.queue, models.DiscoveredLink{
			URL:   absoluteURL,
			Depth: currentDepth + 1,
		})
	}
	c.mu.Unlock()

	// Trigger callback
	if c.onLinkFound != nil && !alreadyVisited {
		c.onLinkFound(models.DiscoveredLink{
			URL:   absoluteURL,
			Depth: currentDepth + 1,
		})
	}

	// Visit the link directly for recursive crawling
	if !alreadyVisited {
		e.Request.Visit(absoluteURL)
	}
}

func (c *Crawler) handleError(r *colly.Response, err error) {
	if c.onError != nil {
		c.onError(r.Request.URL.String(), err)
	}
}

// ─────────────────────────────────────────────────────────────────────────────
//  Internal: Product Detection
// ─────────────────────────────────────────────────────────────────────────────

func (c *Crawler) shouldExtractProduct(pageURL string) bool {
	log.Printf("[shouldExtractProduct] Evaluating URL: %s", pageURL)

	if !c.config.ProductMode {
		log.Printf("[shouldExtractProduct] Result: false | Reason: ProductMode is disabled | URL: %s", pageURL)
		return false
	} else if c.config.ProductPattern != nil {
		isMatch := c.config.ProductPattern.MatchString(pageURL)
		if isMatch {
			log.Printf("[shouldExtractProduct] Result: true | Reason: URL matches ProductPattern | URL: %s", pageURL)
		} else {
			log.Printf("[shouldExtractProduct] Result: false | Reason: URL does NOT match ProductPattern | URL: %s", pageURL)
		}
		return isMatch
	} else {
		log.Printf("[shouldExtractProduct] Result: true | Reason: ProductMode is enabled but no ProductPattern is specified | URL: %s", pageURL)
		return true
	}
}

// OnLinkDiscovered sets callback for when a new link is found
func (c *Crawler) OnLinkDiscovered(fn func(from, to string)) {
	c.onLinkDiscovered = fn
}
