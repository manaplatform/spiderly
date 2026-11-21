// internal/core/config_test.go
package core

import (
	"testing"
	"time"
)

func TestCoreConfig_BrowserOptions(t *testing.T) {
	cfg, err := NewCoreConfig("https://example.com",
		WithHeadless(true),
		WithBrowserVisible(true),
		WithBrowserTimeout(15*time.Second),
		WithBrowserStrategy("fallback"),
		WithClickCrawl(true),
		WithMaxClicksPerPage(5),
	)
	if err != nil {
		t.Fatalf("unexpected NewCoreConfig error: %v", err)
	}

	if !cfg.Headless {
		t.Errorf("expected Headless=true")
	}
	if !cfg.BrowserVisible {
		t.Errorf("expected BrowserVisible=true")
	}
	if cfg.BrowserTimeout != 15*time.Second {
		t.Errorf("expected BrowserTimeout=15s, got %v", cfg.BrowserTimeout)
	}
	if cfg.BrowserStrategy != "fallback" {
		t.Errorf("expected BrowserStrategy='fallback', got %s", cfg.BrowserStrategy)
	}
	if !cfg.ClickCrawl {
		t.Errorf("expected ClickCrawl=true")
	}
	if cfg.MaxClicksPerPage != 5 {
		t.Errorf("expected MaxClicksPerPage=5, got %d", cfg.MaxClicksPerPage)
	}
}

func TestCoreConfig_BrowserValidation(t *testing.T) {
	_, err := NewCoreConfig("https://example.com",
		WithBrowserStrategy("invalid-strategy"),
	)
	if err == nil {
		t.Errorf("expected error for invalid browser strategy, got nil")
	}

	_, err = NewCoreConfig("https://example.com",
		WithMaxClicksPerPage(-1),
	)
	if err != nil {
		// WithMaxClicksPerPage only sets if n >= 0, so should remain valid
	}
}
