package sitemap

import (
	"context"
	"fmt"
	"net/url"
	"strings"
	"sync"
)

// ParseResult holds the complete result of recursively parsing a sitemap.
type ParseResult struct {
	// URLs contains all leaf URLs collected across all parsed sitemaps.
	URLs []URL

	// Errors maps source sitemap URLs to the errors encountered parsing them.
	// Non-fatal: parsing continues past individual failures.
	Errors map[string]error

	// Visited tracks every sitemap URL that was fetched (for debugging/logging).
	Visited []string
}

// Parse fetches and recursively parses a sitemap URL.
// If the URL points to a sitemap index, child sitemaps are parsed concurrently
// up to the configured depth limit. Cycles are detected and skipped.
func (p *Parser) Parse(ctx context.Context, sitemapURL string) (*ParseResult, error) {
	if sitemapURL == "" {
		return nil, fmt.Errorf("empty sitemap URL")
	}

	// Normalize
	sitemapURL = strings.TrimSpace(sitemapURL)
	if _, err := url.ParseRequestURI(sitemapURL); err != nil {
		return nil, fmt.Errorf("invalid sitemap URL %q: %w", sitemapURL, err)
	}

	result := &ParseResult{
		Errors: make(map[string]error),
	}

	visited := &visitTracker{}

	p.parseRecursive(ctx, sitemapURL, 0, result, visited)

	result.Visited = visited.list()

	p.logVerbose("Parse complete: %d URLs collected, %d errors, %d sitemaps visited",
		len(result.URLs), len(result.Errors), len(result.Visited))

	return result, nil
}

// ParseAll fetches and parses multiple sitemap URLs concurrently.
// Useful after DiscoverSitemaps returns a list of candidates.
func (p *Parser) ParseAll(ctx context.Context, sitemapURLs []string) (*ParseResult, error) {
	if len(sitemapURLs) == 0 {
		return nil, fmt.Errorf("no sitemap URLs provided")
	}

	merged := &ParseResult{
		Errors: make(map[string]error),
	}

	var (
		mu sync.Mutex
		wg sync.WaitGroup
	)

	visited := &visitTracker{}
	sem := make(chan struct{}, p.concurrency)

	for _, u := range sitemapURLs {
		sitemapURL := strings.TrimSpace(u)
		if sitemapURL == "" {
			continue
		}

		wg.Add(1)
		go func(target string) {
			defer wg.Done()

			select {
			case sem <- struct{}{}:
				defer func() { <-sem }()
			case <-ctx.Done():
				return
			}

			local := &ParseResult{
				Errors: make(map[string]error),
			}

			p.parseRecursive(ctx, target, 0, local, visited)

			mu.Lock()
			merged.URLs = append(merged.URLs, local.URLs...)
			for k, v := range local.Errors {
				merged.Errors[k] = v
			}
			mu.Unlock()
		}(sitemapURL)
	}

	wg.Wait()

	merged.Visited = visited.list()

	p.logVerbose("ParseAll complete: %d URLs from %d sitemaps, %d errors",
		len(merged.URLs), len(merged.Visited), len(merged.Errors))

	return merged, nil
}

// DiscoverAndParse is a convenience method that discovers sitemaps for a domain
// and parses all of them. This is the typical entry point for most users.
func (p *Parser) DiscoverAndParse(ctx context.Context, baseURL string) (*ParseResult, error) {
	sitemapURLs, err := p.DiscoverSitemaps(ctx, baseURL)
	if err != nil {
		return nil, fmt.Errorf("discovery failed: %w", err)
	}

	if len(sitemapURLs) == 0 {
		return nil, fmt.Errorf("no sitemaps found for %s", baseURL)
	}

	p.logVerbose("Discovered %d sitemaps for %s, starting parse", len(sitemapURLs), baseURL)

	return p.ParseAll(ctx, sitemapURLs)
}

// ─────────────────────────────────────────────
//  Recursive Engine
// ─────────────────────────────────────────────

