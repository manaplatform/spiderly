// internal/crawler/handlers.go
package crawler

import (
	"fmt"
	"net/url"
	"strconv"
	"strings"

	"spiderly/internal/extractor"
	"spiderly/internal/models"

	"github.com/PuerkitoBio/goquery"
	"github.com/gocolly/colly/v2"
)

// DetectJSRequired checks whether a page is likely a client-side JavaScript application
// or empty shell that requires browser rendering.
func DetectJSRequired(statusCode int, body string, doc *goquery.Selection, clickCrawl bool) (bool, string) {
	if doc == nil {
		if strings.TrimSpace(body) == "" {
			return true, "empty body"
		}
		return false, ""
	}

	// 1. Check for noscript message explicitly stating JavaScript is needed
	noscriptText := strings.ToLower(doc.Find("noscript").Text())
	if strings.Contains(noscriptText, "enable javascript") ||
		strings.Contains(noscriptText, "javascript is required") ||
		strings.Contains(noscriptText, "need to enable javascript") ||
		strings.Contains(noscriptText, "requires javascript") ||
		strings.Contains(noscriptText, "turn on javascript") ||
		strings.Contains(noscriptText, "javascript must be enabled") {
		return true, "noscript tag requires JavaScript"
	}

	// 2. Check for empty SPA root containers (React, Vue, Next.js, Angular)
	for _, selector := range []string{"#root", "#app", "#__next", "app-root"} {
		sel := doc.Find(selector)
		if sel.Length() > 0 && strings.TrimSpace(sel.Text()) == "" {
			return true, fmt.Sprintf("empty SPA container (%s)", selector)
		}
	}

	// 3. Challenge / bot protection pages
	bodyLower := strings.ToLower(body)
	if strings.Contains(bodyLower, "cf-browser-verification") ||
		strings.Contains(bodyLower, "checking your browser before accessing") ||
		strings.Contains(bodyLower, "verifying you are human") ||
		strings.Contains(bodyLower, "attention required! | cloudflare") ||
		strings.Contains(bodyLower, "please turn javascript on and reload") {
		return true, "bot protection or JS challenge detected"
	}

	// 4. Extremely minimal visible body text with script tags present
	bodyText := strings.TrimSpace(doc.Find("body").Text())
	scriptCount := doc.Find("script").Length()
	if len(bodyText) < 120 && scriptCount > 0 && statusCode == 200 {
		return true, "minimal body text with client scripts"
	}

	// 5. If click crawl is enabled, check for interactive elements that require a browser to click
	if clickCrawl {
		if doc.Find("button, [role='button'], a[href='#'], a[href^='javascript:'], [class*='load-more' i], [class*='loadmore' i]").Length() > 0 {
			return true, "interactive elements present with click-crawl enabled"
		}
	}

	return false, ""
}

// handlePage is called for each HTML response. It extracts page data and triggers callbacks.
func (c *Crawler) handlePage(e *colly.HTMLElement) {
	select {
	case <-c.ctx.Done():
		return
	default:
	}

	pageURL := e.Request.URL.String()
	c.logVerbose("handling page: %s", pageURL)

	reqDepth := e.Request.Depth
	if dStr := e.Request.Ctx.Get("depth"); dStr != "" {
		if d, err := strconv.Atoi(dStr); err == nil {
			reqDepth = d
		}
	}
	parentURL := e.Request.Ctx.Get("parent_url")

	// Check if fallback browser rendering should be triggered
	if c.browser != nil && c.config.BrowserStrategy == "fallback" {
		if jsNeeded, reason := DetectJSRequired(e.Response.StatusCode, string(e.Response.Body), e.DOM, c.config.ClickCrawl); jsNeeded {
			c.logVerbose("JS rendering required for %s (%s) — re-fetching with ChromeDP", pageURL, reason)
			res, err := c.browser.Render(c.ctx, pageURL)
			if err == nil {
				doc, docErr := goquery.NewDocumentFromReader(strings.NewReader(res.HTML))
				if docErr == nil {
					// Count page atomically
					if !c.incrementScrapedCount(pageURL) {
						return
					}

					targetURL := res.FinalURL
					if targetURL == "" {
						targetURL = pageURL
					}

					renderedPage := c.extractPageFromDOM(targetURL, doc.Selection, res.StatusCode, res.ContentLength, reqDepth, parentURL)
					if res.Title != "" {
						renderedPage.Title = res.Title
					}
					if renderedPage.URL == "" {
						renderedPage.URL = pageURL
					}

					if !c.config.SitemapMode {
						c.extractLinksFromDOM(doc.Selection, targetURL, reqDepth)
					}
					if len(res.DiscoveredLinks) > 0 {
						c.enqueueDiscoveredLinks(res.DiscoveredLinks, reqDepth, targetURL)
					}

					c.addResult(renderedPage)
					if c.onPageScraped != nil {
						c.onPageScraped(renderedPage)
					}
					return
				}
			} else {
				c.logVerbose("fallback ChromeDP render failed for %s (%v) — retaining Colly result", pageURL, err)
			}
		}
	}

	// Standard Colly path
	if !c.incrementScrapedCount(pageURL) {
		return
	}

	page := c.extractPageFromDOM(pageURL, e.DOM, e.Response.StatusCode, int64(len(e.Response.Body)), reqDepth, parentURL)

	if !c.config.SitemapMode {
		c.extractLinksFromDOM(e.DOM, pageURL, reqDepth)
	}

	c.addResult(page)
	if c.onPageScraped != nil {
		c.onPageScraped(page)
	}
}

