package crawler

import (
	"context"
	"fmt"
	"net/url"
	"strings"
	"sync"
	"time"

	"spiderly/internal/models"
	"spiderly/internal/web"

	"github.com/PuerkitoBio/goquery"
	"github.com/chromedp/chromedp"
)

// Config holds crawler configuration
type Config struct {
	MaxDepth       int
	MaxPages       int
	Timeout        time.Duration
	WaitTime       time.Duration
	FollowExternal bool
	Selectors      ContentSelectors
}

// ContentSelectors defines CSS selectors for content extraction
type ContentSelectors struct {
	Title   []string
	Content []string
	Summary []string
	Author  []string
	Date    []string
	Tags    []string
	Image   []string
}

// DefaultSelectors returns common news site selectors
func DefaultSelectors() ContentSelectors {
	return ContentSelectors{
		Title: []string{
			"h1.entry-title", "h1.post-title", "h1.article-title",
			"h1.news-title", "article h1", ".entry-title",
			".post-title", "h1",
		},
		Content: []string{
			"article .entry-content", ".post-content", ".article-content",
			".entry-content", ".news-content", "article p",
			".content p", "main p",
		},
		Summary: []string{
			".entry-summary", ".post-excerpt", ".article-summary",
			".lead", ".excerpt", "meta[name='description']",
		},
		Author: []string{
			".author-name", ".entry-author", ".post-author",
			"[rel='author']", ".byline",
		},
		Date: []string{
			"time[datetime]", ".entry-date", ".post-date",
			".publish-date", ".date",
		},
		Tags: []string{
			".tags a", ".post-tags a", ".entry-tags a", "[rel='tag']",
		},
		Image: []string{
			"article img", ".featured-image img", ".post-thumbnail img",
		},
	}
}

// Crawler handles web crawling operations
type Crawler struct {
	config     Config
	server     *web.Server
	visited    map[string]bool
	visitedMux sync.RWMutex
	baseURL    *url.URL
	results    []models.CrawlResult
	resultsMux sync.Mutex
	startTime  time.Time
}

// NewCrawler creates a new Crawler instance
func NewCrawler(config Config, server *web.Server) *Crawler {
	if config.MaxDepth == 0 {
		config.MaxDepth = 2
	}
	if config.MaxPages == 0 {
		config.MaxPages = 10
	}
	if config.Timeout == 0 {
		config.Timeout = 30 * time.Second
	}
	if config.WaitTime == 0 {
		config.WaitTime = 2 * time.Second
	}
	if len(config.Selectors.Title) == 0 {
		config.Selectors = DefaultSelectors()
	}

	return &Crawler{
		config:  config,
		server:  server,
		visited: make(map[string]bool),
		results: make([]models.CrawlResult, 0),
	}
}

// sendLog sends a log message to the dashboard
func (c *Crawler) sendLog(level, message string) {
	c.server.Hub.SendMessage(models.WSMessage{
		Type: "log",
		Payload: models.LogEntry{
			Level:     level,
			Message:   message,
			Timestamp: time.Now().Format("15:04:05"),
		},
	})
}

// sendProgress sends progress update to the dashboard
func (c *Crawler) sendProgress(currentURL string) {
	c.visitedMux.RLock()
	done := len(c.visited)
	c.visitedMux.RUnlock()

	progress := float64(done) / float64(c.config.MaxPages) * 100
	if progress > 100 {
		progress = 100
	}

	c.server.Hub.SendMessage(models.WSMessage{
		Type: "progress",
		Payload: models.ProgressPayload{
			CurrentURL: currentURL,
			Progress:   progress,
			PagesDone:  done,
			PagesTotal: c.config.MaxPages,
		},
	})
}

// sendStats sends statistics update to the dashboard
func (c *Crawler) sendStats() {
	totalPages, totalNews, totalLinks, errors := c.GetStats()
	elapsed := time.Since(c.startTime)
	min := int(elapsed.Minutes())
	sec := int(elapsed.Seconds()) % 60

	c.server.Hub.SendMessage(models.WSMessage{
		Type: "stats",
		Payload: models.StatsPayload{
			TotalPages:  totalPages,
			TotalNews:   totalNews,
			TotalLinks:  totalLinks,
			Errors:      errors,
			ElapsedTime: fmt.Sprintf("%02d:%02d", min, sec),
		},
	})
}

