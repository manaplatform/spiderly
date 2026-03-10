package crawler

import (
	"context"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"spiderly/internal/models"

	"github.com/gocolly/colly/v2"
)

// Config holds crawler configuration
type Config struct {
	MaxPages    int
	MaxDepth    int
	Concurrency int
	Delay       time.Duration
	Timeout     time.Duration
	Headless    bool
	SitemapMode bool
}

// Callbacks for real-time updates
type Callbacks struct {
	OnPageScraped    func(models.ScrapedPage)
	OnError          func(url string, err error)
	OnLinkDiscovered func(models.DiscoveredLink)
}

// Crawler manages the web crawling process
type Crawler struct {
	config      Config
	collector   *colly.Collector
	visited     map[string]bool
	results     []models.ScrapedPage
	queue       []queueItem
	mu          sync.RWMutex
	callbacks   Callbacks
	ctx         context.Context
	cancel      context.CancelFunc
	httpClient  *http.Client
}

type queueItem struct {
	URL   string
	Depth int
}

// NewCrawler creates a new crawler instance
func NewCrawler(cfg Config) *Crawler {
	ctx, cancel := context.WithCancel(context.Background())
	
	// Set defaults
	if cfg.Concurrency == 0 {
		cfg.Concurrency = 5
	}
	if cfg.MaxDepth == 0 {
		cfg.MaxDepth = 3
	}
	if cfg.Timeout == 0 {
		cfg.Timeout = 30 * time.Second
	}
	
	c := &Crawler{
		config:  cfg,
		visited: make(map[string]bool),
		results: make([]models.ScrapedPage, 0),
		queue:   make([]queueItem, 0),
		ctx:     ctx,
		cancel:  cancel,
		httpClient: &http.Client{
			Timeout: cfg.Timeout,
		},
	}
	
	c.setupCollector()
	return c
}

// setupCollector configures the colly collector
func (c *Crawler) setupCollector() {
	c.collector = colly.NewCollector(
		colly.Async(true),
		colly.MaxDepth(c.config.MaxDepth),
	)
	
	c.collector.SetRequestTimeout(c.config.Timeout)
	
	// Rate limiting
	c.collector.Limit(&colly.LimitRule{
		DomainGlob:  "*",
		Parallelism: c.config.Concurrency,
		Delay:       c.config.Delay,
	})
	
	// Set up handlers
	c.collector.OnHTML("html", func(e *colly.HTMLElement) {
		c.handlePage(e)
	})
	
	// Only discover links in recursive mode
	if !c.config.SitemapMode {
		c.collector.OnHTML("a[href]", func(e *colly.HTMLElement) {
			c.handleLink(e)
		})
	}
	
	c.collector.OnError(func(r *colly.Response, err error) {
		if c.callbacks.OnError != nil {
			c.callbacks.OnError(r.Request.URL.String(), err)
		}
	})
}

// handlePage processes a scraped page
func (c *Crawler) handlePage(e *colly.HTMLElement) {
	c.mu.Lock()
	defer c.mu.Unlock()
	
	// Check page limit
	if c.config.MaxPages > 0 && len(c.results) >= c.config.MaxPages {
		return
	}
	
	pageURL := e.Request.URL.String()
	
	page := models.ScrapedPage{
		URL:         pageURL,
		Title:       strings.TrimSpace(e.ChildText("title")),
		StatusCode:  e.Response.StatusCode,
		ContentType: e.Response.Headers.Get("Content-Type"),
		ScrapedAt:   time.Now(),
	}
	
	// Extract metadata
	e.ForEach("meta", func(_ int, el *colly.HTMLElement) {
		name := el.Attr("name")
		property := el.Attr("property")
		content := el.Attr("content")
		
		switch {
		case name == "description" || property == "og:description":
			if page.Description == "" {
				page.Description = content
			}
		case name == "keywords":
			page.Keywords = content
		case name == "author":
			page.Author = content
		case property == "og:image":
			page.OGImage = content
		case property == "article:published_time":
			page.PublishedDate = content
		}
	})
	
	// Extract H1
	page.H1 = strings.TrimSpace(e.ChildText("h1"))
	
	// Extract body text (simplified)
	page.BodyText = extractBodyText(e)
	
	c.results = append(c.results, page)
	
	if c.callbacks.OnPageScraped != nil {
		c.callbacks.OnPageScraped(page)
	}
}

