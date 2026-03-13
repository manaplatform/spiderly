// internal/crawler/crawler.go
package crawler

import (
	"context"
	"fmt"
	"log"
	"sync"
	"sync/atomic"

	"spiderly/internal/models"

	"github.com/gocolly/colly/v2"
)

// Crawler handles web scraping operations.
type Crawler struct {
	config    Config
	collector *colly.Collector

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

	return &Crawler{
		config:  cfg,
		results: make([]models.ScrapedPage, 0),
		queue:   NewURLQueue(),
		ctx:     ctx,
		cancel:  cancel,
	}, nil
}

// Crawl starts the crawling process from startURL.
func (c *Crawler) Crawl(startURL string) ([]models.ScrapedPage, error) {
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

// Stop cancels the crawl.
func (c *Crawler) Stop() {
	c.cancel()
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
			if _, retry := c.queue.Pop(); !retry {
				break
			} else {
				// Put it back and continue the loop.
				c.queue.Push(link)
				continue
			}
		}

		if visited[link.URL] {
			continue
		}
		visited[link.URL] = true

		if err := c.collector.Visit(link.URL); err != nil {
			c.logVerbose("visit failed: %s — %v", link.URL, err)
			if c.onError != nil {
				c.onError(link.URL, err)
			}
		}
	}

	c.collector.Wait()
	return c.GetResults(), nil
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
