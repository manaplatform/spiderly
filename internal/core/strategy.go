package core

import (
	"context"
	"fmt"
	"strings"

	"spiderly/internal/models"
	"spiderly/internal/sitemap"
)

// ─────────────────────────────────────────────
//  Crawl Strategy
// ─────────────────────────────────────────────

// CrawlStrategy represents the chosen crawl approach.
type CrawlStrategy string

const (
	StrategySitemap   CrawlStrategy = "sitemap"
	StrategyRecursive CrawlStrategy = "recursive"
)

// strategyResult bundles the resolved strategy with its data.
type strategyResult struct {
	Strategy CrawlStrategy
	Entries  []models.SitemapEntry
}

// determineCrawlStrategy inspects config, discovers sitemaps, and picks the
// best crawl approach.  It returns a strategyResult or falls back to
// recursive crawling when no usable sitemap data is found.
func (c *Core) determineCrawlStrategy(targetURL string) (strategyResult, error) {
	// ── Force recursive ──
	if c.config.ForceRecursive {
		c.logger.Info("Forced recursive mode — skipping sitemap discovery")
		return strategyResult{Strategy: StrategyRecursive}, nil
	}

	// ── Direct sitemap URL provided ──
	if c.config.SitemapURL != "" {
		return c.strategyFromDirectSitemap(targetURL)
	}

	// ── Auto-discovery ──
	return c.strategyFromDiscovery(targetURL)
}

// ─────────────────────────────────────────────
//  Direct Sitemap
// ─────────────────────────────────────────────

func (c *Core) strategyFromDirectSitemap(targetURL string) (strategyResult, error) {
	c.logger.Verbose("Direct sitemap URL provided: %s", c.config.SitemapURL)

	entries, err := c.fetchSitemapEntries(c.config.SitemapURL)
	if err != nil {
		c.logger.Warning("Failed to fetch provided sitemap: %v", err)
		return strategyResult{Strategy: StrategyRecursive},
			NewCrawlError(ErrKindNetwork, c.config.SitemapURL, err).WithMessage("fetch sitemap")
	}

	if len(entries) == 0 {
		c.logger.Warning("Provided sitemap was empty — falling back to recursive")
		return strategyResult{Strategy: StrategyRecursive},
			NewCrawlError(ErrKindConfig, c.config.SitemapURL, fmt.Errorf("0 entries")).WithMessage("empty sitemap")
	}

	filtered := c.applyAllFilters(entries)

	if len(filtered) == 0 {
		c.logger.Warning("All entries filtered out — using unfiltered set")
		filtered = entries
	}

	return strategyResult{
		Strategy: StrategySitemap,
		Entries:  filtered,
	}, nil
}

// ─────────────────────────────────────────────
//  Auto-Discovery
// ─────────────────────────────────────────────

func (c *Core) strategyFromDiscovery(targetURL string) (strategyResult, error) {
	c.logger.Phase("discovery", "Searching for sitemaps...")

	sitemapURLs, err := c.discoverSitemapURLs(targetURL)
	if err != nil {
		c.logger.Verbose("Sitemap discovery error: %v", err)
	}

	if len(sitemapURLs) == 0 {
		c.logger.Warning("No sitemaps found — falling back to recursive crawl")
		return strategyResult{Strategy: StrategyRecursive}, nil
	}

	c.logger.Success("Found %d sitemap(s)", len(sitemapURLs))

	// Parse every discovered sitemap
	allEntries := c.parseSitemapURLs(sitemapURLs)

	if len(allEntries) == 0 {
		c.logger.Warning("All sitemaps were empty — falling back to recursive crawl")
		return strategyResult{Strategy: StrategyRecursive}, nil
	}

	// Normalize + deduplicate before filtering
	allEntries = c.deduplicateEntries(allEntries)

	filtered := c.applyAllFilters(allEntries)

	c.logger.SitemapStats(len(allEntries), len(filtered), len(sitemapURLs))

	if len(filtered) == 0 {
		c.logger.Warning("All entries filtered out — using unfiltered set")
		filtered = allEntries
	}

	return strategyResult{
		Strategy: StrategySitemap,
		Entries:  filtered,
	}, nil
}

// ─────────────────────────────────────────────
//  Sitemap Discovery Helpers
// ─────────────────────────────────────────────

// newSitemapParser creates a sitemap.Parser configured from Core's config.
func (c *Core) newSitemapParser() *sitemap.Parser {
	return sitemap.NewParser(
		sitemap.WithTimeout(c.config.Timeout),
		sitemap.WithVerbose(c.config.Verbose),
	)
}

