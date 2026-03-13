package core

import (
	"regexp"
	"strings"

	"spiderly/internal/exclude"
	"spiderly/internal/models"
)

// ─────────────────────────────────────────────
//  Sitemap Entry Filtering
// ─────────────────────────────────────────────

// FilterResult holds filtering statistics for observability.
type FilterResult struct {
	InputCount          int
	OutputCount         int
	DroppedByPriority   int
	DroppedByURLPattern int
	DroppedByExclude    int
	DroppedByProduct    int
	DroppedByDuplicate  int
}

// filterSitemapEntries applies priority and URL-pattern filters to raw
// sitemap entries. It returns the surviving entries plus a FilterResult
// so callers can log what was dropped and why.
func (c *Core) filterSitemapEntries(entries []models.SitemapEntry) ([]models.SitemapEntry, FilterResult) {
	fr := FilterResult{InputCount: len(entries)}

	// Compile URL pattern once
	var urlRegex *regexp.Regexp
	if c.config.URLPattern != "" {
		var err error
		urlRegex, err = regexp.Compile(c.config.URLPattern)
		if err != nil {
			c.logger.Warning("Invalid URL pattern regex %q: %v", c.config.URLPattern, err)
			urlRegex = nil
		}
	}

	filtered := make([]models.SitemapEntry, 0, len(entries))

	for _, entry := range entries {
		// ── Priority gate ──
		if c.config.MinPriority > 0 && entry.Priority < c.config.MinPriority {
			fr.DroppedByPriority++
			continue
		}

		// ── URL pattern gate ──
		if urlRegex != nil && !urlRegex.MatchString(entry.URL) {
			fr.DroppedByURLPattern++
			continue
		}

		filtered = append(filtered, entry)
	}

	fr.OutputCount = len(filtered)
	return filtered, fr
}

// ─────────────────────────────────────────────
//  Product Entry Filtering
// ─────────────────────────────────────────────

// filterProductEntries keeps only URLs that look like product pages.
// It applies exclude-patterns first (drop non-product paths like
// /category/, /blog/, etc.) then requires a positive match against
// the configured product pattern (if any).
//
// The method is a no-op when ProductMode is disabled.
func (c *Core) filterProductEntries(entries []models.SitemapEntry) ([]models.SitemapEntry, FilterResult) {
	fr := FilterResult{InputCount: len(entries)}

	if !c.config.ProductMode {
		fr.OutputCount = len(entries)
		return entries, fr
	}

	// ── Resolve exclude regexes ──
	excludeRegexes := c.resolveExcludePatterns()

	// ── Dedup set (uses normalized URLs) ──
	seen := make(map[string]struct{}, len(entries))

	filtered := make([]models.SitemapEntry, 0, len(entries))

	for _, entry := range entries {
		// ── Normalize for dedup ──
		norm := NormalizeURL(entry.URL)
		if norm == "" {
			fr.DroppedByURLPattern++
			continue
		}
		if _, dup := seen[norm]; dup {
			fr.DroppedByDuplicate++
			c.logger.Verbose("Duplicate (normalized): %s", entry.URL)
			continue
		}
		seen[norm] = struct{}{}

		// ── Exclude patterns ──
		if matchesAny(entry.URL, excludeRegexes) {
			fr.DroppedByExclude++
			c.logger.Verbose("Excluded by pattern: %s", entry.URL)
			continue
		}

		// ── Positive product pattern ──
		if c.config.CompiledProductPattern != nil {
			if !c.config.CompiledProductPattern.MatchString(entry.URL) {
				fr.DroppedByProduct++
				c.logger.Verbose("Doesn't match product pattern: %s", entry.URL)
				continue
			}
		}

		filtered = append(filtered, entry)
	}

	fr.OutputCount = len(filtered)
	c.logger.Verbose("Product filter: %d → %d entries (excl=%d, pattern=%d, dup=%d)",
		fr.InputCount, fr.OutputCount,
		fr.DroppedByExclude, fr.DroppedByProduct, fr.DroppedByDuplicate)

	return filtered, fr
}

// ─────────────────────────────────────────────
//  Deduplication (general purpose)
// ─────────────────────────────────────────────

// DeduplicateEntries removes duplicate sitemap entries based on
// normalized URLs. It preserves the first occurrence's order.
func DeduplicateEntries(entries []models.SitemapEntry) ([]models.SitemapEntry, int) {
	seen := make(map[string]struct{}, len(entries))
	out := make([]models.SitemapEntry, 0, len(entries))
	dupes := 0

	for _, e := range entries {
		norm := NormalizeURL(e.URL)
		if norm == "" {
			continue
		}
		if _, exists := seen[norm]; exists {
			dupes++
			continue
		}
		seen[norm] = struct{}{}
		out = append(out, e)
	}

	return out, dupes
}

// DeduplicateURLs removes duplicate strings based on normalized form.
func DeduplicateURLs(urls []string) ([]string, int) {
	seen := make(map[string]struct{}, len(urls))
	out := make([]string, 0, len(urls))
	dupes := 0

	for _, u := range urls {
		norm := NormalizeURL(u)
		if norm == "" {
			continue
		}
		if _, exists := seen[norm]; exists {
			dupes++
			continue
		}
		seen[norm] = struct{}{}
		out = append(out, u)
	}

	return out, dupes
}

// ─────────────────────────────────────────────
//  Product Sitemap Heuristics
// ─────────────────────────────────────────────

// defaultProductSitemapKeywords are substrings that indicate a sitemap
// is likely to contain product URLs.
var defaultProductSitemapKeywords = []string{
	"product",
	"pdp",
	"item",
	"sku",
	"catalog",
	"shop",
	"merchandise",
}

// IsProductSitemap returns true if the sitemap URL looks like it
// contains product pages, based on keyword heuristics.
func IsProductSitemap(sitemapURL string, extraKeywords []string) bool {
	lower := strings.ToLower(sitemapURL)

	keywords := defaultProductSitemapKeywords
	if len(extraKeywords) > 0 {
		keywords = extraKeywords
	}

	for _, kw := range keywords {
		if strings.Contains(lower, kw) {
			return true
		}
	}
	return false
}

// FilterProductSitemaps returns only the sitemap URLs that match
// product-related keywords.
func FilterProductSitemaps(sitemapURLs []string, keywords []string) []string {
	var out []string
	for _, u := range sitemapURLs {
		if IsProductSitemap(u, keywords) {
			out = append(out, u)
		}
	}
	return out
}

// ─────────────────────────────────────────────
//  Internal helpers
// ─────────────────────────────────────────────

// resolveExcludePatterns returns the compiled exclude regexes to use.
// It prefers user-configured patterns; falls back to built-in defaults.
func (c *Core) resolveExcludePatterns() []*regexp.Regexp {
	if len(c.config.CompiledExcludePatterns) > 0 {
		return c.config.CompiledExcludePatterns
	}

	compiled, err := exclude.CompilePatterns(exclude.DefaultPatterns)
	if err != nil {
		c.logger.Warning("Failed to compile default exclude patterns: %v", err)
		return nil
	}
	return compiled
}

// matchesAny returns true if the URL matches any of the regexes.
func matchesAny(url string, patterns []*regexp.Regexp) bool {
	for _, re := range patterns {
		if re.MatchString(url) {
			return true
		}
	}
	return false
}