// parseRecursive fetches, decodes, and processes a single sitemap URL.
// For sitemap indexes, it spawns concurrent workers for child sitemaps.
func (p *Parser) parseRecursive(ctx context.Context, sitemapURL string, depth int, result *ParseResult, visited *visitTracker) {
	// Context cancelled — bail immediately
	if ctx.Err() != nil {
		return
	}

	// Depth guard
	if depth > p.maxDepth {
		p.logVerbose("Max depth %d reached, skipping %s", p.maxDepth, sitemapURL)
		return
	}

	// Cycle detection
	if !visited.mark(sitemapURL) {
		p.logVerbose("Already visited %s, skipping", sitemapURL)
		return
	}

	// Fetch
	data, err := p.fetch(ctx, sitemapURL)
	if err != nil {
		result.recordError(sitemapURL, fmt.Errorf("fetch failed: %w", err))
		return
	}

	// Decode
	parsed, err := p.decode(data, sitemapURL)
	if err != nil {
		result.recordError(sitemapURL, fmt.Errorf("decode failed: %w", err))
		return
	}

	switch parsed.Type {
	case "urlset":
		// Separate real page URLs from child sitemaps disguised as <url> entries
		var pageURLs []URL
		var childSitemaps []Sitemap

		for _, u := range parsed.URLs {
			if isSitemapURL(u.Loc) {
				childSitemaps = append(childSitemaps, Sitemap{
					Loc:     u.Loc,
					LastMod: u.LastMod,
				})
			} else {
				pageURLs = append(pageURLs, u)
			}
		}

		// Append actual page URLs
		if len(pageURLs) > 0 {
			result.appendURLs(pageURLs)
		}

		// Recurse into child sitemaps found inside <urlset>
		if len(childSitemaps) > 0 {
			p.logVerbose("Found %d child sitemap(s) inside <urlset> at %s", len(childSitemaps), sitemapURL)
			p.parseChildren(ctx, childSitemaps, depth, result, visited)
		}

	case "sitemapindex":
		p.parseChildren(ctx, parsed.Sitemaps, depth, result, visited)

	default:
		result.recordError(sitemapURL, fmt.Errorf("unknown sitemap type: %s", parsed.Type))
	}

}

// parseChildren processes child sitemaps from a sitemap index concurrently.
func (p *Parser) parseChildren(ctx context.Context, children []Sitemap, parentDepth int, result *ParseResult, visited *visitTracker) {
	if len(children) == 0 {
		return
	}

	var wg sync.WaitGroup
	sem := make(chan struct{}, p.concurrency)

	for _, child := range children {
		childURL := strings.TrimSpace(child.Loc)
		if childURL == "" {
			continue
		}

		wg.Add(1)
		go func(target string) {
			defer wg.Done()

			select {
			case sem <- struct{}{}:
				defer func() { <-sem }()
			case <-ctx.Done():
				return
			}

			p.parseRecursive(ctx, target, parentDepth+1, result, visited)
		}(childURL)
	}

	wg.Wait()
}

// ─────────────────────────────────────────────
//  Visit Tracker (Cycle Detection)
// ─────────────────────────────────────────────

// visitTracker is a concurrency-safe set for tracking visited sitemap URLs.
type visitTracker struct {
	mu      sync.Mutex
	visited map[string]struct{}
	order   []string
}

// mark attempts to mark a URL as visited. Returns true if this is the first visit,
// false if already seen (cycle detected).
func (v *visitTracker) mark(rawURL string) bool {
	normalized := normalizeVisitKey(rawURL)

	v.mu.Lock()
	defer v.mu.Unlock()

	if v.visited == nil {
		v.visited = make(map[string]struct{})
	}

	if _, exists := v.visited[normalized]; exists {
		return false
	}

	v.visited[normalized] = struct{}{}
	v.order = append(v.order, rawURL)
	return true
}

// list returns all visited URLs in the order they were first seen.
func (v *visitTracker) list() []string {
	v.mu.Lock()
	defer v.mu.Unlock()

	out := make([]string, len(v.order))
	copy(out, v.order)
	return out
}

// normalizeVisitKey normalizes a URL for deduplication in cycle detection.
// Strips trailing slashes, lowercases scheme and host.
func normalizeVisitKey(rawURL string) string {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return strings.TrimRight(strings.ToLower(rawURL), "/")
	}

	parsed.Scheme = strings.ToLower(parsed.Scheme)
	parsed.Host = strings.ToLower(parsed.Host)
	parsed.Path = strings.TrimRight(parsed.Path, "/")
	parsed.Fragment = ""

	return parsed.String()
}

// ─────────────────────────────────────────────
//  ParseResult Helpers (thread-safe)
// ─────────────────────────────────────────────

var resultMu sync.Mutex

// appendURLs adds URLs to the result in a thread-safe manner.
func (r *ParseResult) appendURLs(urls []URL) {
	resultMu.Lock()
	r.URLs = append(r.URLs, urls...)
	resultMu.Unlock()
}

// recordError records a non-fatal error for a specific sitemap URL.
func (r *ParseResult) recordError(sitemapURL string, err error) {
	resultMu.Lock()
	r.Errors[sitemapURL] = err
	resultMu.Unlock()
}

// isSitemapURL checks if a URL likely points to another sitemap rather than a page.
func isSitemapURL(u string) bool {
	lower := strings.ToLower(u)
	return strings.HasSuffix(lower, ".xml") ||
		strings.HasSuffix(lower, ".xml.gz") ||
		strings.HasSuffix(lower, "sitemap.xml") ||
		strings.HasSuffix(lower, "-sitemap.xml")
}