// incrementScrapedCount atomically checks and updates the scraped counter.
func (c *Crawler) incrementScrapedCount(pageURL string) bool {
	if c.config.MaxPages > 0 {
		current := c.scraped.Add(1)
		if current > int64(c.config.MaxPages) {
			c.scraped.Add(-1) // roll back
			c.logVerbose("max pages (%d) reached, dropping %s", c.config.MaxPages, pageURL)
			return false
		}
	} else {
		c.scraped.Add(1)
	}
	return true
}

// extractPageFromDOM extracts structured data from a GoQuery selection.
func (c *Crawler) extractPageFromDOM(pageURL string, sel *goquery.Selection, statusCode int, contentLength int64, depth int, parentURL string) models.ScrapedPage {
	title := strings.TrimSpace(sel.Find("title").First().Text())

	page := models.ScrapedPage{
		URL:           pageURL,
		Title:         title,
		StatusCode:    statusCode,
		ContentLength: contentLength,
		Depth:         depth,
		ParentURL:     parentURL,
	}

	if !c.config.ProductMode || c.config.ExtractSpecs || c.config.ExtractImages || c.config.NewsMode {
		page.Content = extractor.ExtractMainContent(sel)
	}

	// Always extract structured data (JSON-LD and DOM breadcrumbs)
	sd := extractor.ExtractJSONLD(sel)
	if sd != nil {
		page.StructuredData = sd
		if len(sd.Breadcrumbs) > 0 {
			page.Breadcrumbs = sd.Breadcrumbs
		}
		if sd.Category != "" {
			page.Category = sd.Category
		}
		if (sd.Name != "" || sd.Price > 0 || sd.SKU != "") && page.Product == nil {
			page.Product = sd.ToProductInfo()
			if page.PageType == "" {
				page.PageType = "product"
			}
		}
	}

	if len(page.Breadcrumbs) == 0 {
		page.Breadcrumbs = extractor.ExtractDOMBreadcrumbs(sel)
	}

	if c.shouldExtractProduct(pageURL) {
		c.logVerbose("extracting product data for: %s", pageURL)
		opts := extractor.ProductOptions{
			ExtractSpecs:  c.config.ExtractSpecs,
			ExtractImages: c.config.ExtractImages,
		}
		productData := extractor.ExtractProduct(sel, pageURL, opts)

		if productData != nil && productData.Product.Name != "" {
			page.Product = productData.Product
			if page.PageType == "" {
				page.PageType = "product"
			}
			c.logVerbose("extracted product '%s'", productData.Product.Name)
		} else {
			c.logVerbose("no product found on: %s", pageURL)
		}
	}

	if c.shouldExtractNews(pageURL) {
		newsData := extractor.ExtractNews(sel, pageURL)
		if newsData != nil {
			page.News = newsData
			if page.Author == "" {
				page.Author = newsData.Author
			}
			if page.PublishedDate == "" {
				page.PublishedDate = newsData.PublishedDate
			}
			if page.Description == "" {
				page.Description = newsData.Summary
			}
			if page.PageType == "" {
				page.PageType = "article"
			}
		}
	}

	return page
}

// extractLinksFromDOM finds all valid <a href> links in a DOM selection and enqueues them.
func (c *Crawler) extractLinksFromDOM(sel *goquery.Selection, sourceURL string, depth int) []string {
	var discovered []string

	sel.Find("a[href]").Each(func(_ int, s *goquery.Selection) {
		href, ok := s.Attr("href")
		if !ok || href == "" || strings.HasPrefix(href, "javascript:") || href == "#" {
			return
		}

		absURL := resolveURL(sourceURL, href)
		if absURL == "" {
			return
		}

		normalizedURL, skipReason := c.normalizeAndFilterURL(absURL)
		if normalizedURL == "" {
			c.logVerbose("skip %s: %s", absURL, skipReason)
			return
		}

		if c.config.RobotsValidator != nil && !c.config.RobotsValidator(normalizedURL) {
			c.logVerbose("skip %s: disallowed by robots.txt", normalizedURL)
			return
		}

		discovered = append(discovered, normalizedURL)

		newDepth := depth + 1
		if newDepth > c.config.MaxDepth {
			c.logVerbose("depth exceeded for %s (%d > %d)", normalizedURL, newDepth, c.config.MaxDepth)
			return
		}

		link := models.DiscoveredLink{
			URL:        normalizedURL,
			Depth:      newDepth,
			Source:     "page",
			SourceURL:  sourceURL,
			AnchorText: strings.TrimSpace(s.Text()),
		}
		c.queue.Push(link)
		c.logVerbose("queued: %s (depth %d)", normalizedURL, newDepth)

		if c.onLinkFound != nil {
			c.onLinkFound(link)
		}
		if c.onLinkDiscovered != nil {
			c.onLinkDiscovered(sourceURL, normalizedURL)
		}
	})

	return discovered
}

