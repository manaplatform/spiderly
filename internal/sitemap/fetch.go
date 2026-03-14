package sitemap

import (
	"bytes"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const (
	// maxRetries for transient HTTP failures.
	maxRetries = 3

	// baseBackoff is the initial wait between retries.
	baseBackoff = 500 * time.Millisecond
)

// fetchResult holds raw bytes and metadata from a fetch.
type fetchResult struct {
	Data        []byte
	ContentType string
	StatusCode  int
}

// fetch retrieves a URL with retries, size limiting, and transparent gzip handling.
// It returns the decompressed body bytes ready for XML parsing.
func (p *Parser) fetch(ctx context.Context, targetURL string) ([]byte, error) {
	result, err := p.fetchWithRetry(ctx, http.MethodGet, targetURL)
	if err != nil {
		return nil, err
	}

	return p.decompress(result.Data, result.ContentType, targetURL)
}

// exists checks whether a sitemap URL is reachable and looks like valid sitemap content.
// Uses HEAD first, falls back to a small-body GET if HEAD fails or returns ambiguous results.
func (p *Parser) exists(ctx context.Context, targetURL string) bool {
	// Try HEAD first — cheap, no body transfer
	result, err := p.doRequest(ctx, http.MethodHead, targetURL)
	if err == nil && result.StatusCode == http.StatusOK && p.looksLikeSitemap(result.ContentType, targetURL) {
		return true
	}

	// HEAD failed or was ambiguous — fall back to a tiny GET to peek at content type
	result, err = p.doRequest(ctx, http.MethodGet, targetURL)
	if err != nil {
		return false
	}

	return result.StatusCode == http.StatusOK && p.looksLikeSitemap(result.ContentType, targetURL)
}

// looksLikeSitemap returns true if the content type or URL suffix suggests sitemap content.
func (p *Parser) looksLikeSitemap(contentType, targetURL string) bool {
	ct := strings.ToLower(contentType)
	urlLower := strings.ToLower(targetURL)

	return strings.Contains(ct, "xml") ||
		strings.Contains(ct, "text/plain") ||
		strings.Contains(ct, "gzip") ||
		strings.HasSuffix(urlLower, ".xml") ||
		strings.HasSuffix(urlLower, ".xml.gz") ||
		strings.HasSuffix(urlLower, ".gz")
}

// ─────────────────────────────────────────────
//  Retry Logic
// ─────────────────────────────────────────────

// fetchWithRetry wraps doRequest with exponential backoff for transient errors.
func (p *Parser) fetchWithRetry(ctx context.Context, method, targetURL string) (*fetchResult, error) {
	var lastErr error

	for attempt := 0; attempt <= maxRetries; attempt++ {
		if attempt > 0 {
			backoff := baseBackoff * time.Duration(1<<(attempt-1)) // 500ms, 1s, 2s
			p.logVerbose("Retry %d/%d for %s (waiting %v)", attempt, maxRetries, targetURL, backoff)

			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(backoff):
			}
		}

		result, err := p.doRequest(ctx, method, targetURL)
		if err != nil {
			lastErr = err
			continue
		}

		// Non-retryable client errors (4xx except 429)
		if result.StatusCode >= 400 && result.StatusCode < 500 && result.StatusCode != http.StatusTooManyRequests {
			return nil, fmt.Errorf("HTTP %d for %s", result.StatusCode, targetURL)
		}

		// Retryable: 429, 5xx
		if result.StatusCode == http.StatusTooManyRequests || result.StatusCode >= 500 {
			lastErr = fmt.Errorf("HTTP %d for %s", result.StatusCode, targetURL)
			continue
		}

		// Success
		if result.StatusCode == http.StatusOK {
			return result, nil
		}

		// Any other status — don't retry
		return nil, fmt.Errorf("unexpected HTTP %d for %s", result.StatusCode, targetURL)
	}

	return nil, fmt.Errorf("all %d retries exhausted for %s: %w", maxRetries, targetURL, lastErr)
}

// ─────────────────────────────────────────────
//  Core HTTP Request
// ─────────────────────────────────────────────

// doRequest performs a single HTTP request with size-limited body reading.
func (p *Parser) doRequest(ctx context.Context, method, targetURL string) (*fetchResult, error) {
	req, err := p.newRequest(ctx, method, targetURL)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	result := &fetchResult{
		ContentType: resp.Header.Get("Content-Type"),
		StatusCode:  resp.StatusCode,
	}

	// Only read body for GET requests with success status
	if method == http.MethodGet && resp.StatusCode == http.StatusOK {
		// Size-limited read: prevents OOM from absurdly large responses
		limited := io.LimitReader(resp.Body, p.maxResponseSize+1)
		data, err := io.ReadAll(limited)
		if err != nil {
			return nil, fmt.Errorf("failed to read response body: %w", err)
		}

		if int64(len(data)) > p.maxResponseSize {
			return nil, fmt.Errorf("response exceeds max size (%d bytes) for %s", p.maxResponseSize, targetURL)
		}

		result.Data = data
	} else {
		// Drain body to allow connection reuse
		_, _ = io.Copy(io.Discard, resp.Body)
	}

	return result, nil
}

// ─────────────────────────────────────────────
//  Gzip Decompression
// ─────────────────────────────────────────────

// decompress transparently handles gzip content.
// Detection uses three signals: Content-Type header, URL suffix, and magic bytes.
// Note: Content-Encoding is NOT checked because we set DisableCompression: true
// on the transport, so the server won't apply transfer-level gzip.
func (p *Parser) decompress(data []byte, contentType, sourceURL string) ([]byte, error) {
	if !p.isGzipped(data, contentType, sourceURL) {
		return data, nil
	}

	p.logVerbose("Decompressing gzip data (%d bytes) from %s", len(data), sourceURL)

	gzReader, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("failed to create gzip reader for %s: %w", sourceURL, err)
	}
	defer gzReader.Close()

	// Size-limited decompression to prevent gzip bombs
	limited := io.LimitReader(gzReader, p.maxResponseSize+1)
	decompressed, err := io.ReadAll(limited)
	if err != nil {
		return nil, fmt.Errorf("failed to decompress gzip from %s: %w", sourceURL, err)
	}

	if int64(len(decompressed)) > p.maxResponseSize {
		return nil, fmt.Errorf("decompressed data exceeds max size (%d bytes) for %s", p.maxResponseSize, sourceURL)
	}

	p.logVerbose("Decompressed %d → %d bytes from %s", len(data), len(decompressed), sourceURL)
	return decompressed, nil
}

// isGzipped detects gzip content using three methods.
// Content-Encoding is intentionally excluded — DisableCompression on the transport
// means the server sends raw bytes, so Content-Encoding won't be "gzip" for
// transfer-level compression. We only care about content-level gzip (.gz files).
func (p *Parser) isGzipped(data []byte, contentType, sourceURL string) bool {
	// Method 1: Magic bytes (most reliable — gzip always starts with 0x1f 0x8b)
	if len(data) >= 2 && data[0] == 0x1f && data[1] == 0x8b {
		return true
	}

	// Method 2: Content-Type header
	ct := strings.ToLower(contentType)
	if strings.Contains(ct, "gzip") || strings.Contains(ct, "x-gzip") {
		return true
	}

	// Method 3: URL suffix
	if strings.HasSuffix(strings.ToLower(sourceURL), ".gz") {
		return true
	}

	return false
}
