package core

import (
	"context"
	"fmt"
	"net/url"
	"strings"
	"time"

	"spiderly/internal/chunker"
	"spiderly/internal/crawler"
	"spiderly/internal/models"
)

// validateTargetURL ensures we have a valid base URL for crawling.
func validateTargetURL(targetURL, sitemapURL string) (string, error) {
	if targetURL == "" {
		if sitemapURL == "" {
			return "", fmt.Errorf("no target URL or sitemap URL specified")
		}
		parsed, err := url.Parse(sitemapURL)
		if err != nil {
			return "", fmt.Errorf("invalid sitemap URL: %v", err)
		}
		if parsed.Scheme == "" || parsed.Host == "" {
			return "", fmt.Errorf("sitemap URL must be absolute (include scheme and host)")
		}
		return fmt.Sprintf("%s://%s", parsed.Scheme, parsed.Host), nil
	}
	normalized := targetURL
	if !strings.HasPrefix(normalized, "http://") && !strings.HasPrefix(normalized, "https://") {
		normalized = "https://" + normalized
	}
	parsed, err := url.Parse(normalized)
	if err != nil {
		return "", fmt.Errorf("invalid URL: %v", err)
	}
	if parsed.Host == "" {
		return "", fmt.Errorf("URL must have a valid host")
	}
	return normalized, nil
}

// ─────────────────────────────────────────────
//  Run — Main Pipeline Entry Point
// ─────────────────────────────────────────────

func (c *Core) Run() ([]models.ScrapedPage, error) {
	c.startTime = time.Now()

	// ── Step 1: Validate target URL ──
	targetURL, err := validateTargetURL(c.config.TargetURL, c.config.SitemapURL)
	if err != nil {
		return nil, NewCrawlError(ErrKindConfig, "", err).
			WithMessage("invalid target URL")
	}

	// ── Step 2: Initialize robots.txt checker ──
	robotsChecker := NewRobotsChecker(RobotsConfig{
		Enabled: true,
		Timeout: c.config.Timeout,
	})
	c.logger.Verbose("robots.txt checker initialized")
	c.robots = robotsChecker

	// ── Step 3: Initialize URL normalizer + dedup set ──
	c.normalizer = NewURLNormalizer()
	c.seen = NewURLDedup()

	// ── Step 4: Initialize streaming sink if configured ──
	if c.sink != nil {
		if err := c.sink.Open(); err != nil {
			return nil, NewCrawlError(ErrKindSink, "", err).
				WithMessage("failed to open output sink")
		}
		defer func() {
			if closeErr := c.sink.Close(); closeErr != nil {
				c.logger.Error("Failed to close sink: %v", closeErr)
			}
		}()
	}

	// ── Step 5: Show header (non-chunked mode) ──
	if !c.config.EnableChunker {
		c.logger.Header()
		c.logger.Phase("init", fmt.Sprintf("Target: %s", targetURL))
		c.logger.Info("Max pages: %d | Concurrency: %d | Timeout: %s",
			c.config.MaxPages, c.config.Concurrency, c.config.Timeout)

		if robotsChecker != nil {
			ctx := context.Background()
			allowed, _ := robotsChecker.IsAllowed(ctx, targetURL)
			crawlDelay := robotsChecker.CrawlDelay(ctx, targetURL)
			c.logger.Info("robots.txt: %s | Crawl-delay: %v",
				colorBool(allowed), crawlDelay)
		}
	}

	// ── Step 6: Determine crawl strategy ──
	result, err := c.determineCrawlStrategy(targetURL)

	// ── Step 7: Execute crawl ──
	var results []models.ScrapedPage

	if err != nil {
		return nil, NewCrawlError(ErrKindConfig, targetURL, err).
			WithMessage("failed to determine crawl strategy")
	}

	// Access the strategy field on the struct:
	if result.Strategy == StrategySitemap {
		if c.config.EnableChunker {
			results, err = c.executeChunkedSitemapCrawl(targetURL, result.Entries)
		} else {
			results, err = c.executeSitemapCrawl(targetURL, result.Entries)
		}
	} else {
		results, err = c.executeRecursiveCrawl(targetURL)
	}

	if err != nil {
		return results, err
	}

	return results, nil
}

// ─────────────────────────────────────────────
//  Sitemap Crawl Execution
// ─────────────────────────────────────────────

