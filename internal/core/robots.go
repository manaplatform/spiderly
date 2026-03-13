package core

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"
)

// ─────────────────────────────────────────────
//  Constants
// ─────────────────────────────────────────────

const (
	// DefaultUserAgent is sent in HTTP requests and matched against
	// robots.txt rules when no custom agent is configured.
	DefaultUserAgent = "Spiderly/1.0"

	// robotsTxtMaxSize caps how many bytes we read from robots.txt
	// to prevent memory exhaustion on malformed files.
	robotsTxtMaxSize = 512 * 1024 // 512 KB

	// robotsTxtCacheTTL controls how long a parsed robots.txt stays
	// valid before we re-fetch it.
	robotsTxtCacheTTL = 1 * time.Hour
)

// ─────────────────────────────────────────────
//  RobotsChecker — Public Interface
// ─────────────────────────────────────────────

// RobotsChecker fetches, parses, caches and evaluates robots.txt
// files.  It is safe for concurrent use from multiple goroutines.
type RobotsChecker struct {
	mu        sync.RWMutex
	cache     map[string]*robotsCacheEntry
	client    *http.Client
	userAgent string
	enabled   bool

	stats RobotsStats
}

// RobotsStats exposes counters for observability.
type RobotsStats struct {
	Fetches  int64 // number of robots.txt HTTP requests made
	Hits     int64 // cache hits
	Allowed  int64 // URLs that passed the check
	Denied   int64 // URLs blocked by robots.txt
	Errors   int64 // fetch / parse failures (we allow on error)
}

// robotsCacheEntry stores a parsed robots.txt for one origin.
type robotsCacheEntry struct {
	rules     *RobotsRules
	fetchedAt time.Time
	err       error // non-nil if the fetch failed
}

// ─────────────────────────────────────────────
//  Constructor
// ─────────────────────────────────────────────

// RobotsConfig controls the checker behaviour.
type RobotsConfig struct {
	// UserAgent is matched against User-agent directives.
	// Falls back to DefaultUserAgent if empty.
	UserAgent string

	// Enabled turns compliance checking on/off globally.
	// When false, IsAllowed always returns true.
	Enabled bool

	// Timeout for fetching each robots.txt file.
	Timeout time.Duration
}

// NewRobotsChecker creates a ready-to-use checker.
func NewRobotsChecker(cfg RobotsConfig) *RobotsChecker {
	ua := cfg.UserAgent
	if ua == "" {
		ua = DefaultUserAgent
	}

	timeout := cfg.Timeout
	if timeout == 0 {
		timeout = 10 * time.Second
	}

	return &RobotsChecker{
		cache:     make(map[string]*robotsCacheEntry),
		userAgent: ua,
		enabled:   cfg.Enabled,
		client: &http.Client{
			Timeout: timeout,
			// Don't follow redirects — we handle them manually
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				if len(via) >= 5 {
					return fmt.Errorf("too many redirects fetching robots.txt")
				}
				return nil
			},
		},
	}
}

// ─────────────────────────────────────────────
//  Public Methods
// ─────────────────────────────────────────────

// IsAllowed checks whether the crawler is permitted to fetch rawURL
// according to the site's robots.txt.
//
// Behaviour on edge cases (following the standard):
//   - robots.txt fetch error → ALLOW (fail open)
//   - robots.txt 4xx         → ALLOW (treat as "no restrictions")
//   - robots.txt 5xx         → DENY  (server is having trouble)
//   - Empty robots.txt       → ALLOW
//   - Checker disabled       → ALLOW
func (rc *RobotsChecker) IsAllowed(ctx context.Context, rawURL string) (bool, error) {
	if !rc.enabled {
		return true, nil
	}

	parsed, err := url.Parse(rawURL)
	if err != nil {
		return true, fmt.Errorf("invalid URL: %w", err)
	}

	origin := fmt.Sprintf("%s://%s", parsed.Scheme, parsed.Host)
	urlPath := parsed.Path
	if urlPath == "" {
		urlPath = "/"
	}
	if parsed.RawQuery != "" {
		urlPath = urlPath + "?" + parsed.RawQuery
	}

	rules, err := rc.getRules(ctx, origin)
	if err != nil {
		rc.mu.Lock()
		rc.stats.Errors++
		rc.stats.Allowed++ // fail open
		rc.mu.Unlock()
		return true, nil
	}

	allowed := rules.IsAllowed(rc.userAgent, urlPath)

	rc.mu.Lock()
	if allowed {
		rc.stats.Allowed++
	} else {
		rc.stats.Denied++
	}
	rc.mu.Unlock()

	return allowed, nil
}