// discoverSitemapURLs returns sitemap URLs, optionally filtered for product
// mode.  It checks robots.txt first when a RobotsChecker is available.
func (c *Core) discoverSitemapURLs(targetURL string) ([]string, error) {
	parser := c.newSitemapParser()
	ctx := context.Background()

	// In product mode, try product-specific sitemaps first
	if c.config.ProductMode {
		c.logger.Info("Product mode enabled — prioritizing product sitemaps")

		filters := []string{"pdp", "product"}
		if len(c.config.ProductSitemaps) > 0 {
			filters = c.config.ProductSitemaps
		}

		// Build a filter func that matches any of the keywords
		filterFn := func(sitemapURL string) bool {
			lower := strings.ToLower(sitemapURL)
			for _, kw := range filters {
				if strings.Contains(lower, strings.ToLower(kw)) {
					return true
				}
			}
			return false
		}

		urls, err := parser.DiscoverSitemapsFiltered(ctx, targetURL, filterFn)
		if err == nil && len(urls) > 0 {
			c.logger.Success("Found %d product sitemap(s)", len(urls))
			return urls, nil
		}

		c.logger.Warning("No product-specific sitemaps — trying all sitemaps")
	}

	return parser.DiscoverSitemaps(ctx, targetURL)
}

// parseSitemapURLs fetches and parses each sitemap URL, collecting all entries.
func (c *Core) parseSitemapURLs(sitemapURLs []string) []models.SitemapEntry {
	var all []models.SitemapEntry

	for _, sURL := range sitemapURLs {
		c.logger.Verbose("Parsing: %s", sURL)

		entries, err := c.fetchSitemapEntries(sURL)
		if err != nil {
			c.logger.Verbose("Failed to parse sitemap %s: %v", sURL, err)
			continue
		}

		all = append(all, entries...)
	}

	return all
}

// fetchSitemapEntries wraps the sitemap parser.
func (c *Core) fetchSitemapEntries(sitemapURL string) ([]models.SitemapEntry, error) {
	parser := c.newSitemapParser()
	ctx := context.Background()
	result, err := parser.Parse(ctx, sitemapURL)
	if err != nil {
		return nil, err
	}

	entries := make([]models.SitemapEntry, 0, len(result.URLs))
	for _, u := range result.URLs {
		entries = append(entries, models.SitemapEntry{
			URL:        u.Loc,
			LastMod:    u.LastMod,
			ChangeFreq: u.ChangeFreq,
			Priority:   float64(u.Priority),
		})
	}
	return entries, nil
}


// ─────────────────────────────────────────────
//  Deduplication
// ─────────────────────────────────────────────

// deduplicateEntries normalizes every entry URL and removes duplicates,
// keeping the first occurrence.
func (c *Core) deduplicateEntries(entries []models.SitemapEntry) []models.SitemapEntry {
	seen := make(map[string]struct{}, len(entries))
	deduped := make([]models.SitemapEntry, 0, len(entries))

	for _, entry := range entries {
		normalized := NormalizeURL(entry.URL)
		if normalized == "" {
			continue
		}

		if _, exists := seen[normalized]; exists {
			continue
		}

		seen[normalized] = struct{}{}
		entry.URL = normalized
		deduped = append(deduped, entry)
	}

	if removed := len(entries) - len(deduped); removed > 0 {
		c.logger.Verbose("Deduplication removed %d duplicate URLs", removed)
	}

	return deduped
}

// ─────────────────────────────────────────────
//  Combined Filtering Pipeline
// ─────────────────────────────────────────────

// applyAllFilters runs the standard sitemap filter followed by the product
// filter (when product mode is active).  It also checks robots.txt when a
// checker is configured.
func (c *Core) applyAllFilters(entries []models.SitemapEntry) []models.SitemapEntry {
	// 1. Standard filters (priority, URL pattern)
	filtered, _ := c.filterSitemapEntries(entries)

	// 2. Product mode filter
	if c.config.ProductMode {
		filtered, _ = c.filterProductEntries(filtered)
	}

	// 3. robots.txt filter
	if c.robots != nil {
		filtered = c.filterByRobots(filtered)
	}

	return filtered
}

// filterByRobots removes entries disallowed by robots.txt.
func (c *Core) filterByRobots(entries []models.SitemapEntry) []models.SitemapEntry {
	if c.robots == nil {
		return entries
	}

	allowed := make([]models.SitemapEntry, 0, len(entries))
	blocked := 0

	ctx := context.Background()

	for _, entry := range entries {
		ok, err := c.robots.IsAllowed(ctx, entry.URL)
		if err != nil {
			c.logger.Verbose("robots.txt check error for %s: %v", entry.URL, err)
			// fail open — keep the entry
			allowed = append(allowed, entry)
			continue
		}
		if ok {
			allowed = append(allowed, entry)
		} else {
			blocked++
			c.logger.Verbose("Blocked by robots.txt: %s", entry.URL)
		}
	}

	if blocked > 0 {
		c.logger.Info("robots.txt blocked %d URLs", blocked)
	}

	return allowed
}
