package crawler

import (
	"context"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"sync"
	"time"

	"spiderly/internal/extractor"
	"spiderly/internal/models"

	"github.com/gocolly/colly/v2"
)

type Config struct {
	MaxPages       int
	MaxDepth       int
	Concurrency    int
	Delay          time.Duration
	Timeout        time.Duration
	Headless       bool
	SitemapMode    bool
	ProductMode    bool
	ProductPattern *regexp.Regexp
}

type Callbacks struct {
	OnPageScraped    func(models.ScrapedPage)
	OnError          func(url string, err error)
	OnLinkDiscovered func(models.DiscoveredLink)
}

type Crawler struct {
	config     Config
	collector  *colly.Collector
	visited    map[string]bool
	results    []models.ScrapedPage
	queue      []queueItem
	mu         sync.RWMutex
	callbacks  Callbacks
	ctx        context.Context
	cancel     context.CancelFunc
	httpClient *http.Client
}

type queueItem struct {
	URL   string
	Depth int
}

func NewCrawler(cfg Config) *Crawler {
	ctx, cancel := context.WithCancel(context.Background())

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
		results: []models.ScrapedPage{},
		queue:   []queueItem{},
		ctx:     ctx,
		cancel:  cancel,
		httpClient: &http.Client{
			Timeout: cfg.Timeout,
		},
	}

	c.setupCollector()
	return c
}

func (c *Crawler) setupCollector() {
	c.collector = colly.NewCollector(
		colly.Async(true),
		colly.MaxDepth(c.config.MaxDepth),
	)

	c.collector.SetRequestTimeout(c.config.Timeout)

	c.collector.Limit(&colly.LimitRule{
		DomainGlob:  "*",
		Parallelism: c.config.Concurrency,
		Delay:       c.config.Delay,
	})

	// Page handler
	c.collector.OnHTML("html", func(e *colly.HTMLElement) {
		c.handlePage(e)
	})

	// Link handler
	if !c.config.SitemapMode {
		c.collector.OnHTML("a[href]", func(e *colly.HTMLElement) {
			c.handleLink(e)
		})
	}

	// Error handler
	c.collector.OnError(func(r *colly.Response, err error) {
		if c.callbacks.OnError != nil {
			c.callbacks.OnError(r.Request.URL.String(), err)
		}
	})
}

func (c *Crawler) handlePage(e *colly.HTMLElement) {
	pageURL := e.Request.URL.String()

	page := models.ScrapedPage{
		URL:         pageURL,
		Title:       strings.TrimSpace(e.ChildText("title")),
		StatusCode:  e.Response.StatusCode,
		ContentType: e.Response.Headers.Get("Content-Type"),
		ScrapedAt:   time.Now(),
	}

	// -------- PRODUCT DETECTION FIRST --------
	// Determine if we should attempt product extraction
	shouldExtract := false

	if c.config.ProductMode {
		if c.config.ProductPattern == nil {
			// If no pattern is provided via CLI, attempt extraction on all scraped pages
			shouldExtract = true
		} else if c.config.ProductPattern.MatchString(pageURL) {
			// If a pattern is provided, strictly enforce the regex match
			shouldExtract = true
		}
	}

	if shouldExtract {
		// e.DOM is the parsed *goquery.Selection for the HTML document
		productData := extractor.ExtractProduct(e.DOM, pageURL)
		if productData != nil {
			page.Product = productData
		}
	}

	// -------- META EXTRACTION --------
	e.ForEach("meta", func(_ int, el *colly.HTMLElement) {
		name := el.Attr("name")
		prop := el.Attr("property")
		content := el.Attr("content")

		switch {
		case name == "description" || prop == "og:description":
			if page.Description == "" {
				page.Description = content
			}
		case name == "keywords":
			page.Keywords = content
		case name == "author":
			page.Author = content
		case prop == "og:image":
			page.OGImage = content
		case prop == "article:published_time":
			page.PublishedDate = content
		}
	})

	// -------- BODY & H1 --------
	page.H1 = strings.TrimSpace(e.ChildText("h1"))
	page.BodyText = extractBodyText(e)

	// Save
	c.mu.Lock()
	if c.config.MaxPages == 0 || len(c.results) < c.config.MaxPages {
		c.results = append(c.results, page)
	}
	c.mu.Unlock()

	if c.callbacks.OnPageScraped != nil {
		c.callbacks.OnPageScraped(page)
	}
}

func (c *Crawler) handleLink(e *colly.HTMLElement) {
	href := strings.TrimSpace(e.Attr("href"))
	if href == "" {
		return
	}

	abs := e.Request.AbsoluteURL(href)
	if abs == "" {
		return
	}

	u, err := url.Parse(abs)
	if err != nil {
		return
	}

	if u.Scheme != "http" && u.Scheme != "https" {
		return
	}

	u.Fragment = ""
	cleanURL := u.String()

	c.mu.Lock()
	if c.visited[cleanURL] {
		c.mu.Unlock()
		return
	}
	c.visited[cleanURL] = true
	c.mu.Unlock()

	if c.callbacks.OnLinkDiscovered != nil {
		c.callbacks.OnLinkDiscovered(models.DiscoveredLink{
			URL:        cleanURL,
			SourceURL:  e.Request.URL.String(),
			Depth:      e.Request.Depth + 1,
			AnchorText: strings.TrimSpace(e.Text),
		})
	}

	_ = e.Request.Visit(cleanURL)
}

func extractBodyText(e *colly.HTMLElement) string {
	var out []string

	e.ForEach("article, main, .content, .post, .entry, p", func(_ int, el *colly.HTMLElement) {
		t := strings.TrimSpace(el.Text)
		if len(t) > 20 {
			out = append(out, t)
		}
	})

	joined := strings.Join(out, " ")

	if len(joined) > 5000 {
		joined = joined[:5000]
	}

	return joined
}

func (c *Crawler) QueueURL(u string, depth int) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if !c.visited[u] {
		c.visited[u] = true
		c.queue = append(c.queue, queueItem{URL: u, Depth: depth})
	}
}

func (c *Crawler) Crawl(startURL string) ([]models.ScrapedPage, error) {
	c.mu.Lock()
	c.visited[startURL] = true
	c.mu.Unlock()

	if len(c.queue) > 0 {
		for _, it := range c.queue {
			if c.ctx.Err() != nil {
				break
			}
			c.collector.Visit(it.URL)
		}
	} else {
		c.collector.Visit(startURL)
	}

	c.collector.Wait()

	c.mu.RLock()
	out := make([]models.ScrapedPage, len(c.results))
	copy(out, c.results)
	c.mu.RUnlock()

	return out, nil
}

func (c *Crawler) Stop() { c.cancel() }

func (c *Crawler) OnPageScraped(fn func(models.ScrapedPage)) {
	c.callbacks.OnPageScraped = fn
}

func (c *Crawler) OnError(fn func(string, error)) {
	c.callbacks.OnError = fn
}

func (c *Crawler) OnLinkDiscovered(fn func(models.DiscoveredLink)) {
	c.callbacks.OnLinkDiscovered = fn
}
