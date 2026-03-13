package core

import (
	"context"
	"fmt"
	"sync"
	"time"

	"spiderly/internal/chunker"
	"spiderly/internal/crawler"
	"spiderly/internal/models"
)

// ─────────────────────────────────────────────
//  Core — Main Orchestrator
// ─────────────────────────────────────────────

// Core is the central orchestrator for Spiderly crawl operations.
// It coordinates strategy resolution, URL normalization, robots.txt
// compliance, crawl execution, result streaming, and metrics.
type Core struct {
	config    CoreConfig
	crawler   *crawler.Crawler
	chunker   *chunker.Chunker
	stats     *models.CrawlStats
	metrics   *Metrics
	results   []models.ScrapedPage
	mu        sync.RWMutex
	ctx       context.Context
	cancel    context.CancelFunc
	startTime time.Time
	logger    *Logger

	// P0: URL normalization & deduplication
	normalizer *URLNormalizer
	errors []*CrawlError // collected crawl errors

	// P0: robots.txt compliance
	robots *RobotsChecker

	// P1: Streaming sinks for memory-efficient output
	sinks []Sink
	seen *URLDedup // URL deduplication set
	sink Sink  
}

// ─────────────────────────────────────────────
//  Constructors
// ─────────────────────────────────────────────

// New creates a Core with sensible defaults for simple crawling.
func New(targetURL string, maxPages int) *Core {
	return NewCore(CoreConfig{
		TargetURL:   targetURL,
		MaxPages:    maxPages,
		MaxDepth:    10,
		Concurrency: 5,
		Delay:       200 * time.Millisecond,
		Timeout:     30 * time.Second,
		ChunkSize:   50,
		MaxWorkers:  4,
	})
}

// NewChunked creates a Core with chunker enabled for large sitemap crawls.
func NewChunked(targetURL string, maxPages, chunkSize, workers int) *Core {
	return NewCore(CoreConfig{
		TargetURL:     targetURL,
		MaxPages:      maxPages,
		MaxDepth:      10,
		Concurrency:   5,
		Delay:         200 * time.Millisecond,
		Timeout:       30 * time.Second,
		EnableChunker: true,
		ChunkSize:     chunkSize,
		MaxWorkers:    workers,
	})
}

// NewWithOptions creates a Core using functional options pattern.
func NewWithOptions(targetURL string, opts ...Option) *Core {
	cfg := CoreConfig{
		TargetURL:   targetURL,
		MaxPages:    100,
		MaxDepth:    10,
		Concurrency: 5,
		Delay:       200 * time.Millisecond,
		Timeout:     30 * time.Second,
		ChunkSize:   50,
		MaxWorkers:  4,
		RetryConfig: DefaultRetryConfig(),
	}
	for _, opt := range opts {
		opt(&cfg)
	}
	return NewCore(cfg)
}

// NewCore creates a Core from an explicit CoreConfig.
// This is the canonical constructor — all other constructors delegate here.
func NewCore(cfg CoreConfig) *Core {
	ctx, cancel := context.WithCancel(context.Background())

	// Ensure retry config has sane defaults
	if cfg.RetryConfig.MaxRetries == 0 && cfg.RetryConfig.BaseDelay == 0 {
		cfg.RetryConfig = DefaultRetryConfig()
	}

	// Compile patterns if raw strings were provided
	cfg.CompilePatterns()

	c := &Core{
		config:    cfg,
		stats:     &models.CrawlStats{},
		metrics:   NewMetrics(),
		results:   make([]models.ScrapedPage, 0, cfg.MaxPages),
		ctx:       ctx,
		cancel:    cancel,
		startTime: time.Now(),
		logger:    NewLogger(cfg.NoColor, cfg.Verbose),
	}

	// P0: Initialize URL normalizer with dedup tracking
	c.normalizer = NewURLNormalizer()

	// P0: Initialize robots.txt checker (lazy-loaded on first use)
	c.robots = NewRobotsChecker(RobotsConfig{
		UserAgent: cfg.UserAgent,
		Enabled:   cfg.RespectRobots,
		Timeout:   cfg.Timeout,
	})


	return c
}

// ─────────────────────────────────────────────
//  ApplyConfig — Merge partial config updates
// ─────────────────────────────────────────────

