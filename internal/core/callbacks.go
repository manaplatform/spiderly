// internal/core/callbacks.go
package core

import (
	"context"
	"net"
	"strings"
	"time"
	"spiderly/internal/chunker"
	"spiderly/internal/models"
	"fmt"
)

// classifyError maps a raw error to an ErrorCategory.
func classifyError(err error) ErrorCategory {
	if err == nil {
		return ErrorCategoryUnknown
	}

	if err == context.DeadlineExceeded || err == context.Canceled {
		return ErrorCategoryTimeout
	}

	msg := err.Error()

	if _, ok := err.(net.Error); ok {
		var netErr net.Error
		if ok && netErr.Timeout() {
			return ErrorCategoryTimeout
		}
		return ErrorCategoryNetwork
	}

	if strings.Contains(msg, "timeout") || strings.Contains(msg, "deadline") {
		return ErrorCategoryTimeout
	}
	if strings.Contains(msg, "connection refused") || strings.Contains(msg, "no such host") ||
		strings.Contains(msg, "network") || strings.Contains(msg, "dns") {
		return ErrorCategoryNetwork
	}
	if strings.Contains(msg, "robots") {
		return ErrorCategoryRobots
	}
	if strings.Contains(msg, "parse") || strings.Contains(msg, "unmarshal") || strings.Contains(msg, "decode") {
		return ErrorCategoryParse
	}

	return ErrorCategoryUnknown
}

func (c *Core) setupCrawlerCallbacks() {
	if c.crawler == nil {
		return
	}

	// ── Page Scraped ──
	c.crawler.OnPageScraped(func(page models.ScrapedPage) {
		c.mu.Lock()
		c.stats.PagesScraped++
		c.mu.Unlock()

		// Stream to sinks
		c.flushToSinks(page)

		// Collect metrics
		c.metrics.RecordPage(
			page.StatusCode,
			page.ContentType,
			page.ContentLength,
			time.Duration(page.LoadTimeMs)*time.Millisecond,
			page.LinksCount,
		)

		// Log to console
		c.logger.PageScraped(page.URL, page.Title, page.StatusCode, page.LoadTimeMs)
	})

	// ── Error ──
	c.crawler.OnError(func(pageURL string, err error) {
		c.mu.Lock()
		c.stats.Errors++
		c.mu.Unlock()

		category := classifyError(err)
		crawlErr := c.recordError(pageURL, err, category)

		c.logger.PageError(pageURL, crawlErr)
		c.metrics.RecordError(category)
	})
	// ── Link Discovered ──
	c.crawler.OnLinkDiscovered(func(from, to string) {
		normalized := NormalizeURL(to)
		if normalized == "" {
			c.logger.Verbose("Skipping malformed discovered URL: %s", to)
			return
		}

		if c.robots != nil {
			allowed, err := c.robots.IsAllowed(context.Background(), normalized)
			if err != nil {
				c.logger.Verbose("Robots check failed for %s: %v", normalized, err)
			}
			if !allowed {
				c.logger.Verbose("Blocked by robots.txt: %s", normalized)
				c.metrics.IncrementRobotsBlocked()
				return
			}
		}

		// Dedup via normalizer (Core has no `seen` field)
		if c.normalizer != nil && c.normalizer.IsSeen(normalized) {
			return
		}
		if c.normalizer != nil {
			c.normalizer.MarkSeen(normalized)
		}

		c.metrics.IncrementLinksDiscovered()
		c.logger.LinkDiscovered(normalized, 0)
	})



}

func (c *Core) setupChunkerCallbacks() {
	if c.chunker == nil {
		return
	}

	c.chunker.OnPageScraped(func(page models.ScrapedPage, chunkID int) {
		c.mu.Lock()
		c.stats.PagesScraped++
		c.mu.Unlock()

		// Stream to sinks
		c.flushToSinks(page)

		c.metrics.RecordPage(
			page.StatusCode,
			page.ContentType,
			page.ContentLength,
			time.Duration(page.LoadTimeMs)*time.Millisecond,
			page.LinksCount,
		)
		c.logger.PageScraped(page.URL, page.Title, page.StatusCode, page.LoadTimeMs)
	})

	c.chunker.OnError(func(workerErr chunker.WorkerError) {
		c.mu.Lock()
		c.stats.Errors++
		c.mu.Unlock()

		err := fmt.Errorf("%s", workerErr.Error)
		category := classifyError(err)
		crawlErr := c.recordError(workerErr.URL, err, category)

		c.logger.PageError(workerErr.URL, crawlErr)
		c.metrics.RecordError(category)
	})

}

func (c *Core) recordError(pageURL string, err error, category ErrorCategory) *CrawlError {
	kind := kindFromCategory(category)
	crawlErr := NewCrawlError(kind, pageURL, err)

	c.mu.Lock()
	c.errors = append(c.errors, crawlErr)
	c.mu.Unlock()

	return crawlErr
}

// kindFromCategory maps an ErrorCategory back to an ErrKind for the constructor.
func kindFromCategory(cat ErrorCategory) ErrKind {
	switch cat {
	case ErrorCategoryNetwork:
		return ErrKindNetwork
	case ErrorCategoryParse:
		return ErrKindParse
	case ErrorCategoryRobots:
		return ErrKindRobots
	case ErrorCategoryTimeout:
		return ErrKindTimeout
	case ErrorCategorySink:
		return ErrKindSink
	case ErrorCategoryConfig:
		return ErrKindConfig
	default:
		return ErrKindUnknown
	}
}