// CrawlDelay returns the Crawl-delay directive for the configured
// user agent on the given origin, or 0 if none is specified.
func (rc *RobotsChecker) CrawlDelay(ctx context.Context, origin string) time.Duration {
	if !rc.enabled {
		return 0
	}

	rules, err := rc.getRules(ctx, origin)
	if err != nil || rules == nil {
		return 0
	}

	return rules.CrawlDelay(rc.userAgent)
}

// Sitemaps returns any Sitemap directives found in the robots.txt
// for the given origin.
func (rc *RobotsChecker) Sitemaps(ctx context.Context, origin string) []string {
	rules, err := rc.getRules(ctx, origin)
	if err != nil || rules == nil {
		return nil
	}
	return rules.Sitemaps()
}

// Stats returns a snapshot of checker statistics.
func (rc *RobotsChecker) Stats() RobotsStats {
	rc.mu.RLock()
	defer rc.mu.RUnlock()
	return rc.stats
}

// ClearCache drops all cached robots.txt data.
func (rc *RobotsChecker) ClearCache() {
	rc.mu.Lock()
	defer rc.mu.Unlock()
	rc.cache = make(map[string]*robotsCacheEntry)
}

// Prefetch fetches and caches robots.txt for the given origins
// concurrently.  Useful to warm the cache before crawling starts.
func (rc *RobotsChecker) Prefetch(ctx context.Context, origins []string) {
	var wg sync.WaitGroup
	sem := make(chan struct{}, 5) // limit concurrent fetches

	for _, origin := range origins {
		wg.Add(1)
		go func(o string) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			rc.getRules(ctx, o) //nolint:errcheck
		}(origin)
	}

	wg.Wait()
}

// ─────────────────────────────────────────────
//  Cache & Fetch
// ─────────────────────────────────────────────

// getRules returns the parsed rules for an origin, fetching and
// caching as needed.
func (rc *RobotsChecker) getRules(ctx context.Context, origin string) (*RobotsRules, error) {
	rc.mu.RLock()
	entry, exists := rc.cache[origin]
	rc.mu.RUnlock()

	if exists && time.Since(entry.fetchedAt) < robotsTxtCacheTTL {
		rc.mu.Lock()
		rc.stats.Hits++
		rc.mu.Unlock()

		if entry.err != nil {
			return nil, entry.err
		}
		return entry.rules, nil
	}

	// Fetch (or re-fetch if TTL expired)
	rules, err := rc.fetch(ctx, origin)

	rc.mu.Lock()
	rc.stats.Fetches++
	rc.cache[origin] = &robotsCacheEntry{
		rules:     rules,
		fetchedAt: time.Now(),
		err:       err,
	}
	rc.mu.Unlock()

	return rules, err
}

// fetch downloads and parses robots.txt from origin.
func (rc *RobotsChecker) fetch(ctx context.Context, origin string) (*RobotsRules, error) {
	robotsURL := origin + "/robots.txt"

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, robotsURL, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("User-Agent", rc.userAgent)
	req.Header.Set("Accept", "text/plain")

	resp, err := rc.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch %s: %w", robotsURL, err)
	}
	defer resp.Body.Close()

	switch {
	case resp.StatusCode >= 200 && resp.StatusCode < 300:
		// Success — parse the body
		body, err := io.ReadAll(io.LimitReader(resp.Body, robotsTxtMaxSize))
		if err != nil {
			return nil, fmt.Errorf("read body: %w", err)
		}
		return ParseRobotsTxt(string(body)), nil

	case resp.StatusCode >= 400 && resp.StatusCode < 500:
		// 4xx → no restrictions (file doesn't exist or is forbidden)
		return newAllowAllRules(), nil

	case resp.StatusCode >= 500:
		// 5xx → server trouble, be cautious and deny
		return newDenyAllRules(), nil

	default:
		// 3xx should have been followed by the client; treat as allow
		return newAllowAllRules(), nil
	}
}

// ─────────────────────────────────────────────
//  RobotsRules — Parsed Representation
// ─────────────────────────────────────────────

// RobotsRules holds the parsed directives from a robots.txt file.
type RobotsRules struct {
	groups   []robotsGroup
	sitemaps []string

	// Special modes
	allowAll bool
	denyAll  bool
}

// robotsGroup represents one User-agent block.
type robotsGroup struct {
	agents     []string // lowercased user-agent tokens
	allow      []string // path prefixes / patterns
	disallow   []string
	crawlDelay time.Duration
}

// ─────────────────────────────────────────────
//  Parser
// ─────────────────────────────────────────────

