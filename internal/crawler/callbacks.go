// internal/crawler/callbacks.go
package crawler

import "spiderly/internal/models"

// ---------- callback type aliases ----------

// PageScrapedFunc is invoked after a page has been successfully scraped and stored.
type PageScrapedFunc func(page models.ScrapedPage)

// LinkFoundFunc is invoked when a new link is discovered and queued.
type LinkFoundFunc func(link models.DiscoveredLink)

// LinkDiscoveredFunc is the legacy two-string variant (source → target).
type LinkDiscoveredFunc func(fromURL, toURL string)

// ErrorFunc is invoked when a request fails.
type ErrorFunc func(url string, err error)

// ---------- setter methods ----------

// OnPageScraped registers a callback fired after each page is stored.
func (c *Crawler) OnPageScraped(fn PageScrapedFunc) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.onPageScraped = fn
}

// OnLinkFound registers a callback fired when a link is pushed to the queue.
func (c *Crawler) OnLinkFound(fn LinkFoundFunc) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.onLinkFound = fn
}

// OnLinkDiscovered registers a callback fired with (source, target) URLs.
func (c *Crawler) OnLinkDiscovered(fn LinkDiscoveredFunc) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.onLinkDiscovered = fn
}

// OnError registers a callback fired when a request errors out.
func (c *Crawler) OnError(fn ErrorFunc) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.onError = fn
}