func (c *Core) executeSitemapCrawl(baseURL string, entries []models.SitemapEntry) ([]models.ScrapedPage, error) {
	entries = c.preprocessEntries(entries)

	c.logger.Phase("crawling", fmt.Sprintf("Sitemap mode: %d URLs to process", len(entries)))

	if len(entries) == 0 {
		return nil, NewCrawlError(ErrKindConfig, baseURL, fmt.Errorf("no URLs remaining after filtering")).
			WithMessage("no URLs remaining after filtering")
	}

	urls := make([]string, 0, len(entries))
	for _, entry := range entries {
		urls = append(urls, entry.URL)
	}

	if c.config.MaxPages > 0 && len(urls) > c.config.MaxPages {
		urls = urls[:c.config.MaxPages]
		c.logger.Info("Limited to %d pages (max pages setting)", c.config.MaxPages)
	}

	crwl, err := crawler.NewCrawler(crawler.Config{
		MaxPages:       c.config.MaxPages,
		MaxDepth:       1,
		Concurrency:    c.config.Concurrency,
		Delay:          c.effectiveDelay(),
		Timeout:        c.config.Timeout,
		SitemapMode:    true,
		ProductMode:    c.config.ProductMode,
		ProductPattern: c.config.CompiledProductPattern,
		ExtractSpecs:   c.config.ExtractSpecs,
		ExtractImages:  c.config.ExtractImages,
	})
	if err != nil {
		return nil, NewCrawlError(ErrKindConfig, baseURL, err).
			WithMessage("failed to create crawler")
	}
	c.crawler = crwl


	c.setupCrawlerCallbacks()

	for _, u := range urls {
		c.crawler.QueueURL(u, 0)
	}

	results, err := c.crawler.Crawl(baseURL)
	if err != nil {
		return nil, NewCrawlError(ErrKindNetwork, baseURL, err).
			WithMessage("sitemap crawl failed")
	}

	c.mu.Lock()
	c.results = results
	c.mu.Unlock()


	return results, nil
}

// ─────────────────────────────────────────────
//  Recursive Crawl Execution
// ─────────────────────────────────────────────

func (c *Core) executeRecursiveCrawl(targetURL string) ([]models.ScrapedPage, error) {
	c.logger.Phase("crawling", fmt.Sprintf("Recursive mode from %s", targetURL))

	if c.robots != nil {
		ctx := context.Background()
		allowed, _ := c.robots.IsAllowed(ctx, targetURL)
		if !allowed {
			c.logger.Warning("Seed URL blocked by robots.txt: %s", targetURL)
			return nil, NewCrawlError(ErrKindRobots, targetURL, fmt.Errorf("seed URL is disallowed by robots.txt")).
				WithMessage("seed URL is disallowed by robots.txt")
		}
	}

	crwl, err := crawler.NewCrawler(crawler.Config{
		MaxPages:       c.config.MaxPages,
		MaxDepth:       c.config.MaxDepth,
		Concurrency:    c.config.Concurrency,
		Delay:          c.effectiveDelay(),
		Timeout:        c.config.Timeout,
		SitemapMode:    false,
		ProductMode:    c.config.ProductMode,
		ProductPattern: c.config.CompiledProductPattern,
		ExtractSpecs:   c.config.ExtractSpecs,
		ExtractImages:  c.config.ExtractImages,
	})
	if err != nil {
		return nil, NewCrawlError(ErrKindConfig, targetURL, err).
			WithMessage("failed to create crawler")
	}
	c.crawler = crwl


	c.setupCrawlerCallbacks()

	results, err := c.crawler.Crawl(targetURL)
	if err != nil {
		return nil, NewCrawlError(ErrKindNetwork, targetURL, err).
			WithMessage("recursive crawl failed")
	}

	c.mu.Lock()
	c.results = results
	c.mu.Unlock()


	return results, nil
}

// ─────────────────────────────────────────────
//  Chunked Sitemap Crawl
// ─────────────────────────────────────────────

