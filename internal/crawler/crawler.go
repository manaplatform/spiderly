package crawler

import (
	"context"
	"fmt"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/chromedp/chromedp"

	"spiderly/internal/models"
	"spiderly/internal/sitemap"
)

// Crawler handles web crawling operations
type Crawler struct {
	config     Config
	visited    map[string]bool
	visitedMu  sync.RWMutex
	queue      []QueueItem
	queueMu    sync.Mutex
	
	// Sitemap parser
	sitemapParser *sitemap.Parser
	sitemapURLs   []models.SitemapURL

	// Callbacks
	onNews      func(news models.News)
	onLog       func(level, message string)
	onStats     func(stats models.CrawlStats)
	onLink      func(link models.DiscoveredLink)
	onProgress  func(progress float64)

	// Stats
	stats      models.CrawlStats
	statsMu    sync.RWMutex
	startTime  time.Time
}

// Config holds crawler configuration
type Config struct {
	MaxDepth       int
	MaxPages       int
	Timeout        time.Duration
	FollowExternal bool
	UserAgent      string
	Delay          time.Duration
	
	// Sitemap mode settings
	SitemapMode    bool
	SitemapURL     string  // Direct sitemap URL (optional)
	AutoDiscover   bool    // Auto-discover sitemaps from robots.txt
	MinPriority    float64 // Minimum priority filter (0-1)
	URLPattern     string  // Regex pattern to filter URLs
}

// QueueItem represents a URL in the crawl queue
type QueueItem struct {
	URL    string
	Depth  int
	Source string // "crawl" or "sitemap"
}

// NewCrawler creates a new crawler instance
func NewCrawler(config Config) *Crawler {
	if config.UserAgent == "" {
		config.UserAgent = "Spiderly/1.0 (Web Crawler)"
	}
	if config.Timeout == 0 {
		config.Timeout = 30 * time.Second
	}
	if config.Delay == 0 {
		config.Delay = 500 * time.Millisecond
	}

	c := &Crawler{
		config:  config,
		visited: make(map[string]bool),
		queue:   make([]QueueItem, 0),
	}

	// Initialize sitemap parser if in sitemap mode
	if config.SitemapMode {
		c.sitemapParser = sitemap.NewParser(sitemap.ParserConfig{
			UserAgent:   config.UserAgent,
			MaxSitemaps: 100,
			Timeout:     config.Timeout,
		})
	}

	return c
}

// SetNewsCallback sets the callback for discovered news
func (c *Crawler) SetNewsCallback(fn func(news models.News)) {
	c.onNews = fn
}

// SetLogCallback sets the logging callback
func (c *Crawler) SetLogCallback(fn func(level, message string)) {
	c.onLog = fn
	if c.sitemapParser != nil {
		c.sitemapParser.SetLogCallback(fn)
	}
}

// SetStatsCallback sets the stats update callback
func (c *Crawler) SetStatsCallback(fn func(stats models.CrawlStats)) {
	c.onStats = fn
}

// SetLinkCallback sets the discovered link callback
func (c *Crawler) SetLinkCallback(fn func(link models.DiscoveredLink)) {
	c.onLink = fn
}

// SetProgressCallback sets the progress callback
func (c *Crawler) SetProgressCallback(fn func(progress float64)) {
	c.onProgress = fn
}

func (c *Crawler) log(level, message string) {
	if c.onLog != nil {
		c.onLog(level, message)
	}
}

func (c *Crawler) updateStats() {
	c.statsMu.Lock()
	elapsed := time.Since(c.startTime)
	c.stats.ElapsedTime = formatDuration(elapsed)
	if c.stats.TotalURLs > 0 {
		c.stats.Progress = float64(c.stats.ProcessedURLs) / float64(c.stats.TotalURLs) * 100
	}
	stats := c.stats
	c.statsMu.Unlock()

	if c.onStats != nil {
		c.onStats(stats)
	}
	if c.onProgress != nil {
		c.onProgress(stats.Progress)
	}
}

