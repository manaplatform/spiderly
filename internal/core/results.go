// internal/core/results.go
package core

import (
	"encoding/json"
	"fmt"
	"io"
	"sync"
	"time"

	"spiderly/internal/models"
)

// ─────────────────────────────────────────────
//  Exported Result Types
// ─────────────────────────────────────────────

// ProductResult holds product-specific data for export.
type ProductResult struct {
	Name          string            `json:"name,omitempty"`
	Brand         string            `json:"brand,omitempty"`
	SKU           string            `json:"sku,omitempty"`
	GTIN          string            `json:"gtin,omitempty"`
	MPN           string            `json:"mpn,omitempty"`
	Price         float64           `json:"price,omitempty"`
	Currency      string            `json:"currency,omitempty"`
	OriginalPrice float64           `json:"original_price,omitempty"`
	Discount      float64           `json:"discount,omitempty"`
	Availability  string            `json:"availability,omitempty"`
	InStock       bool              `json:"in_stock"`
	Rating        float64           `json:"rating,omitempty"`
	ReviewCount   int               `json:"review_count,omitempty"`
	Category      string            `json:"category,omitempty"`
	Categories    []string          `json:"categories,omitempty"`
	Images        []string          `json:"images,omitempty"`
	Description   string            `json:"description,omitempty"`
	Specs         map[string]string `json:"specs,omitempty"`
}

// ScrapedPageResult is the public-facing result for a single crawled page.
type ScrapedPageResult struct {
	URL           string    `json:"url"`
	Title         string    `json:"title"`
	H1            string    `json:"h1,omitempty"`
	Description   string    `json:"description,omitempty"`
	Keywords      string    `json:"keywords,omitempty"`
	Author        string    `json:"author,omitempty"`
	PublishedDate string    `json:"published_date,omitempty"`
	OGImage       string    `json:"og_image,omitempty"`
	BodyText      string    `json:"body_text,omitempty"`
	StatusCode    int       `json:"status_code"`
	ContentType   string    `json:"content_type,omitempty"`
	ContentLength int64     `json:"content_length"`
	LoadTimeMs    int64     `json:"load_time_ms"`
	LinksCount    int       `json:"links_count"`
	ImagesCount   int       `json:"images_count"`
	Depth         int       `json:"depth"`
	PageType      string    `json:"page_type,omitempty"`
	ScrapedAt     time.Time `json:"scraped_at"`

	// Product data (populated when ProductMode is enabled)
	Product *ProductResult `json:"product,omitempty"`
}

// ─────────────────────────────────────────────
//  Conversion: internal model → export type
// ─────────────────────────────────────────────

// ToScrapedPageResult converts a single internal ScrapedPage to the export type.
func ToScrapedPageResult(p models.ScrapedPage) ScrapedPageResult {
	r := ScrapedPageResult{
		URL:           p.URL,
		Title:         p.Title,
		H1:            p.H1,
		Description:   p.Description,
		Keywords:      p.Keywords,
		Author:        p.Author,
		PublishedDate: p.PublishedDate,
		OGImage:       p.OGImage,
		BodyText:      p.BodyText,
		StatusCode:    p.StatusCode,
		ContentType:   p.ContentType,
		ContentLength: p.ContentLength,
		LoadTimeMs:    p.LoadTimeMs,
		LinksCount:    p.LinksCount,
		ImagesCount:   p.ImagesCount,
		Depth:         p.Depth,
		PageType:      p.PageType,
		ScrapedAt:     p.ScrapedAt,
	}

	if p.Product != nil {
		r.Product = toProductResult(p.Product)
	}

	return r
}

// ToScrapedPageResults converts a slice of internal pages to export results.
func ToScrapedPageResults(pages []models.ScrapedPage) []ScrapedPageResult {
	results := make([]ScrapedPageResult, len(pages))
	for i, p := range pages {
		results[i] = ToScrapedPageResult(p)
	}
	return results
}

// toProductResult maps the internal product model to the export type.
func toProductResult(p *models.ProductData) *ProductResult {
	if p == nil {
		return nil
	}
	return &ProductResult{
		Name:          p.Name,
		Brand:         p.Brand,
		SKU:           p.SKU,
		GTIN:          p.GTIN,
		MPN:           p.MPN,
		Price:         p.Price,
		Currency:      p.Currency,
		OriginalPrice: p.OriginalPrice,
		Discount:      p.Discount,
		Availability:  p.Availability,
		InStock:       p.InStock,
		Rating:        p.Rating,
		ReviewCount:   p.ReviewCount,
		Category:      p.Category,
		Categories:    p.Categories,
		Images:        p.Images,
		Description:   p.Description,
		Specs:         p.Specs,
	}
}

// ─────────────────────────────────────────────
//  Result Collector (thread-safe, bounded)
// ─────────────────────────────────────────────

// ResultCollector accumulates results in memory with an optional cap.
// When a streaming sink is attached, pages are flushed to the sink
// instead of being held in memory indefinitely.
type ResultCollector struct {
	mu          sync.RWMutex
	pages       []models.ScrapedPage
	seen        map[string]struct{} // dedup by normalized URL
	maxCapacity int                 // 0 = unlimited
	sink        ResultSink          // optional streaming sink
	dropped     int64               // pages dropped due to capacity
}

// NewResultCollector creates a collector.
// maxCapacity <= 0 means unlimited in-memory storage.
func NewResultCollector(maxCapacity int) *ResultCollector {
	rc := &ResultCollector{
		pages:       make([]models.ScrapedPage, 0, min(maxCapacity, 1024)),
		seen:        make(map[string]struct{}, 256),
		maxCapacity: maxCapacity,
	}
	return rc
}

