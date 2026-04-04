// internal/crawler/config.go
package crawler

import (
	"fmt"
	"net/url"
	"regexp"
	"time"

	"spiderly/internal/exclude"
)

// Config holds crawler configuration.
type Config struct {
	MaxPages      int
	MaxDepth      int
	Concurrency   int
	Delay         time.Duration
	Timeout       time.Duration
	UserAgent     string
	Verbose       bool
	SitemapMode   bool
	RespectRobots bool
	Proxies       []string

	// Product extraction
	ProductMode    bool
	ProductPattern *regexp.Regexp
	ExtractSpecs   bool
	ExtractImages  bool
	NewsMode       bool
	NewsPattern    *regexp.Regexp

	// URL filtering
	ExcludePatterns         []string
	CompiledExcludePatterns []*regexp.Regexp
}

// DefaultConfig returns sensible defaults.
func DefaultConfig() Config {
	return Config{
		MaxPages:      100,
		MaxDepth:      3,
		Concurrency:   5,
		Delay:         200 * time.Millisecond,
		Timeout:       30 * time.Second,
		UserAgent:     "Spiderly/1.0",
		Verbose:       false,
		SitemapMode:   false,
		RespectRobots: true,
	}
}

// Validate checks the config for logical errors and compiles exclude patterns.
func (cfg *Config) Validate() error {
	if cfg.MaxPages < 0 {
		return fmt.Errorf("MaxPages must be >= 0, got %d", cfg.MaxPages)
	}
	if cfg.MaxDepth < 0 {
		return fmt.Errorf("MaxDepth must be >= 0, got %d", cfg.MaxDepth)
	}
	if cfg.Concurrency < 1 {
		return fmt.Errorf("Concurrency must be >= 1, got %d", cfg.Concurrency)
	}
	if cfg.Timeout <= 0 {
		return fmt.Errorf("Timeout must be > 0, got %v", cfg.Timeout)
	}
	if cfg.Delay < 0 {
		return fmt.Errorf("Delay must be >= 0, got %v", cfg.Delay)
	}

	if cfg.ProductMode && cfg.NewsMode {
		return fmt.Errorf("ProductMode and NewsMode cannot both be enabled")
	}

	for _, rawProxy := range cfg.Proxies {
		parsed, err := url.Parse(rawProxy)
		if err != nil {
			return fmt.Errorf("invalid proxy URL %q: %w", rawProxy, err)
		}
		if parsed.Scheme == "" || parsed.Host == "" {
			return fmt.Errorf("proxy URL must include scheme and host: %q", rawProxy)
		}
	}

	// Compile exclude patterns once
	if len(cfg.ExcludePatterns) > 0 && len(cfg.CompiledExcludePatterns) == 0 {
		compiled, err := exclude.CompilePatterns(cfg.ExcludePatterns)
		if err != nil {
			return fmt.Errorf("invalid exclude pattern: %w", err)
		}
		cfg.CompiledExcludePatterns = compiled
	}

	return nil
}