func (c *Core) executeChunkedSitemapCrawl(baseURL string, entries []models.SitemapEntry) ([]models.ScrapedPage, error) {
	entries = c.preprocessEntries(entries)

	if c.config.MaxPages > 0 && len(entries) > c.config.MaxPages {
		entries = entries[:c.config.MaxPages]
	}

	if len(entries) == 0 {
		return nil, NewCrawlError(ErrKindConfig, baseURL, fmt.Errorf("no URLs remaining after filtering for chunked crawl")).
			WithMessage("no URLs remaining after filtering for chunked crawl")
	}

	c.chunker = chunker.New(chunker.Config{
		ChunkSize:      c.config.ChunkSize,
		MaxWorkers:     c.config.MaxWorkers,
		Concurrency:    c.config.Concurrency,
		Delay:          c.effectiveDelay(),
		Timeout:        c.config.Timeout,
		Headless:       c.config.Headless,
		Verbose:        c.config.Verbose,
		NoColor:        c.config.NoColor,
		ProductMode:    c.config.ProductMode,
		ProductPattern: c.config.ProductPattern,
		ExtractSpecs:   c.config.ExtractSpecs,
		ExtractImages:  c.config.ExtractImages,
	})

	c.chunker.OnPageScraped(func(page models.ScrapedPage, chunkID int) {
		c.mu.Lock()
		c.metrics.IncrementPagesScraped()
		c.metrics.RecordStatus(page.StatusCode)
		c.metrics.AddBytes(page.ContentLength)
		c.mu.Unlock()

		if c.sink != nil {
			if err := c.sink.Write(page); err != nil {
				c.logger.Error("Sink write error: %v", err)
			}
		}
	})

	c.chunker.OnError(func(err chunker.WorkerError) {
		c.mu.Lock()
		c.metrics.IncrementErrors()
		c.mu.Unlock()
	})

	c.chunker.SplitEntries(entries)

	results, err := c.chunker.Process(baseURL)
	if err != nil {
		return nil, NewCrawlError(ErrKindNetwork, baseURL, err).
			WithMessage("chunked crawl failed")
	}

	c.mu.Lock()
	c.results = results
	c.mu.Unlock()

	return results, nil
}

// ─────────────────────────────────────────────
//  Preprocessing Pipeline
// ─────────────────────────────────────────────

func (c *Core) preprocessEntries(entries []models.SitemapEntry) []models.SitemapEntry {
	if len(entries) == 0 {
		return entries
	}

	originalCount := len(entries)
	result := make([]models.SitemapEntry, 0, len(entries))

	var (
		dupCount    int
		robotsCount int
	)

	for _, entry := range entries {
		// ── Normalize URL ──
		normalized := entry.URL
		if c.normalizer != nil {
			norm, err := c.normalizer.Normalize(entry.URL)
			if err != nil || norm == "" {
				continue
			}
			normalized = norm
		}

		// ── Dedup ──
		if c.seen != nil && !c.seen.Add(normalized) {
			dupCount++
			continue
		}

		// ── robots.txt check ──
		if c.robots != nil {
			ctx := context.Background()
			allowed, _ := c.robots.IsAllowed(ctx, normalized)
			if !allowed {
				robotsCount++
				c.logger.Verbose("Blocked by robots.txt: %s", normalized)
				continue
			}
		}

		entry.URL = normalized
		result = append(result, entry)
	}

	if dupCount > 0 || robotsCount > 0 {
		c.logger.Info("Preprocessing: %d → %d URLs (dedup: -%d, robots: -%d)",
			originalCount, len(result), dupCount, robotsCount)
	}

	return result
}

// ─────────────────────────────────────────────
//  Helpers
// ─────────────────────────────────────────────

func (c *Core) effectiveDelay() time.Duration {
	delay := c.config.Delay

	if c.robots != nil {
		ctx := context.Background()
		// Extract origin from the target URL for CrawlDelay lookup
		origin := c.config.TargetURL
		if parsed, err := url.Parse(origin); err == nil && parsed.Host != "" {
			origin = fmt.Sprintf("%s://%s", parsed.Scheme, parsed.Host)
		}
		robotsDelay := c.robots.CrawlDelay(ctx, origin)
		if robotsDelay > delay {
			c.logger.Verbose("Using robots.txt crawl-delay: %v (overrides configured %v)", robotsDelay, delay)
			delay = robotsDelay
		}
	}

	return delay
}

func colorBool(allowed bool) string {
	if allowed {
		return "allowed"
	}
	return "blocked"
}