// handleLink processes discovered links
func (c *Crawler) handleLink(e *colly.HTMLElement) {
	href := e.Attr("href")
	if href == "" {
		return
	}
	
	// Resolve relative URL
	absURL := e.Request.AbsoluteURL(href)
	if absURL == "" {
		return
	}
	
	// Parse and validate
	parsed, err := url.Parse(absURL)
	if err != nil {
		return
	}
	
	// Skip non-HTTP schemes
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return
	}
	
	// Skip fragments and query strings for simplicity
	parsed.Fragment = ""
	cleanURL := parsed.String()
	
	// Check if already visited
	c.mu.Lock()
	if c.visited[cleanURL] {
		c.mu.Unlock()
		return
	}
	c.visited[cleanURL] = true
	c.mu.Unlock()
	
	// Emit discovered link
	if c.callbacks.OnLinkDiscovered != nil {
		c.callbacks.OnLinkDiscovered(models.DiscoveredLink{
			URL:        cleanURL,
			SourceURL:  e.Request.URL.String(),
			Depth:      e.Request.Depth + 1,
			AnchorText: strings.TrimSpace(e.Text),
		})
	}
	
	// Visit the link
	e.Request.Visit(cleanURL)
}

// extractBodyText extracts readable text from the page
func extractBodyText(e *colly.HTMLElement) string {
	// Simple extraction - get text from main content areas
	var textParts []string
	
	e.ForEach("article, main, .content, .post, .entry, p", func(_ int, el *colly.HTMLElement) {
		text := strings.TrimSpace(el.Text)
		if len(text) > 20 {
			textParts = append(textParts, text)
		}
	})
	
	fullText := strings.Join(textParts, " ")
	
	// Limit text length
	if len(fullText) > 5000 {
		fullText = fullText[:5000]
	}
	
	return fullText
}

// QueueURL adds a URL to the crawl queue
func (c *Crawler) QueueURL(url string, depth int) {
	c.mu.Lock()
	defer c.mu.Unlock()
	
	if !c.visited[url] {
		c.queue = append(c.queue, queueItem{URL: url, Depth: depth})
		c.visited[url] = true
	}
}

// Crawl starts the crawling process
func (c *Crawler) Crawl(startURL string) ([]models.ScrapedPage, error) {
	// Mark start URL as visited
	c.mu.Lock()
	c.visited[startURL] = true
	c.mu.Unlock()
	
	// If we have queued URLs (sitemap mode), process them
	if len(c.queue) > 0 {
		for _, item := range c.queue {
			select {
			case <-c.ctx.Done():
				break
			default:
				c.collector.Visit(item.URL)
			}
		}
	} else {
		// Start from the provided URL
		c.collector.Visit(startURL)
	}
	
	// Wait for completion
	c.collector.Wait()
	
	c.mu.RLock()
	results := make([]models.ScrapedPage, len(c.results))
	copy(results, c.results)
	c.mu.RUnlock()
	
	return results, nil
}

// Stop cancels the crawl
func (c *Crawler) Stop() {
	c.cancel()
}

// Callback setters
func (c *Crawler) OnPageScraped(fn func(models.ScrapedPage)) {
	c.callbacks.OnPageScraped = fn
}

func (c *Crawler) OnError(fn func(string, error)) {
	c.callbacks.OnError = fn
}

func (c *Crawler) OnLinkDiscovered(fn func(models.DiscoveredLink)) {
	c.callbacks.OnLinkDiscovered = fn
}
