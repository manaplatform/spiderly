// internal/browser/browser.go
package browser

import (
	"context"
	"fmt"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/chromedp/cdproto/network"
	"github.com/chromedp/chromedp"
)

// Config defines the options for the ChromeDP browser engine.
type Config struct {
	Headless         bool          // true for headless, false for visible window
	Timeout          time.Duration // request/navigation timeout
	Delay            time.Duration // delay between renders
	Concurrency      int           // max concurrent browser tabs
	UserAgent        string        // custom User-Agent
	Proxies          []string      // proxy URLs
	ClickCrawl       bool          // enable click-based link discovery
	MaxClicksPerPage int           // max interactive elements to click per page
}

// DefaultConfig returns sensible defaults for browser rendering.
func DefaultConfig() Config {
	return Config{
		Headless:         true,
		Timeout:          30 * time.Second,
		Delay:            200 * time.Millisecond,
		Concurrency:      3,
		UserAgent:        "Spiderly/1.0 (+https://github.com/manaplatform/spiderly)",
		MaxClicksPerPage: 10,
	}
}

// PageResult captures the rendered page data and discovered links.
type PageResult struct {
	URL             string
	FinalURL        string
	StatusCode      int
	Title           string
	HTML            string
	ContentLength   int64
	LoadTime        time.Duration
	DiscoveredLinks []string
}

// Engine manages the ChromeDP browser process, contexts, and tab concurrency.
type Engine struct {
	cfg           Config
	allocCtx      context.Context
	allocCancel   context.CancelFunc
	browserCtx    context.Context
	browserCancel context.CancelFunc
	sem           chan struct{}
	mu            sync.Mutex
	closed        bool
	proxyIdx      atomic.Uint64
}

// NewEngine creates and initializes a new ChromeDP browser engine.
func NewEngine(cfg Config) (*Engine, error) {
	if cfg.Concurrency <= 0 {
		cfg.Concurrency = 1
	}
	if cfg.Timeout <= 0 {
		cfg.Timeout = 30 * time.Second
	}
	if cfg.MaxClicksPerPage <= 0 {
		cfg.MaxClicksPerPage = 10
	}

	opts := make([]chromedp.ExecAllocatorOption, 0, len(chromedp.DefaultExecAllocatorOptions)+8)
	opts = append(opts, chromedp.DefaultExecAllocatorOptions[:]...)

	if cfg.Headless {
		opts = append(opts, chromedp.Headless)
	} else {
		opts = append(opts, chromedp.Flag("headless", false))
	}

	opts = append(opts,
		chromedp.NoSandbox,
		chromedp.DisableGPU,
		chromedp.Flag("disable-dev-shm-usage", true),
		chromedp.Flag("disable-background-networking", true),
		chromedp.Flag("disable-default-apps", true),
		chromedp.Flag("disable-extensions", true),
		chromedp.Flag("disable-sync", true),
		chromedp.Flag("disable-translate", true),
		chromedp.Flag("hide-scrollbars", true),
		chromedp.Flag("metrics-recording-only", true),
		chromedp.Flag("mute-audio", true),
		chromedp.Flag("no-first-run", true),
		chromedp.WindowSize(1920, 1080),
	)

	if cfg.UserAgent != "" {
		opts = append(opts, chromedp.UserAgent(cfg.UserAgent))
	}

	if len(cfg.Proxies) > 0 {
		// Use the first configured proxy for the allocator
		proxyURL := cfg.Proxies[0]
		opts = append(opts, chromedp.ProxyServer(proxyURL))
	}

	allocCtx, allocCancel := chromedp.NewExecAllocator(context.Background(), opts...)
	browserCtx, browserCancel := chromedp.NewContext(allocCtx)

	// Start browser process
	if err := chromedp.Run(browserCtx); err != nil {
		browserCancel()
		allocCancel()
		return nil, fmt.Errorf("failed to start ChromeDP browser: %w", err)
	}

	return &Engine{
		cfg:           cfg,
		allocCtx:      allocCtx,
		allocCancel:   allocCancel,
		browserCtx:    browserCtx,
		browserCancel: browserCancel,
		sem:           make(chan struct{}, cfg.Concurrency),
	}, nil
}

