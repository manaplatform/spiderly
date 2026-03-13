package core

import (
	"fmt"
	"net/url"
	"regexp"
	"strings"
	"time"

	"spiderly/internal/exclude"
)

// ─────────────────────────────────────────────
//  Default Constants
// ─────────────────────────────────────────────

const (
	DefaultMaxPages    = 100
	DefaultMaxDepth    = 10
	DefaultConcurrency = 5
	DefaultDelay       = 200 * time.Millisecond
	DefaultTimeout     = 30 * time.Second
	DefaultChunkSize   = 50
	DefaultMaxWorkers  = 4
	DefaultMaxRetries  = 3
	DefaultRetryDelay  = 1 * time.Second

	// Sink defaults
	DefaultSinkBuffer = 1000

	// Rate limiting
	DefaultMinDelay = 100 * time.Millisecond
	DefaultMaxDelay = 5 * time.Second
)

// ─────────────────────────────────────────────
//  CoreConfig — All crawl configuration
// ─────────────────────────────────────────────

// CoreConfig holds all configuration for the crawl core.
// Use NewCoreConfig() for safe defaults, then apply Option functions.
type CoreConfig struct {
	// ── Target ──
	TargetURL  string `json:"target_url"`
	SitemapURL string `json:"sitemap_url,omitempty"`

	// ── Crawl limits ──
	MaxPages    int           `json:"max_pages"`
	MaxDepth    int           `json:"max_depth"`
	Concurrency int           `json:"concurrency"`
	Delay       time.Duration `json:"delay"`
	Timeout     time.Duration `json:"timeout"`

	// ── Filtering ──
	MinPriority float64 `json:"min_priority,omitempty"`
	URLPattern  string  `json:"url_pattern,omitempty"`

	// ── Behavior flags ──
	ForceRecursive bool `json:"force_recursive"`
	Headless       bool `json:"headless"`
	Verbose        bool `json:"verbose"`
	NoColor        bool `json:"no_color"`

	// ── Retry ──
	MaxRetries int           `json:"max_retries"`
	RetryDelay time.Duration `json:"retry_delay"`

	// ── Chunker settings ──
	EnableChunker  bool   `json:"enable_chunker"`
	ChunkSize      int    `json:"chunk_size"`
	MaxWorkers     int    `json:"max_workers"`
	ProductPattern string `json:"product_pattern,omitempty"` // Raw pattern string

	// ── Product mode ──
	ProductMode     bool     `json:"product_mode"`
	ProductSitemaps []string `json:"product_sitemaps,omitempty"`
	ExtractSpecs    bool     `json:"extract_specs"`
	ExtractImages   bool     `json:"extract_images"`
	ExcludePatterns []string `json:"exclude_patterns,omitempty"`

	// ── Robots.txt ──
	RespectRobotsTxt bool `json:"respect_robots_txt"`
	UserAgent        string `json:"user_agent,omitempty"`

	// ── Normalization ──
	EnableNormalization bool `json:"enable_normalization"`
    DisableNormalization bool

	// ── Streaming sink ──
	SinkType   SinkType `json:"sink_type,omitempty"`
	SinkPath   string   `json:"sink_path,omitempty"`   // For file sinks
	SinkBuffer int      `json:"sink_buffer,omitempty"` // Channel buffer size

	// ── Compiled (internal, not serialized) ──
	CompiledExcludePatterns []*regexp.Regexp `json:"-"`
	CompiledProductPattern  *regexp.Regexp   `json:"-"`
	CompiledURLPattern      *regexp.Regexp   `json:"-"`
	RetryConfig RetryConfig
	// -- robots
	RespectRobots        bool
}

// ─────────────────────────────────────────────
//  SinkType enum
// ─────────────────────────────────────────────

type SinkType string

const (
	SinkMemory  SinkType = "memory"  // Default: collect in slice
	SinkChannel SinkType = "channel" // Stream via channel
	SinkFile    SinkType = "file"    // Stream to JSONL file
	SinkNone    SinkType = "none"    // Discard (metrics only)
)

// ConfigError captures which option failed validation.
type ConfigError struct {
	Field   string
	Message string
}

// Error implements error.
func (e *ConfigError) Error() string {
	if e == nil {
		return ""
	}
	if e.Field == "" {
		return e.Message
	}
	return fmt.Sprintf("%s: %s", e.Field, e.Message)
}

// NewConfigError creates a ConfigError for the provided field.
func NewConfigError(field, msg string) error {
	return &ConfigError{Field: field, Message: msg}
}

// ─────────────────────────────────────────────
//  Default Config Constructor
// ─────────────────────────────────────────────