// Start begins the crawling process
func (c *Crawler) Start(ctx context.Context, startURL string) error {
	c.startTime = time.Now()
	c.stats = models.CrawlStats{
		SitemapMode: c.config.SitemapMode,
	}

	c.log("info", fmt.Sprintf("🕷️ Spiderly starting crawl of: %s", startURL))
	c.log("info", fmt.Sprintf("⚙️ Mode: %s | Max Pages: %d", 
		map[bool]string{true: "Sitemap", false: "Recursive"}[c.config.SitemapMode], 
		c.config.MaxPages))

	// If sitemap mode, discover and parse sitemaps first
	if c.config.SitemapMode {
		return c.startSitemapMode(ctx, startURL)
	}

	// Regular recursive crawl mode
	return c.startRecursiveMode(ctx, startURL)
}

// startSitemapMode handles sitemap-based crawling
func (c *Crawler) startSitemapMode(ctx context.Context, baseURL string) error {
	c.log("sitemap", "🗺️ Starting sitemap discovery and parsing...")

	// Setup sitemap parser callbacks
	c.sitemapParser.SetURLFoundCallback(func(smURL models.SitemapURL, source string) {
		if c.onLink != nil {
			priority := ""
			if smURL.Priority > 0 {
				priority = fmt.Sprintf("%.1f", smURL.Priority)
			}
			c.onLink(models.DiscoveredLink{
				URL:      smURL.Loc,
				Source:   "sitemap",
				Priority: priority,
				LastMod:  smURL.LastMod,
			})
		}
	})

	c.sitemapParser.SetSitemapFoundCallback(func(url string) {
		c.statsMu.Lock()
		c.stats.SitemapsFound++
		c.statsMu.Unlock()
		c.updateStats()
	})

	// Parse sitemaps
	var result *models.SitemapResult
	var err error

	if c.config.SitemapURL != "" {
		// Parse specific sitemap URL
		c.log("info", fmt.Sprintf("📍 Parsing specific sitemap: %s", c.config.SitemapURL))
		result, err = c.sitemapParser.ParseSingleSitemap(ctx, c.config.SitemapURL)
	} else {
		// Auto-discover sitemaps
		result, err = c.sitemapParser.DiscoverAndParse(ctx, baseURL)
	}

	if err != nil {
		return fmt.Errorf("sitemap parsing failed: %w", err)
	}

	if result.TotalURLs == 0 {
		c.log("warn", "⚠️ No URLs found in sitemaps")
		return nil
	}

	// Apply filters
	urls := result.URLs
	
	if c.config.MinPriority > 0 {
		urls = sitemap.FilterByPriority(urls, c.config.MinPriority)
		c.log("info", fmt.Sprintf("🔍 Filtered by priority (≥%.1f): %d URLs", c.config.MinPriority, len(urls)))
	}

	if c.config.URLPattern != "" {
		urls = sitemap.FilterByPattern(urls, c.config.URLPattern)
		c.log("info", fmt.Sprintf("🔍 Filtered by pattern: %d URLs", len(urls)))
	}

	c.sitemapURLs = urls
	
	c.statsMu.Lock()
	c.stats.SitemapURLs = len(urls)
	c.stats.TotalURLs = min(len(urls), c.config.MaxPages)
	c.statsMu.Unlock()

	c.log("success", fmt.Sprintf("📊 Sitemap Summary: %d sitemaps, %d total URLs, %d to process",
		result.SitemapsFound, len(urls), c.stats.TotalURLs))

	// Queue sitemap URLs for crawling
	for i, smURL := range urls {
		if i >= c.config.MaxPages {
			break
		}
		c.queue = append(c.queue, QueueItem{
			URL:    smURL.Loc,
			Depth:  0,
			Source: "sitemap",
		})
	}

	// Process the queue
	return c.processQueue(ctx)
}

// startRecursiveMode handles traditional recursive crawling
func (c *Crawler) startRecursiveMode(ctx context.Context, startURL string) error {
	c.queue = append(c.queue, QueueItem{
		URL:    startURL,
		Depth:  0,
		Source: "crawl",
	})

	c.statsMu.Lock()
	c.stats.TotalURLs = 1
	c.statsMu.Unlock()

	return c.processQueue(ctx)
}