// Crawl starts crawling from the given URL
func (c *Crawler) Crawl(startURL string) ([]models.CrawlResult, error) {
	parsedURL, err := url.Parse(startURL)
	if err != nil {
		return nil, err
	}
	c.baseURL = parsedURL
	c.startTime = time.Now()

	// Notify dashboard that crawling has started
	c.server.Hub.SendMessage(models.WSMessage{Type: "started", Payload: nil})
	c.sendLog("info", "شروع خزش از: "+startURL)

	// Give time for WebSocket clients to connect
	time.Sleep(2 * time.Second)

	c.crawlPage(startURL, 0)

	// Send final stats
	totalPages, totalNews, totalLinks, errors := c.GetStats()
	elapsed := time.Since(c.startTime)
	min := int(elapsed.Minutes())
	sec := int(elapsed.Seconds()) % 60

	c.server.Hub.SendMessage(models.WSMessage{
		Type: "finished",
		Payload: models.StatsPayload{
			TotalPages:  totalPages,
			TotalNews:   totalNews,
			TotalLinks:  totalLinks,
			Errors:      errors,
			ElapsedTime: fmt.Sprintf("%02d:%02d", min, sec),
			Status:      "completed",
		},
	})

	c.sendLog("success", fmt.Sprintf("خزش به پایان رسید! %d صفحه | %d خبر | %d لینک", totalPages, totalNews, totalLinks))

	return c.results, nil
}

// crawlPage crawls a single page and discovers links
func (c *Crawler) crawlPage(pageURL string, depth int) {
	// Check if already visited
	c.visitedMux.RLock()
	if c.visited[pageURL] {
		c.visitedMux.RUnlock()
		return
	}
	c.visitedMux.RUnlock()

	// Check limits
	c.visitedMux.Lock()
	if len(c.visited) >= c.config.MaxPages {
		c.visitedMux.Unlock()
		return
	}
	c.visited[pageURL] = true
	c.visitedMux.Unlock()

	// Check depth
	if depth > c.config.MaxDepth {
		return
	}

	c.sendLog("info", fmt.Sprintf("در حال خزش [عمق %d]: %s", depth, pageURL))
	c.sendProgress(pageURL)

	startTime := time.Now()
	result := models.CrawlResult{URL: pageURL}

	// Fetch and render the page
	htmlContent, err := c.fetchPage(pageURL)
	if err != nil {
		result.Error = err
		result.ErrorMsg = err.Error()
		c.sendLog("error", "خطا در دریافت: "+pageURL+" - "+err.Error())
		c.addResult(result)
		c.sendStats()
		return
	}

	result.ElapsedTime = time.Since(startTime)
	result.StatusCode = 200
	c.sendLog("success", fmt.Sprintf("دریافت شد (%s): %s", result.ElapsedTime.Round(time.Millisecond), pageURL))

	// Parse HTML
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(htmlContent))
	if err != nil {
		result.Error = err
		result.ErrorMsg = err.Error()
		c.addResult(result)
		return
	}

	// Extract news content
	news := c.extractNews(doc, pageURL)
	if news != nil && (news.Title != "" || news.Content != "") {
		result.News = news
		c.server.Hub.SendMessage(models.WSMessage{
			Type:    "news",
			Payload: news,
		})
		c.sendLog("info", "📰 خبر استخراج شد: "+news.Title)
	}

	// Extract links
	links := c.extractLinks(doc, pageURL, depth)
	result.Links = links

	// Send discovered links to dashboard
	for _, link := range links {
		c.server.Hub.SendMessage(models.WSMessage{
			Type:    "link",
			Payload: link,
		})
	}

	c.addResult(result)
	c.sendStats()

	// Recursively crawl discovered links
	for _, link := range links {
		c.visitedMux.RLock()
		visitedCount := len(c.visited)
		c.visitedMux.RUnlock()

		if visitedCount >= c.config.MaxPages {
			break
		}

		if depth < c.config.MaxDepth {
			time.Sleep(500 * time.Millisecond) // Be polite
			c.crawlPage(link.URL, depth+1)
		}
	}
}

// fetchPage fetches a page using chromedp
func (c *Crawler) fetchPage(pageURL string) (string, error) {
	opts := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.Flag("headless", true),
		chromedp.Flag("disable-gpu", true),
		chromedp.Flag("no-sandbox", true),
		chromedp.Flag("disable-dev-shm-usage", true),
	)

	allocCtx, allocCancel := chromedp.NewExecAllocator(context.Background(), opts...)
	defer allocCancel()

	ctx, cancel := chromedp.NewContext(allocCtx)
	defer cancel()

	ctx, cancel = context.WithTimeout(ctx, c.config.Timeout)
	defer cancel()

	var htmlContent string

	err := chromedp.Run(ctx,
		chromedp.Navigate(pageURL),
		chromedp.WaitVisible("body", chromedp.ByQuery),
		chromedp.Sleep(c.config.WaitTime),
		chromedp.OuterHTML("html", &htmlContent),
	)

	return htmlContent, err
}