// SetSink attaches a streaming sink. When set, each page is written to
// the sink immediately and NOT retained in memory (unless maxCapacity > 0
// is also set, in which case a bounded buffer is kept).
func (rc *ResultCollector) SetSink(s ResultSink) {
	rc.mu.Lock()
	defer rc.mu.Unlock()
	rc.sink = s
}

// Add stores a page if it hasn't been seen before (by normalized URL).
// Returns true if the page was accepted, false if duplicate or dropped.
func (rc *ResultCollector) Add(page models.ScrapedPage) bool {
	normalized := NormalizeURL(page.URL)

	rc.mu.Lock()
	defer rc.mu.Unlock()

	// Dedup check
	if _, exists := rc.seen[normalized]; exists {
		return false
	}
	rc.seen[normalized] = struct{}{}

	// Stream to sink if available
	if rc.sink != nil {
		if err := rc.sink.Write(page); err != nil {
			// Sink write failed — still count as seen but don't store
			rc.dropped++
			return false
		}
		// If we have a sink, only keep in memory if capacity allows
		if rc.maxCapacity <= 0 {
			return true // streamed, don't buffer
		}
	}

	// Capacity check
	if rc.maxCapacity > 0 && len(rc.pages) >= rc.maxCapacity {
		rc.dropped++
		return false
	}

	rc.pages = append(rc.pages, page)
	return true
}

// Results returns a copy of all collected pages.
func (rc *ResultCollector) Results() []models.ScrapedPage {
	rc.mu.RLock()
	defer rc.mu.RUnlock()

	out := make([]models.ScrapedPage, len(rc.pages))
	copy(out, rc.pages)
	return out
}

// Count returns the number of unique pages seen (including streamed).
func (rc *ResultCollector) Count() int {
	rc.mu.RLock()
	defer rc.mu.RUnlock()
	return len(rc.seen)
}

// BufferedCount returns the number of pages held in memory.
func (rc *ResultCollector) BufferedCount() int {
	rc.mu.RLock()
	defer rc.mu.RUnlock()
	return len(rc.pages)
}

// DroppedCount returns pages dropped due to capacity limits or sink errors.
func (rc *ResultCollector) DroppedCount() int64 {
	rc.mu.RLock()
	defer rc.mu.RUnlock()
	return rc.dropped
}

// HasSeen checks if a URL has already been collected.
func (rc *ResultCollector) HasSeen(rawURL string) bool {
	normalized := NormalizeURL(rawURL)
	rc.mu.RLock()
	defer rc.mu.RUnlock()
	_, exists := rc.seen[normalized]
	return exists
}

// Close flushes and closes the attached sink, if any.
func (rc *ResultCollector) Close() error {
	rc.mu.Lock()
	defer rc.mu.Unlock()
	if rc.sink != nil {
		return rc.sink.Close()
	}
	return nil
}


// ─────────────────────────────────────────────
//  Built-in Sinks
// ─────────────────────────────────────────────

// JSONStreamSink writes pages as newline-delimited JSON (NDJSON).
type JSONStreamSink struct {
	mu      sync.Mutex
	writer  io.Writer
	encoder *json.Encoder
	count   int64
}

// NewJSONStreamSink creates a sink that writes NDJSON to the given writer.
func NewJSONStreamSink(w io.Writer) *JSONStreamSink {
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	return &JSONStreamSink{
		writer:  w,
		encoder: enc,
	}
}

func (s *JSONStreamSink) Write(page models.ScrapedPage) error {
	result := ToScrapedPageResult(page)

	s.mu.Lock()
	defer s.mu.Unlock()

	if err := s.encoder.Encode(result); err != nil {
		return &CrawlError{
			Type:      ErrInternal,
			URL:       page.URL,
			Message:   fmt.Sprintf("JSON stream write failed: %v", err),
			Cause:     err,
			Timestamp: time.Now(),
		}
	}
	s.count++
	return nil
}


func (s *JSONStreamSink) Close() error {
	// Flush if writer supports it
	if f, ok := s.writer.(interface{ Flush() error }); ok {
		return f.Flush()
	}
	if c, ok := s.writer.(io.Closer); ok {
		return c.Close()
	}
	return nil
}

// Count returns the number of pages written.
func (s *JSONStreamSink) Count() int64 {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.count
}

// CallbackSink invokes a user function for each page (zero-allocation streaming).
type CallbackSink struct {
	mu sync.Mutex
	fn func(ScrapedPageResult) error
}

// NewCallbackSink creates a sink that calls fn for every page.
func NewCallbackSink(fn func(ScrapedPageResult) error) *CallbackSink {
	return &CallbackSink{fn: fn}
}

func (s *CallbackSink) Write(page models.ScrapedPage) error {
	result := ToScrapedPageResult(page)

	s.mu.Lock()
	defer s.mu.Unlock()

	if err := s.fn(result); err != nil {
		return &CrawlError{
		Type:      ErrInternal,
		URL:       page.URL,
		Message:   fmt.Sprintf("..."),
		Cause:     err,
		Timestamp: time.Now(),
		}
	}
	return nil
}

func (s *CallbackSink) Close() error {
	return nil
}

// NullSink discards all pages (useful for benchmarking or count-only runs).
type NullSink struct {
	mu    sync.Mutex
	count int64
}

func NewNullSink() *NullSink {
	return &NullSink{}
}

func (s *NullSink) Write(_ models.ScrapedPage) error {
	s.mu.Lock()
	s.count++
	s.mu.Unlock()
	return nil
}

func (s *NullSink) Close() error { return nil }

func (s *NullSink) Count() int64 {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.count
}

// ─────────────────────────────────────────────
//  Helpers
// ─────────────────────────────────────────────

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}