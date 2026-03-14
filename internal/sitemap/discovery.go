package sitemap

import (
	"bufio"
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"sync"
)

// commonSitemapPaths are checked when robots.txt yields no results.
var commonSitemapPaths = []string{
	"/sitemap.xml",
	"/sitemap.xml.gz",
	"/sitemap_index.xml",
	"/sitemap_index.xml.gz",
	"/sitemap1.xml",
	"/sitemaps/sitemap.xml",
	"/sitemap/sitemap.xml",
	"/sitemap/sitemap-index.xml",
	"/wp-sitemap.xml",
	"/news-sitemap.xml",
	"/post-sitemap.xml",
}

// DiscoverSitemaps finds all sitemap URLs for a domain by checking robots.txt
// and probing common paths. Returns deduplicated URLs.
func (p *Parser) DiscoverSitemaps(ctx context.Context, baseURL string) ([]string, error) {
	base, err := normalizeBaseURL(baseURL)
	if err != nil {
		return nil, fmt.Errorf("invalid base URL: %w", err)
	}

	// Phase 1: Parse robots.txt for declared sitemaps
	robotsSitemaps := p.sitemapsFromRobots(ctx, base)
	p.logVerbose("Found %d sitemaps in robots.txt for %s", len(robotsSitemaps), base)

	// Phase 2: Probe common paths concurrently
	probedSitemaps := p.probeCommonPaths(ctx, base)
	p.logVerbose("Found %d sitemaps from common path probing for %s", len(probedSitemaps), base)

	// Merge and deduplicate
	return deduplicate(append(robotsSitemaps, probedSitemaps...)), nil
}

// DiscoverSitemapsFiltered runs DiscoverSitemaps then filters results by a predicate.
func (p *Parser) DiscoverSitemapsFiltered(ctx context.Context, baseURL string, filter func(string) bool) ([]string, error) {
	all, err := p.DiscoverSitemaps(ctx, baseURL)
	if err != nil {
		return nil, err
	}

	filtered := make([]string, 0, len(all))
	for _, u := range all {
		if filter(u) {
			filtered = append(filtered, u)
		}
	}

	return filtered, nil
}

// ─────────────────────────────────────────────
//  robots.txt Parsing
// ─────────────────────────────────────────────

// sitemapsFromRobots fetches and parses robots.txt, extracting Sitemap: directives.
func (p *Parser) sitemapsFromRobots(ctx context.Context, base string) []string {
	robotsURL := base + "/robots.txt"

	req, err := p.newRequest(ctx, http.MethodGet, robotsURL)
	if err != nil {
		p.logVerbose("Failed to create robots.txt request: %v", err)
		return nil
	}

	resp, err := p.client.Do(req)
	if err != nil {
		p.logVerbose("Failed to fetch robots.txt: %v", err)
		return nil
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		p.logVerbose("robots.txt returned HTTP %d", resp.StatusCode)
		return nil
	}

	var sitemaps []string
	scanner := bufio.NewScanner(resp.Body)

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		// Skip empty lines and comments
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		lower := strings.ToLower(line)
		if !strings.HasPrefix(lower, "sitemap:") {
			continue
		}

		// Extract the URL after "Sitemap:"
		sitemapURL := strings.TrimSpace(line[len("sitemap:"):])
		if sitemapURL == "" {
			continue
		}

		// Validate it's a proper URL
		if _, err := url.ParseRequestURI(sitemapURL); err == nil {
			sitemaps = append(sitemaps, sitemapURL)
		} else {
			p.logVerbose("Skipping invalid sitemap URL in robots.txt: %s", sitemapURL)
		}
	}

	return sitemaps
}

// ─────────────────────────────────────────────
//  Common Path Probing
// ─────────────────────────────────────────────

// probeCommonPaths checks common sitemap paths concurrently using a worker pool.
func (p *Parser) probeCommonPaths(ctx context.Context, base string) []string {
	var (
		mu    sync.Mutex
		found []string
		wg    sync.WaitGroup
	)

	// Semaphore channel limits concurrency
	sem := make(chan struct{}, p.concurrency)

	for _, path := range commonSitemapPaths {
		candidateURL := base + path

		wg.Add(1)
		go func(u string) {
			defer wg.Done()

			// Acquire semaphore slot
			select {
			case sem <- struct{}{}:
				defer func() { <-sem }()
			case <-ctx.Done():
				return
			}

			if p.exists(ctx, u) {
				mu.Lock()
				found = append(found, u)
				mu.Unlock()
				p.logVerbose("Probed and found: %s", u)
			}
		}(candidateURL)
	}

	wg.Wait()
	return found
}

// ─────────────────────────────────────────────
//  Helpers
// ─────────────────────────────────────────────

// normalizeBaseURL ensures the base URL has a scheme and no trailing slash.
func normalizeBaseURL(rawURL string) (string, error) {
	raw := strings.TrimSpace(rawURL)
	if raw == "" {
		return "", fmt.Errorf("empty URL")
	}

	// Add scheme if missing
	if !strings.HasPrefix(raw, "http://") && !strings.HasPrefix(raw, "https://") {
		raw = "https://" + raw
	}

	parsed, err := url.Parse(raw)
	if err != nil {
		return "", err
	}

	if parsed.Host == "" {
		return "", fmt.Errorf("missing host in URL: %s", raw)
	}

	// Rebuild as scheme://host only — strip path, query, fragment
	return fmt.Sprintf("%s://%s", parsed.Scheme, parsed.Host), nil
}

// deduplicate returns unique URLs preserving first-seen order.
func deduplicate(urls []string) []string {
	seen := make(map[string]struct{}, len(urls))
	result := make([]string, 0, len(urls))

	for _, u := range urls {
		normalized := strings.TrimRight(strings.TrimSpace(u), "/")
		if _, exists := seen[normalized]; exists {
			continue
		}
		seen[normalized] = struct{}{}
		result = append(result, u)
	}

	return result
}