// processQueue processes the URL queue
func (c *Crawler) processQueue(ctx context.Context) error {
	for {
		select {
		case <-ctx.Done():
			c.log("warn", "🛑 Crawling cancelled")
			return ctx.Err()
		default:
		}

		// Get next item from queue
		c.queueMu.Lock()
		if len(c.queue) == 0 {
			c.queueMu.Unlock()
			break
		}
		item := c.queue[0]
		c.queue = c.queue[1:]
		c.queueMu.Unlock()

		// Check if already visited
		c.visitedMu.RLock()
		if c.visited[item.URL] {
			c.visitedMu.RUnlock()
			continue
		}
		c.visitedMu.RUnlock()

		// Check page limit
		c.statsMu.RLock()
		processed := c.stats.ProcessedURLs
		c.statsMu.RUnlock()

		if processed >= c.config.MaxPages {
			c.log("info", fmt.Sprintf("📄 Reached page limit (%d)", c.config.MaxPages))
			break
		}

		// Mark as visited
		c.visitedMu.Lock()
		c.visited[item.URL] = true
		c.visitedMu.Unlock()

		// Update current URL in stats
		c.statsMu.Lock()
		c.stats.CurrentURL = item.URL
		c.statsMu.Unlock()
		c.updateStats()

		// Crawl the page
		c.log("info", fmt.Sprintf("🔗 [%d/%d] Crawling: %s", processed+1, c.config.MaxPages, truncateString(item.URL, 60)))
		
		news, links, err := c.crawlPage(ctx, item.URL, item.Depth)
		
		c.statsMu.Lock()
		c.stats.ProcessedURLs++
		if err != nil {
			c.stats.ErrorCount++
			c.statsMu.Unlock()
			c.log("error", fmt.Sprintf("❌ Error: %v", err))
		} else {
			c.stats.SuccessCount++
			c.statsMu.Unlock()

			// Emit news if found
			if news != nil && c.onNews != nil {
				c.onNews(*news)
			}

			// Add discovered links to queue (only in recursive mode)
			if !c.config.SitemapMode && item.Depth < c.config.MaxDepth {
				c.addLinksToQueue(links, item.Depth+1)
			}
		}

		c.updateStats()

		// Delay between requests
		time.Sleep(c.config.Delay)
	}

	c.log("success", fmt.Sprintf("✅ Crawling complete! Processed %d pages", c.stats.ProcessedURLs))
	return nil
}

// crawlPage fetches and parses a single page
func (c *Crawler) crawlPage(ctx context.Context, pageURL string, depth int) (*models.News, []string, error) {
	// Create browser context
	allocCtx, cancel := chromedp.NewExecAllocator(ctx,
		append(chromedp.DefaultExecAllocatorOptions[:],
			chromedp.Flag("headless", true),
			chromedp.Flag("disable-gpu", true),
			chromedp.Flag("no-sandbox", true),
			chromedp.UserAgent(c.config.UserAgent),
		)...,
	)
	defer cancel()

	browserCtx, cancel := chromedp.NewContext(allocCtx)
	defer cancel()

	timeoutCtx, cancel := context.WithTimeout(browserCtx, c.config.Timeout)
	defer cancel()

	var htmlContent string
	err := chromedp.Run(timeoutCtx,
		chromedp.Navigate(pageURL),
		chromedp.WaitReady("body"),
		chromedp.OuterHTML("html", &htmlContent),
	)

	if err != nil {
		return nil, nil, fmt.Errorf("failed to load page: %w", err)
	}

	// Parse HTML
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(htmlContent))
	if err != nil {
		return nil, nil, fmt.Errorf("failed to parse HTML: %w", err)
	}

	// Extract news content
	news := c.extractNews(doc, pageURL, depth)

	// Extract links
	var links []string
	doc.Find("a[href]").Each(func(i int, s *goquery.Selection) {
		if href, exists := s.Attr("href"); exists {
			absURL := resolveURL(pageURL, href)
			if absURL != "" && c.shouldFollow(pageURL, absURL) {
				links = append(links, absURL)
				if c.onLink != nil {
					c.onLink(models.DiscoveredLink{
						URL:    absURL,
						Source: "crawl",
						Depth:  depth + 1,
					})
				}
			}
		}
	})

	return news, links, nil
}