// enqueueDiscoveredLinks enqueues links found via click-crawler interactions.
func (c *Crawler) enqueueDiscoveredLinks(links []string, depth int, sourceURL string) {
	newDepth := depth + 1
	if newDepth > c.config.MaxDepth {
		return
	}

	for _, rawLink := range links {
		normalizedURL, skipReason := c.normalizeAndFilterURL(rawLink)
		if normalizedURL == "" {
			c.logVerbose("skip click link %s: %s", rawLink, skipReason)
			continue
		}

		if c.config.RobotsValidator != nil && !c.config.RobotsValidator(normalizedURL) {
			c.logVerbose("skip click link %s: disallowed by robots.txt", normalizedURL)
			continue
		}

		link := models.DiscoveredLink{
			URL:       normalizedURL,
			Depth:     newDepth,
			Source:    "click",
			SourceURL: sourceURL,
		}
		c.queue.Push(link)
		c.logVerbose("queued from click: %s (depth %d)", normalizedURL, newDepth)

		if c.onLinkFound != nil {
			c.onLinkFound(link)
		}
		if c.onLinkDiscovered != nil {
			c.onLinkDiscovered(sourceURL, normalizedURL)
		}
	}
}

// resolveURL resolves a relative href against a base URL.
func resolveURL(base, href string) string {
	baseURL, err := url.Parse(base)
	if err != nil {
		return ""
	}
	hrefURL, err := url.Parse(href)
	if err != nil {
		return ""
	}
	return baseURL.ResolveReference(hrefURL).String()
}

// handleLink is called for each <a href> on a page during standard Colly traversal.
func (c *Crawler) handleLink(e *colly.HTMLElement) {
	select {
	case <-c.ctx.Done():
		return
	default:
	}

	href := e.Attr("href")
	if href == "" {
		return
	}

	absoluteURL := e.Request.AbsoluteURL(href)
	if absoluteURL == "" {
		return
	}

	normalizedURL, skipReason := c.normalizeAndFilterURL(absoluteURL)
	if normalizedURL == "" {
		c.logVerbose("skip %s: %s", absoluteURL, skipReason)
		return
	}

	if c.config.RobotsValidator != nil && !c.config.RobotsValidator(normalizedURL) {
		c.logVerbose("skip %s: disallowed by robots.txt", normalizedURL)
		return
	}

	reqDepth := e.Request.Depth
	if dStr := e.Request.Ctx.Get("depth"); dStr != "" {
		if d, err := strconv.Atoi(dStr); err == nil {
			reqDepth = d
		}
	}

	newDepth := reqDepth + 1
	if newDepth > c.config.MaxDepth {
		c.logVerbose("depth exceeded for %s (%d > %d)", normalizedURL, newDepth, c.config.MaxDepth)
		return
	}

	link := models.DiscoveredLink{
		URL:        normalizedURL,
		Depth:      newDepth,
		Source:     "page",
		SourceURL:  e.Request.URL.String(),
		AnchorText: strings.TrimSpace(e.Text),
	}
	c.queue.Push(link)
	c.logVerbose("queued: %s (depth %d)", normalizedURL, newDepth)

	if c.onLinkFound != nil {
		c.onLinkFound(link)
	}
	if c.onLinkDiscovered != nil {
		c.onLinkDiscovered(e.Request.URL.String(), normalizedURL)
	}
}

// handleError is called when a request fails.
func (c *Crawler) handleError(r *colly.Response, err error) {
	urlStr := ""
	if r != nil && r.Request != nil {
		urlStr = r.Request.URL.String()
	}
	c.logVerbose("error %s: %v", urlStr, err)

	if c.onError != nil {
		c.onError(urlStr, err)
	}
}

// shouldExtractProduct decides whether product extraction applies to a URL.
func (c *Crawler) shouldExtractProduct(pageURL string) bool {
	if !c.config.ProductMode {
		return false
	}
	if c.config.ProductPattern == nil {
		return true
	}
	return c.config.ProductPattern.MatchString(pageURL)
}

func (c *Crawler) shouldExtractNews(pageURL string) bool {
	if !c.config.NewsMode {
		return false
	}
	if c.config.NewsPattern == nil {
		return true
	}
	return c.config.NewsPattern.MatchString(pageURL)
}
