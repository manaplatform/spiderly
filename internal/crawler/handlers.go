// internal/crawler/handlers.go
package crawler

import (
	"strings"

	"spiderly/internal/extractor"
	"spiderly/internal/models"

	"github.com/gocolly/colly/v2"
)

// handlePage is called for each HTML response. It extracts page data and triggers callbacks.
func (c *Crawler) handlePage(e *colly.HTMLElement) {
	select {
	case <-c.ctx.Done():
		return
	default:
	}

	pageURL := e.Request.URL.String()
	c.logVerbose("handling page: %s", pageURL)

	// Atomically check and increment scraped count.
	// The OnRequest hook already aborts late arrivals, but this is a
	// second guard so we never store more than MaxPages results.
	if c.config.MaxPages > 0 {
		current := c.scraped.Add(1)
		if current > int64(c.config.MaxPages) {
			c.scraped.Add(-1) // roll back
			c.logVerbose("max pages (%d) reached, dropping %s", c.config.MaxPages, pageURL)
			return
		}
	} else {
		c.scraped.Add(1)
	}

	page := models.ScrapedPage{
		URL:           pageURL,
		Title:         strings.TrimSpace(e.ChildText("title")),
		StatusCode:    e.Response.StatusCode,
		ContentLength: int64(len(e.Response.Body)),
	}

	if !c.config.ProductMode || c.config.ExtractSpecs || c.config.ExtractImages || c.config.NewsMode {
		page.Content = extractor.ExtractMainContent(e.DOM)
	}

	if c.shouldExtractProduct(pageURL) {
		c.logVerbose("extracting product data for: %s", pageURL)
		opts := extractor.ProductOptions{
			ExtractSpecs:  c.config.ExtractSpecs,
			ExtractImages: c.config.ExtractImages,
		}
		productData := extractor.ExtractProduct(e.DOM, pageURL, opts)

		if productData != nil && productData.Product.Name != "" {
			page.Product = productData.Product
			c.logVerbose("extracted product '%s'", productData.Product.Name)
		} else {
			c.logVerbose("no product found on: %s", pageURL)
		}
	}

	if c.shouldExtractNews(pageURL) {
		newsData := extractor.ExtractNews(e.DOM, pageURL)
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

	c.addResult(page)
	if c.onPageScraped != nil {
		c.onPageScraped(page)
	}
}

// handleLink is called for each <a href> on a page.
// It normalizes, filters, and pushes new links into the queue.
// It never calls e.Request.Visit — drainQueue is the sole dispatcher.
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

	newDepth := e.Request.Depth + 1
	if newDepth > c.config.MaxDepth {
		c.logVerbose("depth exceeded for %s (%d > %d)", normalizedURL, newDepth, c.config.MaxDepth)
		return
	}

	// queue.Push deduplicates via its internal seen map — no extra check needed.
	link := models.DiscoveredLink{URL: normalizedURL, Depth: newDepth}
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
