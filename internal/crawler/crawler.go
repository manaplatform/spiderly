// internal/crawler/crawler.go
package crawler

import (
	"context"
	"fmt"
	"log"
	"strings"
	"sync"
	"sync/atomic"

	"spiderly/internal/browser"
	"spiderly/internal/models"

	"github.com/PuerkitoBio/goquery"
	"github.com/gocolly/colly/v2"
)

// Crawler handles web scraping operations.
type Crawler struct {
	config    Config
	collector *colly.Collector
	browser   *browser.Engine

	results []models.ScrapedPage
	mu      sync.Mutex

	queue   *URLQueue
	scraped atomic.Int64

	ctx    context.Context
	cancel context.CancelFunc

	// Callbacks
	onPageScraped    func(page models.ScrapedPage)
	onError          func(url string, err error)
	onLinkFound      func(link models.DiscoveredLink)
	onLinkDiscovered func(from, to string)
}

// NewCrawler creates a new Crawler instance.
// Call Config.Validate() before passing it in, or this constructor
// will validate and return an error on bad config.
func NewCrawler(cfg Config) (*Crawler, error) {
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("invalid config: %w", err)
	}

	ctx, cancel := context.WithCancel(context.Background())

	var bEngine *browser.Engine
	if cfg.Headless {
		browserTimeout := cfg.BrowserTimeout
		if browserTimeout <= 0 {
			browserTimeout = cfg.Timeout
		}
		var err error
		bEngine, err = browser.NewEngine(browser.Config{
			Headless:         !cfg.BrowserVisible,
			Timeout:          browserTimeout,
			Delay:            cfg.Delay,
			Concurrency:      cfg.Concurrency,
			UserAgent:        cfg.UserAgent,
			Proxies:          cfg.Proxies,
			ClickCrawl:       cfg.ClickCrawl,
			MaxClicksPerPage: cfg.MaxClicksPerPage,
		})
		if err != nil {
			log.Printf("[crawler] warning: ChromeDP unavailable (%v); falling back to Colly HTTP engine", err)
		}
	}

	return &Crawler{
		config:  cfg,
		browser: bEngine,
		results: make([]models.ScrapedPage, 0),
		queue:   NewURLQueue(),
		ctx:     ctx,
		cancel:  cancel,
	}, nil
}

// Crawl starts the crawling process from startURL.
func (c *Crawler) Crawl(startURL string) ([]models.ScrapedPage, error) {
	defer func() {
		if c.browser != nil {
			_ = c.browser.Close()
		}
	}()

	if err := c.setupCollector(startURL); err != nil {
		return nil, fmt.Errorf("collector setup failed: %w", err)
	}

	// In sitemap mode the queue is pre-populated externally via QueueURL.
	// In normal mode we seed it with the start URL.
	if !c.config.SitemapMode || c.queue.Len() == 0 {
		c.queue.Push(models.DiscoveredLink{URL: startURL, Depth: 0})
	}

	return c.drainQueue()
}

// QueueURL adds a URL to the crawl queue (used for sitemap mode).
func (c *Crawler) QueueURL(rawURL string, depth int) {
	c.queue.Push(models.DiscoveredLink{URL: rawURL, Depth: depth})
}

// Stop cancels the crawl and closes the browser engine.
func (c *Crawler) Stop() {
	c.cancel()
	if c.browser != nil {
		_ = c.browser.Close()
	}
}

// GetResults returns all scraped pages collected so far.
func (c *Crawler) GetResults() []models.ScrapedPage {
	c.mu.Lock()
	defer c.mu.Unlock()
	dst := make([]models.ScrapedPage, len(c.results))
	copy(dst, c.results)
	return dst
}

// drainQueue processes URLs from the queue until empty or limits are hit.
func (c *Crawler) drainQueue() ([]models.ScrapedPage, error) {
	visited := make(map[string]bool)

	for {
		select {
		case <-c.ctx.Done():
			c.collector.Wait()
			return c.GetResults(), c.ctx.Err()
		default:
		}

		// Max pages reached
		if c.config.MaxPages > 0 && int(c.scraped.Load()) >= c.config.MaxPages {
			break
		}

		link, ok := c.queue.Pop()
		if !ok {
			// Queue empty — wait for in-flight requests, then check again.
			c.collector.Wait()

			// After waiting, the handlers may have pushed new links.
			if c.queue.Len() == 0 {
				break
			}
			continue
		}

		if visited[link.URL] {
			continue
		}
		visited[link.URL] = true

		// Strategy dispatch: "always" uses ChromeDP directly; "fallback" uses Colly with JS fallback
		if c.browser != nil && c.config.BrowserStrategy == "always" {
			if err := c.crawlURLWithBrowser(link.URL, link.Depth); err != nil {
				c.logVerbose("browser render failed for %s (%v); falling back to Colly", link.URL, err)
				if err := c.collector.Visit(link.URL); err != nil {
					c.logVerbose("visit failed: %s — %v", link.URL, err)
					if c.onError != nil {
						c.onError(link.URL, err)
					}
				}
			}
		} else {
			if err := c.collector.Visit(link.URL); err != nil {
				c.logVerbose("visit failed: %s — %v", link.URL, err)
				if c.onError != nil {
					c.onError(link.URL, err)
				}
			}
		}
	}

	c.collector.Wait()
	return c.GetResults(), nil
}

// crawlURLWithBrowser renders a URL via ChromeDP and handles extraction & link discovery.
func (c *Crawler) crawlURLWithBrowser(rawURL string, depth int) error {
	select {
	case <-c.ctx.Done():
		return c.ctx.Err()
	default:
	}

	// Atomically check and increment scraped count
	if !c.incrementScrapedCount(rawURL) {
		return nil
	}

	res, err := c.browser.Render(c.ctx, rawURL)
	if err != nil {
		c.scraped.Add(-1) // roll back so Colly fallback can count
		return err
	}

	targetURL := res.FinalURL
	if targetURL == "" {
		targetURL = rawURL
	}

	doc, err := goquery.NewDocumentFromReader(strings.NewReader(res.HTML))
	if err != nil {
		c.scraped.Add(-1)
		return fmt.Errorf("failed to parse rendered HTML for %s: %w", rawURL, err)
	}

	page := c.extractPageFromDOM(targetURL, doc.Selection, res.StatusCode, res.ContentLength, depth)
	if res.Title != "" {
		page.Title = res.Title
	}
	if page.URL == "" {
		page.URL = rawURL
	}

	c.addResult(page)
	if c.onPageScraped != nil {
		c.onPageScraped(page)
	}

	if !c.config.SitemapMode {
		c.extractLinksFromDOM(doc.Selection, targetURL, depth)
	}

	if len(res.DiscoveredLinks) > 0 {
		c.enqueueDiscoveredLinks(res.DiscoveredLinks, depth, targetURL)
	}

	return nil
}

// addResult appends a page to the results slice (thread-safe).
func (c *Crawler) addResult(page models.ScrapedPage) {
	c.mu.Lock()
	c.results = append(c.results, page)
	c.mu.Unlock()
}

// logVerbose prints only when Verbose is enabled.
func (c *Crawler) logVerbose(format string, args ...any) {
	if c.config.Verbose {
		log.Printf("[crawler] "+format, args...)
	}
}