// NewCoreConfig returns a CoreConfig with safe production defaults.
func NewCoreConfig(targetURL string, opts ...Option) (*CoreConfig, error) {
	cfg := &CoreConfig{
		TargetURL:           targetURL,
		MaxPages:            DefaultMaxPages,
		MaxDepth:            DefaultMaxDepth,
		Concurrency:         DefaultConcurrency,
		Delay:               DefaultDelay,
		Timeout:             DefaultTimeout,
		MaxRetries:          DefaultMaxRetries,
		RetryDelay:          DefaultRetryDelay,
		ChunkSize:           DefaultChunkSize,
		MaxWorkers:          DefaultMaxWorkers,
		RespectRobotsTxt:    true,
		EnableNormalization: true,
		SinkType:            SinkMemory,
		SinkBuffer:          DefaultSinkBuffer,
		UserAgent:           "Spiderly/1.0 (+https://github.com/spiderly)",
	}

	// Apply functional options
	for _, opt := range opts {
		opt(cfg)
	}

	// Validate after all options applied
	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	// Compile patterns
	if err := cfg.CompilePatterns(); err != nil {
		return nil, err
	}

	return cfg, nil
}

// ─────────────────────────────────────────────
//  Functional Options
// ─────────────────────────────────────────────

// Option is a functional option for CoreConfig.
type Option func(*CoreConfig)

// WithSitemapURL sets a direct sitemap URL to crawl.
func WithSitemapURL(u string) Option {
	return func(c *CoreConfig) { c.SitemapURL = u }
}

// WithMaxPages sets the maximum number of pages to crawl.
func WithMaxPages(n int) Option {
	return func(c *CoreConfig) {
		if n > 0 {
			c.MaxPages = n
		}
	}
}

// WithMaxDepth sets the maximum crawl depth for recursive mode.
func WithMaxDepth(n int) Option {
	return func(c *CoreConfig) {
		if n > 0 {
			c.MaxDepth = n
		}
	}
}

// WithConcurrency sets the number of concurrent crawl workers.
func WithConcurrency(n int) Option {
	return func(c *CoreConfig) {
		if n > 0 {
			c.Concurrency = n
		}
	}
}

// WithDelay sets the delay between requests per worker.
func WithDelay(d time.Duration) Option {
	return func(c *CoreConfig) {
		if d >= 0 {
			c.Delay = d
		}
	}
}

// WithTimeout sets the HTTP request timeout.
func WithTimeout(d time.Duration) Option {
	return func(c *CoreConfig) {
		if d > 0 {
			c.Timeout = d
		}
	}
}

// WithRetry configures retry behavior.
func WithRetry(maxRetries int, retryDelay time.Duration) Option {
	return func(c *CoreConfig) {
		if maxRetries >= 0 {
			c.MaxRetries = maxRetries
		}
		if retryDelay > 0 {
			c.RetryDelay = retryDelay
		}
	}
}

// WithMinPriority filters sitemap entries below this priority.
func WithMinPriority(p float64) Option {
	return func(c *CoreConfig) {
		if p >= 0 && p <= 1 {
			c.MinPriority = p
		}
	}
}

// WithURLPattern sets a regex pattern to filter URLs.
func WithURLPattern(pattern string) Option {
	return func(c *CoreConfig) { c.URLPattern = pattern }
}

// WithForceRecursive forces recursive crawl mode (skip sitemap).
func WithForceRecursive(v bool) Option {
	return func(c *CoreConfig) { c.ForceRecursive = v }
}

// WithHeadless enables headless browser rendering.
func WithHeadless(v bool) Option {
	return func(c *CoreConfig) { c.Headless = v }
}

// WithVerbose enables verbose logging output.
func WithVerbose(v bool) Option {
	return func(c *CoreConfig) { c.Verbose = v }
}

// WithNoColor disables ANSI color output.
func WithNoColor(v bool) Option {
	return func(c *CoreConfig) { c.NoColor = v }
}

// WithChunker enables chunked crawling for large sitemaps.
func WithChunker(chunkSize, maxWorkers int) Option {
	return func(c *CoreConfig) {
		c.EnableChunker = true
		if chunkSize > 0 {
			c.ChunkSize = chunkSize
		}
		if maxWorkers > 0 {
			c.MaxWorkers = maxWorkers
		}
	}
}

// WithProductMode enables product extraction mode.
func WithProductMode(pattern string, sitemaps []string) Option {
	return func(c *CoreConfig) {
		c.ProductMode = true
		c.ProductPattern = pattern
		if len(sitemaps) > 0 {
			c.ProductSitemaps = sitemaps
		}
	}
}