// ParseRobotsTxt parses a robots.txt body into RobotsRules.
// It follows the Google robots.txt specification:
// https://developers.google.com/search/docs/crawling-indexing/robots/robots_txt
func ParseRobotsTxt(body string) *RobotsRules {
	rules := &RobotsRules{}

	lines := strings.Split(body, "\n")

	var currentGroup *robotsGroup
	seenAgent := false

	for _, rawLine := range lines {
		// Strip comments
		if idx := strings.IndexByte(rawLine, '#'); idx >= 0 {
			rawLine = rawLine[:idx]
		}
		line := strings.TrimSpace(rawLine)
		if line == "" {
			// Blank line ends the current group
			if currentGroup != nil && seenAgent {
				rules.groups = append(rules.groups, *currentGroup)
				currentGroup = nil
				seenAgent = false
			}
			continue
		}

		// Split directive: value
		colonIdx := strings.IndexByte(line, ':')
		if colonIdx < 0 {
			continue
		}
		directive := strings.TrimSpace(line[:colonIdx])
		value := strings.TrimSpace(line[colonIdx+1:])

		directiveLower := strings.ToLower(directive)

		switch directiveLower {
		case "user-agent":
			if currentGroup == nil || (seenAgent && len(currentGroup.allow)+len(currentGroup.disallow) > 0) {
				// Start a new group
				if currentGroup != nil {
					rules.groups = append(rules.groups, *currentGroup)
				}
				currentGroup = &robotsGroup{}
			}
			currentGroup.agents = append(currentGroup.agents, strings.ToLower(value))
			seenAgent = true

		case "disallow":
			if currentGroup == nil {
				currentGroup = &robotsGroup{agents: []string{"*"}}
				seenAgent = true
			}
			if value != "" {
				currentGroup.disallow = append(currentGroup.disallow, value)
			}

		case "allow":
			if currentGroup == nil {
				currentGroup = &robotsGroup{agents: []string{"*"}}
				seenAgent = true
			}
			if value != "" {
				currentGroup.allow = append(currentGroup.allow, value)
			}

		case "crawl-delay":
			if currentGroup == nil {
				currentGroup = &robotsGroup{agents: []string{"*"}}
				seenAgent = true
			}
			if seconds, err := strconv.ParseFloat(value, 64); err == nil && seconds > 0 {
				currentGroup.crawlDelay = time.Duration(seconds * float64(time.Second))
			}

		case "sitemap":
			if value != "" {
				rules.sitemaps = append(rules.sitemaps, value)
			}
		}
	}

	// Flush last group
	if currentGroup != nil && seenAgent {
		rules.groups = append(rules.groups, *currentGroup)
	}

	return rules
}

// ─────────────────────────────────────────────
//  Rule Evaluation
// ─────────────────────────────────────────────

// IsAllowed checks whether the given user agent may access urlPath.
//
// Matching follows the Google specification:
//  1. Find the most specific matching group for the agent
//  2. Among matching rules, the longest pattern wins
//  3. If allow and disallow have equal length, allow wins
func (r *RobotsRules) IsAllowed(userAgent, urlPath string) bool {
	if r.allowAll {
		return true
	}
	if r.denyAll {
		return false
	}

	group := r.findGroup(userAgent)
	if group == nil {
		return true // no matching group → allowed
	}

	// No rules in the group → allowed
	if len(group.allow) == 0 && len(group.disallow) == 0 {
		return true
	}

	// Find the best matching allow and disallow rules
	bestAllow := -1
	bestDisallow := -1

	for _, pattern := range group.allow {
		if matchLen := robotsMatch(pattern, urlPath); matchLen >= 0 {
			if matchLen > bestAllow {
				bestAllow = matchLen
			}
		}
	}

	for _, pattern := range group.disallow {
		if matchLen := robotsMatch(pattern, urlPath); matchLen >= 0 {
			if matchLen > bestDisallow {
				bestDisallow = matchLen
			}
		}
	}

	// No rules matched → allowed
	if bestAllow < 0 && bestDisallow < 0 {
		return true
	}

	// Only allow matched → allowed
	if bestAllow >= 0 && bestDisallow < 0 {
		return true
	}

	// Only disallow matched → denied
	if bestAllow < 0 && bestDisallow >= 0 {
		return false
	}

	// Both matched → longest wins; tie goes to allow
	return bestAllow >= bestDisallow
}

// CrawlDelay returns the crawl-delay for the matching group.
func (r *RobotsRules) CrawlDelay(userAgent string) time.Duration {
	if r == nil {
		return 0
	}
	group := r.findGroup(userAgent)
	if group == nil {
		return 0
	}
	return group.crawlDelay
}

// Sitemaps returns all Sitemap directives found.
func (r *RobotsRules) Sitemaps() []string {
	if r == nil {
		return nil
	}
	out := make([]string, len(r.sitemaps))
	copy(out, r.sitemaps)
	return out
}