// extractNews extracts news content from a page
func (c *Crawler) extractNews(doc *goquery.Document, pageURL string) *models.News {
	news := &models.News{
		URL:       pageURL,
		ScrapedAt: time.Now(),
	}

	// Extract title
	for _, sel := range c.config.Selectors.Title {
		if title := strings.TrimSpace(doc.Find(sel).First().Text()); title != "" {
			news.Title = title
			break
		}
	}

	// Extract content
	var contentParts []string
	for _, sel := range c.config.Selectors.Content {
		doc.Find(sel).Each(func(i int, s *goquery.Selection) {
			if text := strings.TrimSpace(s.Text()); text != "" {
				contentParts = append(contentParts, text)
			}
		})
		if len(contentParts) > 0 {
			break
		}
	}
	news.Content = strings.Join(contentParts, "\n\n")

	// Extract summary
	for _, sel := range c.config.Selectors.Summary {
		if strings.HasPrefix(sel, "meta") {
			if content, exists := doc.Find(sel).Attr("content"); exists && content != "" {
				news.Summary = content
				break
			}
		} else if summary := strings.TrimSpace(doc.Find(sel).First().Text()); summary != "" {
			news.Summary = summary
			break
		}
	}

	// Extract author
	for _, sel := range c.config.Selectors.Author {
		if author := strings.TrimSpace(doc.Find(sel).First().Text()); author != "" {
			news.Author = author
			break
		}
	}

	// Extract date
	for _, sel := range c.config.Selectors.Date {
		elem := doc.Find(sel).First()
		if datetime, exists := elem.Attr("datetime"); exists {
			news.PublishedAt = datetime
			break
		}
		if date := strings.TrimSpace(elem.Text()); date != "" {
			news.PublishedAt = date
			break
		}
	}

	// Extract tags
	for _, sel := range c.config.Selectors.Tags {
		doc.Find(sel).Each(func(i int, s *goquery.Selection) {
			if tag := strings.TrimSpace(s.Text()); tag != "" {
				news.Tags = append(news.Tags, tag)
			}
		})
		if len(news.Tags) > 0 {
			break
		}
	}

	// Extract image
	for _, sel := range c.config.Selectors.Image {
		elem := doc.Find(sel).First()
		if src, exists := elem.Attr("src"); exists {
			news.ImageURL = c.resolveURL(src)
			break
		}
	}

	return news
}

// extractLinks extracts links from a page
func (c *Crawler) extractLinks(doc *goquery.Document, pageURL string, currentDepth int) []models.Link {
	var links []models.Link
	seen := make(map[string]bool)

	doc.Find("a[href]").Each(func(i int, s *goquery.Selection) {
		href, exists := s.Attr("href")
		if !exists || href == "" {
			return
		}

		absoluteURL := c.resolveURL(href)
		if absoluteURL == "" || seen[absoluteURL] {
			return
		}
		seen[absoluteURL] = true

		if !c.config.FollowExternal && !c.isSameDomain(absoluteURL) {
			return
		}
		if !strings.HasPrefix(absoluteURL, "http") {
			return
		}
		if c.shouldSkipURL(absoluteURL) {
			return
		}

		linkText := strings.TrimSpace(s.Text())
		if linkText == "" {
			linkText = "[بدون عنوان]"
		}

		links = append(links, models.Link{
			URL:   absoluteURL,
			Text:  linkText,
			Depth: currentDepth + 1,
		})
	})

	return links
}

// resolveURL converts relative URLs to absolute
func (c *Crawler) resolveURL(href string) string {
	if c.baseURL == nil {
		return href
	}
	parsedHref, err := url.Parse(href)
	if err != nil {
		return ""
	}
	return c.baseURL.ResolveReference(parsedHref).String()
}

// isSameDomain checks if a URL is from the same domain
func (c *Crawler) isSameDomain(checkURL string) bool {
	if c.baseURL == nil {
		return true
	}
	parsed, err := url.Parse(checkURL)
	if err != nil {
		return false
	}
	return parsed.Host == c.baseURL.Host
}

// shouldSkipURL checks if a URL should be skipped
func (c *Crawler) shouldSkipURL(checkURL string) bool {
	skipPatterns := []string{
		"javascript:", "mailto:", "tel:", "#",
		".pdf", ".jpg", ".jpeg", ".png", ".gif",
		".mp4", ".mp3", ".zip", ".rar",
		"/login", "/signup", "/register", "/cart", "/checkout",
	}
	lowerURL := strings.ToLower(checkURL)
	for _, pattern := range skipPatterns {
		if strings.Contains(lowerURL, pattern) {
			return true
		}
	}
	return false
}

// addResult safely adds a result
func (c *Crawler) addResult(result models.CrawlResult) {
	c.resultsMux.Lock()
	defer c.resultsMux.Unlock()
	c.results = append(c.results, result)
}

// GetStats returns crawling statistics
func (c *Crawler) GetStats() (totalPages, totalNews, totalLinks, errors int) {
	c.resultsMux.Lock()
	defer c.resultsMux.Unlock()

	totalPages = len(c.results)
	for _, result := range c.results {
		if result.News != nil && result.News.Title != "" {
			totalNews++
		}
		totalLinks += len(result.Links)
		if result.Error != nil {
			errors++
		}
	}
	return
}