// Render navigates to targetURL, renders client-side JS, extracts the DOM and metadata,
// and optionally performs click-based discovery.
func (e *Engine) Render(ctx context.Context, targetURL string) (*PageResult, error) {
	e.mu.Lock()
	if e.closed {
		e.mu.Unlock()
		return nil, fmt.Errorf("browser engine is closed")
	}
	e.mu.Unlock()

	// Acquire concurrency slot
	select {
	case e.sem <- struct{}{}:
		defer func() { <-e.sem }()
	case <-ctx.Done():
		return nil, ctx.Err()
	}

	// Apply politeness delay if configured
	if e.cfg.Delay > 0 {
		select {
		case <-time.After(e.cfg.Delay):
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}

	startTime := time.Now()

	// Create an isolated tab context from the browser context
	tabCtx, tabCancel := chromedp.NewContext(e.browserCtx)
	defer tabCancel()

	timeout := e.cfg.Timeout
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	tabCtx, timeoutCancel := context.WithTimeout(tabCtx, timeout)
	defer timeoutCancel()

	var (
		statusCode    int = 200 // default fallback status
		finalURL      string
		title         string
		html          string
		initialStatus int
	)

	// Listen for the main document's HTTP response status code
	chromedp.ListenTarget(tabCtx, func(ev interface{}) {
		switch res := ev.(type) {
		case *network.EventResponseReceived:
			if res.Type == network.ResourceTypeDocument {
				status := int(res.Response.Status)
				if status > 0 {
					initialStatus = status
				}
			}
		}
	})

	// Run navigation and extraction
	actions := []chromedp.Action{
		network.Enable(),
		chromedp.Navigate(targetURL),
		chromedp.ActionFunc(func(ctx context.Context) error {
			// Wait for body or document ready with fallback
			waitCtx, waitCancel := context.WithTimeout(ctx, 5*time.Second)
			defer waitCancel()
			_ = chromedp.WaitReady("body", chromedp.ByQuery).Do(waitCtx)
			return nil
		}),
		// Short grace period for client-side JS rendering (hydration, state updates)
		chromedp.Sleep(400 * time.Millisecond),
		chromedp.Location(&finalURL),
		chromedp.Title(&title),
		chromedp.OuterHTML("html", &html),
	}

	if err := chromedp.Run(tabCtx, actions...); err != nil {
		return nil, fmt.Errorf("render failed for %s: %w", targetURL, err)
	}

	if initialStatus > 0 {
		statusCode = initialStatus
	}
	if finalURL == "" {
		finalURL = targetURL
	}

	var discoveredLinks []string
	if e.cfg.ClickCrawl {
		// Run click discovery in the tab
		discovered, err := e.discoverClicksInTab(tabCtx, finalURL, e.cfg.MaxClicksPerPage)
		if err == nil && len(discovered) > 0 {
			discoveredLinks = discovered
		}
	}

	loadTime := time.Since(startTime)

	return &PageResult{
		URL:             targetURL,
		FinalURL:        finalURL,
		StatusCode:      statusCode,
		Title:           strings.TrimSpace(title),
		HTML:            html,
		ContentLength:   int64(len(html)),
		LoadTime:        loadTime,
		DiscoveredLinks: discoveredLinks,
	}, nil
}

// Close terminates all active tabs and closes the browser process.
func (e *Engine) Close() error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.closed {
		return nil
	}
	e.closed = true

	if e.browserCancel != nil {
		e.browserCancel()
	}
	if e.allocCancel != nil {
		e.allocCancel()
	}
	return nil
}

// IsClosed returns whether the browser engine has been shut down.
func (e *Engine) IsClosed() bool {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.closed
}

// BaseOrigin returns the scheme and host for a URL string.
func BaseOrigin(rawURL string) string {
	parsed, err := url.Parse(rawURL)
	if err != nil || parsed.Host == "" {
		return ""
	}
	return fmt.Sprintf("%s://%s", parsed.Scheme, parsed.Host)
}