// WithExtractSpecs enables product specification extraction.
func WithExtractSpecs(v bool) Option {
	return func(c *CoreConfig) { c.ExtractSpecs = v }
}

// WithExtractImages enables product image extraction.
func WithExtractImages(v bool) Option {
	return func(c *CoreConfig) { c.ExtractImages = v }
}

// WithExcludePatterns sets URL patterns to exclude from crawling.
func WithExcludePatterns(patterns []string) Option {
	return func(c *CoreConfig) { c.ExcludePatterns = patterns }
}

// WithRobotsTxt controls robots.txt compliance.
func WithRobotsTxt(respect bool) Option {
	return func(c *CoreConfig) { c.RespectRobotsTxt = respect }
}

// WithUserAgent sets the crawler's User-Agent string.
func WithUserAgent(ua string) Option {
	return func(c *CoreConfig) {
		if ua != "" {
			c.UserAgent = ua
		}
	}
}

// WithNormalization controls URL normalization.
func WithNormalization(v bool) Option {
	return func(c *CoreConfig) { c.EnableNormalization = v }
}

// WithSink configures the result sink type and optional path.
func WithSink(sinkType SinkType, path string) Option {
	return func(c *CoreConfig) {
		c.SinkType = sinkType
		c.SinkPath = path
	}
}

// WithSinkBuffer sets the channel buffer size for streaming sinks.
func WithSinkBuffer(n int) Option {
	return func(c *CoreConfig) {
		if n > 0 {
			c.SinkBuffer = n
		}
	}
}

// ─────────────────────────────────────────────
//  Validation
// ─────────────────────────────────────────────

// Validate checks the CoreConfig for logical errors.
func (cfg *CoreConfig) Validate() error {
	// Must have at least one target
	if cfg.TargetURL == "" && cfg.SitemapURL == "" {
		return NewConfigError("target_url", "no target URL or sitemap URL specified")
	}

	// Validate target URL format
	if cfg.TargetURL != "" {
		normalized := cfg.TargetURL
		if !strings.HasPrefix(normalized, "http://") && !strings.HasPrefix(normalized, "https://") {
			normalized = "https://" + normalized
		}
		parsed, err := url.Parse(normalized)
		if err != nil {
			return NewConfigError("target_url", fmt.Sprintf("invalid URL: %v", err))
		}
		if parsed.Host == "" {
			return NewConfigError("target_url", "URL must have a valid host")
		}
		// Store normalized version back
		cfg.TargetURL = normalized
	}

	// Validate sitemap URL if provided
	if cfg.SitemapURL != "" {
		parsed, err := url.Parse(cfg.SitemapURL)
		if err != nil {
			return NewConfigError("sitemap_url", fmt.Sprintf("invalid sitemap URL: %v", err))
		}
		if parsed.Scheme == "" || parsed.Host == "" {
			return NewConfigError("sitemap_url", "sitemap URL must be absolute (include scheme and host)")
		}
	}

	// Numeric bounds
	if cfg.MaxPages <= 0 {
		return NewConfigError("max_pages", "must be > 0")
	}
	if cfg.MaxDepth <= 0 {
		return NewConfigError("max_depth", "must be > 0")
	}
	if cfg.Concurrency <= 0 {
		return NewConfigError("concurrency", "must be > 0")
	}
	if cfg.Concurrency > 100 {
		return NewConfigError("concurrency", "must be <= 100 (safety limit)")
	}
	if cfg.Timeout <= 0 {
		return NewConfigError("timeout", "must be > 0")
	}
	if cfg.Delay < 0 {
		return NewConfigError("delay", "must be >= 0")
	}

	// Retry bounds
	if cfg.MaxRetries < 0 {
		return NewConfigError("max_retries", "must be >= 0")
	}
	if cfg.MaxRetries > 10 {
		return NewConfigError("max_retries", "must be <= 10 (safety limit)")
	}

	// Chunker bounds
	if cfg.EnableChunker {
		if cfg.ChunkSize <= 0 {
			return NewConfigError("chunk_size", "must be > 0 when chunker is enabled")
		}
		if cfg.MaxWorkers <= 0 {
			return NewConfigError("max_workers", "must be > 0 when chunker is enabled")
		}
	}

	// MinPriority range
	if cfg.MinPriority < 0 || cfg.MinPriority > 1 {
		return NewConfigError("min_priority", "must be between 0.0 and 1.0")
	}

	// Sink validation
	if cfg.SinkType == SinkFile && cfg.SinkPath == "" {
		return NewConfigError("sink_path", "file sink requires a path")
	}

	return nil
}

