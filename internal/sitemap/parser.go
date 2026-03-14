package sitemap

import (
	"context"
	"fmt"
	"log"
	"net"
	"net/http"
	"time"
)

const (
	// MaxRecursionDepth prevents infinite loops in malformed/cyclic sitemap indices.
	MaxRecursionDepth = 5

	// MaxResponseSize caps the maximum bytes read from a single sitemap response (50 MB).
	MaxResponseSize = 50 * 1024 * 1024

	// DefaultTimeout for HTTP requests.
	DefaultTimeout = 30 * time.Second

	// DefaultMaxRedirects caps redirect chains.
	DefaultMaxRedirects = 5

	// DefaultConcurrency for parallel child-sitemap fetches.
	DefaultConcurrency = 10

	// UserAgent sent with every request.
	UserAgent = "Spiderly/1.0 (+https://github.com/spiderly)"
)

// Option configures a Parser.
type Option func(*Parser)

// WithTimeout sets the HTTP client timeout.
func WithTimeout(d time.Duration) Option {
	return func(p *Parser) { p.timeout = d }
}

// WithConcurrency sets max parallel fetches during discovery/parsing.
func WithConcurrency(n int) Option {
	if n < 1 {
		n = 1
	}
	return func(p *Parser) { p.concurrency = n }
}

// WithVerbose enables debug logging.
func WithVerbose(v bool) Option {
	return func(p *Parser) { p.verbose = v }
}

// WithMaxResponseSize overrides the per-response size cap.
func WithMaxResponseSize(n int64) Option {
	return func(p *Parser) { p.maxResponseSize = n }
}

// Parser handles sitemap discovery, fetching, and parsing.
type Parser struct {
	client          *http.Client
	timeout         time.Duration
	concurrency     int
	maxResponseSize int64
	verbose         bool
	maxDepth    int 
}

// NewParser creates a Parser with sensible defaults, overridden by any provided options.
func NewParser(opts ...Option) *Parser {
	p := &Parser{
		timeout:         DefaultTimeout,
		concurrency:     DefaultConcurrency,
		maxResponseSize: MaxResponseSize,
		verbose:         false,
	}

	for _, opt := range opts {
		opt(p)
	}

	transport := &http.Transport{
		MaxIdleConnsPerHost: p.concurrency, // match concurrency to avoid socket churn
		IdleConnTimeout:     90 * time.Second,
		DisableCompression:  true, // we handle gzip ourselves to avoid double-decompress
		DialContext: (&net.Dialer{
			Timeout:   10 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext,
	}

	p.client = &http.Client{
		Timeout:   p.timeout,
		Transport: transport,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= DefaultMaxRedirects {
				return fmt.Errorf("stopped after %d redirects", DefaultMaxRedirects)
			}
			return nil
		},
	}

	return p
}

// newRequest builds an *http.Request with standard headers and the given context.
func (p *Parser) newRequest(ctx context.Context, method, url string) (*http.Request, error) {
	req, err := http.NewRequestWithContext(ctx, method, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", UserAgent)
	req.Header.Set("Accept", "application/xml, text/xml, application/gzip, */*")
	return req, nil
}

// logVerbose prints a debug message when verbose mode is on.
func (p *Parser) logVerbose(format string, args ...interface{}) {
	if p.verbose {
		log.Printf("[SITEMAP] "+format, args...)
	}
}
