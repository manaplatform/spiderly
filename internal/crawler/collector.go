// internal/crawler/collector.go
package crawler

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/gocolly/colly/v2"
	collyproxy "github.com/gocolly/colly/v2/proxy"
)

// setupCollector initialises the colly collector bound to the start URL's domain.
func (c *Crawler) setupCollector(startURL string) error {
	parsed, err := url.Parse(startURL)
	if err != nil {
		return fmt.Errorf("invalid start URL: %w", err)
	}

	if parsed.Host == "" {
		return fmt.Errorf("start URL has no host: %s", startURL)
	}

	hostname := parsed.Hostname()
	if hostname == "" {
		hostname = parsed.Host
	}

	allowedDomains := []string{hostname}
	if strings.HasPrefix(hostname, "www.") {
		allowedDomains = append(allowedDomains, strings.TrimPrefix(hostname, "www."))
	} else {
		allowedDomains = append(allowedDomains, "www."+hostname)
	}

	opts := []colly.CollectorOption{
		colly.Async(true),
		colly.MaxDepth(c.config.MaxDepth),
		colly.UserAgent(c.config.UserAgent),
		colly.AllowedDomains(allowedDomains...),
	}

	// Optional: let colly honour robots.txt
	if !c.config.RespectRobots {
		opts = append(opts, colly.IgnoreRobotsTxt())
	}

	c.collector = colly.NewCollector(opts...)

	if len(c.config.Proxies) == 1 {
		if err := c.collector.SetProxy(c.config.Proxies[0]); err != nil {
			return fmt.Errorf("failed to set proxy: %w", err)
		}
	} else if len(c.config.Proxies) > 1 {
		switcher, err := collyproxy.RoundRobinProxySwitcher(c.config.Proxies...)
		if err != nil {
			return fmt.Errorf("failed to initialize proxy switcher: %w", err)
		}
		c.collector.SetProxyFunc(switcher)
	}

	// Rate limiting
	if err := c.collector.Limit(&colly.LimitRule{
		DomainGlob:  "*",
		Parallelism: c.config.Concurrency,
		Delay:       c.config.Delay,
	}); err != nil {
		return fmt.Errorf("failed to set limit rule: %w", err)
	}

	c.collector.SetRequestTimeout(c.config.Timeout)

	// Wire up handlers (defined in handlers.go)
	c.registerHandlers()

	return nil
}

// registerHandlers attaches all colly event handlers.
func (c *Crawler) registerHandlers() {
	c.collector.OnHTML("html", c.handlePage)

	// In sitemap mode we don't follow links — the queue is pre-filled.
	if !c.config.SitemapMode {
		c.collector.OnHTML("a[href]", c.handleLink)
	}

	c.collector.OnError(c.handleError)

	c.collector.OnRequest(func(r *colly.Request) {
		// Abort early if we already hit the page limit
		if c.config.MaxPages > 0 && int(c.scraped.Load()) >= c.config.MaxPages {
			r.Abort()
			return
		}

		c.logVerbose("visiting %s", r.URL.String())
	})

	c.collector.OnResponse(func(r *colly.Response) {
		c.logVerbose("response %d from %s (%d bytes)",
			r.StatusCode, r.Request.URL.String(), len(r.Body))
	})
}