// ─────────────────────────────────────────────
//  Pattern Compilation
// ─────────────────────────────────────────────

// CompilePatterns pre-compiles all regex patterns in the config.
// Called automatically by NewCoreConfig after validation.
func (cfg *CoreConfig) CompilePatterns() error {
	// Compile URL pattern
	if cfg.URLPattern != "" {
		re, err := regexp.Compile(cfg.URLPattern)
		if err != nil {
			return NewConfigError("url_pattern", fmt.Sprintf("invalid regex: %v", err))
		}
		cfg.CompiledURLPattern = re
	}

	// Compile product pattern
	if cfg.ProductPattern != "" {
		re, err := regexp.Compile(cfg.ProductPattern)
		if err != nil {
			return NewConfigError("product_pattern", fmt.Sprintf("invalid regex: %v", err))
		}
		cfg.CompiledProductPattern = re
	}

	// Compile exclude patterns
	if len(cfg.ExcludePatterns) > 0 {
		compiled, err := exclude.CompilePatterns(cfg.ExcludePatterns)
		if err != nil {
			return NewConfigError("exclude_patterns", fmt.Sprintf("invalid regex: %v", err))
		}
		cfg.CompiledExcludePatterns = compiled
	}

	return nil
}

// ─────────────────────────────────────────────
//  Config Helpers
// ─────────────────────────────────────────────

// BaseURL extracts the base URL (scheme + host) from the config.
// Prefers SitemapURL if TargetURL is empty.
func (cfg *CoreConfig) BaseURL() (string, error) {
	raw := cfg.TargetURL
	if raw == "" {
		raw = cfg.SitemapURL
	}
	if raw == "" {
		return "", NewConfigError("target_url", "no URL configured")
	}

	if !strings.HasPrefix(raw, "http://") && !strings.HasPrefix(raw, "https://") {
		raw = "https://" + raw
	}

	parsed, err := url.Parse(raw)
	if err != nil {
		return "", fmt.Errorf("parse base URL: %w", err)
	}

	return fmt.Sprintf("%s://%s", parsed.Scheme, parsed.Host), nil
}

// Clone returns a deep copy of the config (compiled patterns are shared).
func (cfg *CoreConfig) Clone() CoreConfig {
	clone := *cfg

	// Deep copy slices
	if cfg.ProductSitemaps != nil {
		clone.ProductSitemaps = make([]string, len(cfg.ProductSitemaps))
		copy(clone.ProductSitemaps, cfg.ProductSitemaps)
	}
	if cfg.ExcludePatterns != nil {
		clone.ExcludePatterns = make([]string, len(cfg.ExcludePatterns))
		copy(clone.ExcludePatterns, cfg.ExcludePatterns)
	}
	// Compiled patterns are safe to share (regexp is goroutine-safe)

	return clone
}

// String returns a human-readable summary of the config.
func (cfg *CoreConfig) String() string {
	var b strings.Builder
	b.WriteString("CoreConfig{\n")
	fmt.Fprintf(&b, "  Target:      %s\n", cfg.TargetURL)
	if cfg.SitemapURL != "" {
		fmt.Fprintf(&b, "  Sitemap:     %s\n", cfg.SitemapURL)
	}
	fmt.Fprintf(&b, "  MaxPages:    %d\n", cfg.MaxPages)
	fmt.Fprintf(&b, "  MaxDepth:    %d\n", cfg.MaxDepth)
	fmt.Fprintf(&b, "  Concurrency: %d\n", cfg.Concurrency)
	fmt.Fprintf(&b, "  Delay:       %s\n", cfg.Delay)
	fmt.Fprintf(&b, "  Timeout:     %s\n", cfg.Timeout)
	fmt.Fprintf(&b, "  Retries:     %d (delay: %s)\n", cfg.MaxRetries, cfg.RetryDelay)
	fmt.Fprintf(&b, "  RobotsTxt:   %v\n", cfg.RespectRobotsTxt)
	fmt.Fprintf(&b, "  Normalize:   %v\n", cfg.EnableNormalization)
	fmt.Fprintf(&b, "  Sink:        %s\n", cfg.SinkType)
	if cfg.ProductMode {
		fmt.Fprintf(&b, "  ProductMode: true\n")
		if cfg.ProductPattern != "" {
			fmt.Fprintf(&b, "  ProductPat:  %s\n", cfg.ProductPattern)
		}
	}
	if cfg.EnableChunker {
		fmt.Fprintf(&b, "  Chunker:     %d chunks × %d workers\n", cfg.ChunkSize, cfg.MaxWorkers)
	}
	b.WriteString("}")
	return b.String()
}