// extractNews extracts news content from a page
func (c *Crawler) extractNews(doc *goquery.Document, pageURL string, depth int) *models.News {
	news := &models.News{
		URL:       pageURL,
		ScrapedAt: time.Now(),
		Depth:     depth,
	}

	// Try various title selectors
	titleSelectors := []string{
		"h1.title", "h1.entry-title", "h1.post-title", "h1.article-title",
		"article h1", ".content h1", "main h1", "h1",
	}
	for _, sel := range titleSelectors {
		if title := strings.TrimSpace(doc.Find(sel).First().Text()); title != "" {
			news.Title = title
			break
		}
	}
	if news.Title == "" {
		news.Title = doc.Find("title").Text()
	}

	// Try various content selectors
	contentSelectors := []string{
		"article .content", "article .entry-content", ".post-content",
		"article p", ".article-body", ".story-body", "main .content",
	}
	for _, sel := range contentSelectors {
		if content := strings.TrimSpace(doc.Find(sel).Text()); len(content) > 100 {
			news.Content = truncateString(content, 500)
			break
		}
	}

	// Extract author
	authorSelectors := []string{
		".author", ".byline", "[rel='author']", ".writer",
	}
	for _, sel := range authorSelectors {
		if author := strings.TrimSpace(doc.Find(sel).First().Text()); author != "" {
			news.Author = author
			break
		}
	}

	// Extract date
	doc.Find("time, .date, .publish-date, .post-date").Each(func(i int, s *goquery.Selection) {
		if news.PublishDate == "" {
			if datetime, exists := s.Attr("datetime"); exists {
				news.PublishDate = datetime
			} else {
				news.PublishDate = strings.TrimSpace(s.Text())
			}
		}
	})

	// Extract tags
	doc.Find(".tags a, .tag, [rel='tag']").Each(func(i int, s *goquery.Selection) {
		if tag := strings.TrimSpace(s.Text()); tag != "" && len(news.Tags) < 10 {
			news.Tags = append(news.Tags, tag)
		}
	})

	return news
}

// addLinksToQueue adds discovered links to the crawl queue
func (c *Crawler) addLinksToQueue(links []string, depth int) {
	c.queueMu.Lock()
	defer c.queueMu.Unlock()

	for _, link := range links {
		c.visitedMu.RLock()
		visited := c.visited[link]
		c.visitedMu.RUnlock()

		if !visited {
			c.queue = append(c.queue, QueueItem{
				URL:    link,
				Depth:  depth,
				Source: "crawl",
			})
			
			c.statsMu.Lock()
			c.stats.TotalURLs++
			c.statsMu.Unlock()
		}
	}
}

// shouldFollow determines if a URL should be followed
func (c *Crawler) shouldFollow(baseURL, targetURL string) bool {
	baseParsed, err := url.Parse(baseURL)
	if err != nil {
		return false
	}

	targetParsed, err := url.Parse(targetURL)
	if err != nil {
		return false
	}

	// Skip non-HTTP(S) URLs
	if targetParsed.Scheme != "http" && targetParsed.Scheme != "https" {
		return false
	}

	// Skip common non-content files
	skipExtensions := []string{".jpg", ".jpeg", ".png", ".gif", ".svg", ".css", ".js", ".pdf", ".zip", ".mp4", ".mp3"}
	for _, ext := range skipExtensions {
		if strings.HasSuffix(strings.ToLower(targetParsed.Path), ext) {
			return false
		}
	}

	// Check if external links should be followed
	if !c.config.FollowExternal && baseParsed.Host != targetParsed.Host {
		return false
	}

	return true
}

// Helper functions
func resolveURL(base, href string) string {
	baseParsed, err := url.Parse(base)
	if err != nil {
		return ""
	}
	hrefParsed, err := url.Parse(href)
	if err != nil {
		return ""
	}
	resolved := baseParsed.ResolveReference(hrefParsed)
	return resolved.String()
}

func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}

func formatDuration(d time.Duration) string {
	d = d.Round(time.Second)
	h := d / time.Hour
	d -= h * time.Hour
	m := d / time.Minute
	d -= m * time.Minute
	s := d / time.Second
	
	if h > 0 {
		return fmt.Sprintf("%dh %dm %ds", h, m, s)
	}
	if m > 0 {
		return fmt.Sprintf("%dm %ds", m, s)
	}
	return fmt.Sprintf("%ds", s)
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