// ApplyConfig merges non-zero fields from cfg into the Core's config.
// This allows CLI or API layers to override specific settings.
func (c *Core) ApplyConfig(cfg CoreConfig) {
	if cfg.TargetURL != "" {
		c.config.TargetURL = cfg.TargetURL
	}
	if cfg.SitemapURL != "" {
		c.config.SitemapURL = cfg.SitemapURL
	}
	if cfg.MaxPages > 0 {
		c.config.MaxPages = cfg.MaxPages
	}
	if cfg.MaxDepth > 0 {
		c.config.MaxDepth = cfg.MaxDepth
	}
	if cfg.Concurrency > 0 {
		c.config.Concurrency = cfg.Concurrency
	}
	if cfg.Delay > 0 {
		c.config.Delay = cfg.Delay
	}
	if cfg.Timeout > 0 {
		c.config.Timeout = cfg.Timeout
	}
	if cfg.MinPriority > 0 {
		c.config.MinPriority = cfg.MinPriority
	}
	if cfg.URLPattern != "" {
		c.config.URLPattern = cfg.URLPattern
	}
	if cfg.ChunkSize > 0 {
		c.config.ChunkSize = cfg.ChunkSize
	}
	if cfg.MaxWorkers > 0 {
		c.config.MaxWorkers = cfg.MaxWorkers
	}
	if cfg.ProductPattern != "" {
		c.config.ProductPattern = cfg.ProductPattern
	}
	if cfg.UserAgent != "" {
		c.config.UserAgent = cfg.UserAgent
		c.robots = NewRobotsChecker(RobotsConfig{
			UserAgent: cfg.UserAgent,
			Enabled:   c.config.RespectRobots,
			Timeout:   c.config.Timeout,
		})
	}

	// Boolean flags — always apply (no "zero value" ambiguity for bools)
	c.config.ForceRecursive = cfg.ForceRecursive
	c.config.Headless = cfg.Headless
	c.config.Verbose = cfg.Verbose
	c.config.NoColor = cfg.NoColor
	c.config.EnableChunker = cfg.EnableChunker
	c.config.RespectRobots = cfg.RespectRobots
	c.config.DisableNormalization = cfg.DisableNormalization

	// Product mode settings
	c.config.ProductMode = cfg.ProductMode
	c.config.ExtractSpecs = cfg.ExtractSpecs
	c.config.ExtractImages = cfg.ExtractImages

	if len(cfg.ProductSitemaps) > 0 {
		c.config.ProductSitemaps = cfg.ProductSitemaps
	}
	if len(cfg.ExcludePatterns) > 0 {
		c.config.ExcludePatterns = cfg.ExcludePatterns
	}

	// Retry config
	if cfg.RetryConfig.MaxRetries > 0 {
		c.config.RetryConfig = cfg.RetryConfig
	}

	// Recompile patterns after config merge
	c.config.CompilePatterns()

	// Rebuild logger with updated settings
	c.logger = NewLogger(c.config.NoColor, c.config.Verbose)
}

// ─────────────────────────────────────────────
//  color
// ─────────────────────────────────────────────


// color is a convenience wrapper for the logger's color method.
func (c *Core) color(clr, text string) string {
	return c.logger.color(clr, text)
}

// ─────────────────────────────────────────────
//  Sink Management
// ─────────────────────────────────────────────

// AddSink registers a streaming sink for real-time result output.
// Sinks receive each page as it's scraped, avoiding full in-memory buffering.
func (c *Core) AddSink(s Sink) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.sinks = append(c.sinks, s)
}

// initSinks opens all registered sinks.
func (c *Core) initSinks() error {
	for _, s := range c.sinks {
		if err := s.Open(); err != nil {
			return fmt.Errorf("sink %s: %w", s.Name(), err)
		}
	}
	return nil
}

// flushToSinks sends a scraped page to all registered sinks.
func (c *Core) flushToSinks(page models.ScrapedPage) {
	for _, s := range c.sinks {
		if err := s.Write(page); err != nil {
			c.logger.Warning("Sink %s write error: %v", s.Name(), err)
		}
	}
}

// closeSinks flushes and closes all registered sinks.
func (c *Core) closeSinks() {
	for _, s := range c.sinks {
		if err := s.Close(); err != nil {
			c.logger.Warning("Sink %s close error: %v", s.Name(), err)
		}
	}
}

// ─────────────────────────────────────────────
//  URL Filtering (robots + normalization)
// ─────────────────────────────────────────────

func (c *Core) shouldCrawlURL(rawURL string) (string, bool) {
	normalized := rawURL
	if !c.config.DisableNormalization {
		var err error
		normalized, err = c.normalizer.Normalize(rawURL)
		if err != nil {
			c.logger.Verbose("Normalization failed for %s: %v", rawURL, err)
			return rawURL, false
		}
	}

	if c.normalizer.IsSeen(normalized) {
		c.logger.Verbose("Duplicate skipped: %s", normalized)
		c.metrics.IncrementDuplicatesSkipped()
		return normalized, false
	}

	c.normalizer.MarkSeen(normalized)

	// robots.txt check — pass context
	if c.config.RespectRobots {
		allowed, _ := c.robots.IsAllowed(c.ctx, normalized)
		if !allowed {
			c.logger.Verbose("Blocked by robots.txt: %s", normalized)
			c.metrics.IncrementRobotsBlocked()
			return normalized, false
		}
	}

	return normalized, true
}


// ─────────────────────────────────────────────
//  Public Accessors & Lifecycle
// ─────────────────────────────────────────────

// Stop cancels the crawl context and stops the underlying crawler.
func (c *Core) Stop() {
	c.cancel()
	if c.crawler != nil {
		c.crawler.Stop()
	}
}

// GetResults returns all scraped pages collected so far.
func (c *Core) GetResults() []models.ScrapedPage {
	c.mu.RLock()
	defer c.mu.RUnlock()
	dst := make([]models.ScrapedPage, len(c.results))
	copy(dst, c.results)
	return dst
}

// GetStats returns a snapshot of the current crawl statistics.
func (c *Core) GetStats() models.CrawlStats {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return *c.stats
}

// GetMetrics returns the live metrics snapshot.
func (c *Core) GetMetrics() MetricsSnapshot {
	return c.metrics.Snapshot()
}

// GetConfig returns a copy of the current configuration.
func (c *Core) GetConfig() CoreConfig {
	return c.config
}

// Context returns the Core's context (useful for external coordination).
func (c *Core) Context() context.Context {
	return c.ctx
}