// ─────────────────────────────────────────────
//  Group Selection
// ─────────────────────────────────────────────

// findGroup returns the most specific group matching userAgent.
// Priority: exact agent name > substring match > wildcard "*".
func (r *RobotsRules) findGroup(userAgent string) *robotsGroup {
	ua := strings.ToLower(userAgent)

	// Extract the product token (first word before '/')
	// e.g. "Spiderly/1.0" → "spiderly"
	token := ua
	if idx := strings.IndexByte(token, '/'); idx > 0 {
		token = token[:idx]
	}
	token = strings.TrimSpace(token)

	var bestGroup *robotsGroup
	bestSpecificity := -1

	for i := range r.groups {
		g := &r.groups[i]
		for _, agent := range g.agents {
			specificity := agentSpecificity(agent, token, ua)
			if specificity > bestSpecificity {
				bestSpecificity = specificity
				bestGroup = g
			}
		}
	}

	return bestGroup
}

// agentSpecificity returns how well an agent directive matches.
//   - 0 = wildcard "*"
//   - 1 = substring match
//   - 2 = exact token match
//   - -1 = no match
func agentSpecificity(directive, token, fullUA string) int {
	directive = strings.TrimSpace(directive)

	if directive == "*" {
		return 0
	}

	// Exact match on the product token
	if directive == token {
		return 2
	}

	// Substring match on the full user-agent string
	if strings.Contains(fullUA, directive) {
		return 1
	}

	return -1
}

// ─────────────────────────────────────────────
//  Pattern Matching
// ─────────────────────────────────────────────

// robotsMatch checks whether a robots.txt pattern matches urlPath.
// Returns the "specificity" (length of the matched prefix) or -1
// if there is no match.
//
// Supported syntax:
//   - Plain prefix:  "/foo"  matches "/foo", "/foobar", "/foo/bar"
//   - Wildcard:      "/foo/*/bar"  matches "/foo/anything/bar"
//   - End anchor:    "/foo$" matches exactly "/foo"
func robotsMatch(pattern, urlPath string) int {
	if pattern == "" {
		return -1
	}

	// Handle end-of-string anchor
	anchored := false
	if strings.HasSuffix(pattern, "$") {
		anchored = true
		pattern = pattern[:len(pattern)-1]
	}

	// No wildcards and no anchor → simple prefix match
	if !strings.Contains(pattern, "*") {
		if anchored {
			if urlPath == pattern {
				return len(pattern)
			}
			return -1
		}
		if strings.HasPrefix(urlPath, pattern) {
			return len(pattern)
		}
		return -1
	}

	// Wildcard matching
	parts := strings.Split(pattern, "*")
	pos := 0

	for i, part := range parts {
		if part == "" {
			continue // leading or consecutive wildcards
		}

		idx := strings.Index(urlPath[pos:], part)
		if idx < 0 {
			return -1
		}

		// First segment must match at the start (prefix)
		if i == 0 && idx != 0 {
			return -1
		}

		pos += idx + len(part)
	}

	if anchored && pos != len(urlPath) {
		return -1
	}

	return len(pattern)
}

// ─────────────────────────────────────────────
//  Special Rule Sets
// ─────────────────────────────────────────────

// newAllowAllRules returns rules that permit everything.
func newAllowAllRules() *RobotsRules {
	return &RobotsRules{allowAll: true}
}

// newDenyAllRules returns rules that block everything.
func newDenyAllRules() *RobotsRules {
	return &RobotsRules{denyAll: true}
}

// ─────────────────────────────────────────────
//  String Representation (debugging)
// ─────────────────────────────────────────────

// String returns a human-readable summary of the rules.
func (r *RobotsRules) String() string {
	if r.allowAll {
		return "robots.txt: ALLOW ALL"
	}
	if r.denyAll {
		return "robots.txt: DENY ALL"
	}

	var b strings.Builder
	fmt.Fprintf(&b, "robots.txt: %d group(s), %d sitemap(s)\n", len(r.groups), len(r.sitemaps))

	for i, g := range r.groups {
		fmt.Fprintf(&b, "  Group %d: agents=%v\n", i, g.agents)
		for _, a := range g.allow {
			fmt.Fprintf(&b, "    Allow: %s\n", a)
		}
		for _, d := range g.disallow {
			fmt.Fprintf(&b, "    Disallow: %s\n", d)
		}
		if g.crawlDelay > 0 {
			fmt.Fprintf(&b, "    Crawl-delay: %s\n", g.crawlDelay)
		}
	}

	for _, s := range r.sitemaps {
		fmt.Fprintf(&b, "  Sitemap: %s\n", s)
	}

	return b.String()
}