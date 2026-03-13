// internal/core/sinks.go
package core

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sync"
	"time"

	"spiderly/internal/models"
)

// ─────────────────────────────────────────────
//  Sink — Streaming output interface
// ─────────────────────────────────────────────

// Sink is a streaming consumer of scraped pages.
// Implementations can write to files, databases, message queues, etc.
// The Core pushes each page to all registered sinks as soon as it's scraped,
// so large crawls never need to buffer everything in memory.
type Sink interface {
	// Name returns a human-readable identifier (used in log messages).
	Name() string

	// Open prepares the sink for writing (create file, open connection, etc.).
	Open() error

	// Write sends a single scraped page to the sink.
	Write(page models.ScrapedPage) error

	// Close flushes any buffers and releases resources.
	Close() error
}

// ─────────────────────────────────────────────
//  ResultSink Interface
// ─────────────────────────────────────────────

// ResultSink receives crawl results one at a time.
// Implementations MUST be safe for concurrent use.
type ResultSink interface {
	// Open prepares the sink (create file, write header, etc.).
	Open() error

	// Write sends a single result to the sink.
	Write(page models.ScrapedPage) error

	// Flush ensures all buffered data is persisted.
	Flush() error

	// Close flushes and releases resources.
	Close() error

	// Count returns the number of results written so far.
	Count() int
}

// ─────────────────────────────────────────────
//  JSONL Sink  (one JSON object per line)
// ─────────────────────────────────────────────

// JSONLSink streams results as newline-delimited JSON.
type JSONLSink struct {
	path    string
	file    *os.File
	encoder *json.Encoder
	mu      sync.Mutex
	count   int
	pretty  bool // indent for debugging
}

// JSONLSinkOption configures a JSONLSink.
type JSONLSinkOption func(*JSONLSink)

// WithPrettyJSON enables indented output (useful for debugging, not for large crawls).
func WithPrettyJSON() JSONLSinkOption {
	return func(s *JSONLSink) { s.pretty = true }
}

// NewJSONLSink creates a sink that writes to the given file path.
func NewJSONLSink(path string, opts ...JSONLSinkOption) *JSONLSink {
	s := &JSONLSink{path: path}
	for _, o := range opts {
		o(s)
	}
	return s
}

func (s *JSONLSink) Open() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	f, err := os.Create(s.path)
	if err != nil {
		return &CrawlError{
			Type:      ErrInternal,
			URL:       s.path,
			Message:   "failed to create JSONL file",
			Cause:     err,
			Timestamp: time.Now(),
		}
	}
	s.file = f
	s.encoder = json.NewEncoder(f)
	if s.pretty {
		s.encoder.SetIndent("", "  ")
	}
	s.encoder.SetEscapeHTML(false)
	return nil
}

func (s *JSONLSink) Write(page models.ScrapedPage) error {
	result := ToScrapedPageResult(page)

	s.mu.Lock()
	defer s.mu.Unlock()

	if s.encoder == nil {
		return &CrawlError{
			Type:      ErrInternal,
			URL:       page.URL,
			Message:   "sink not opened",
			Timestamp: time.Now(),
		}
	}
	if err := s.encoder.Encode(result); err != nil {
		return &CrawlError{
			Type:      ErrInternal,
			URL:       page.URL,
			Message:   fmt.Sprintf("JSONL encode failed: %v", err),
			Cause:     err,
			Timestamp: time.Now(),
		}
	}
	s.count++
	return nil
}



func (s *JSONLSink) Flush() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.file != nil {
		return s.file.Sync()
	}
	return nil
}

func (s *JSONLSink) Close() error {
	if err := s.Flush(); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.file != nil {
		err := s.file.Close()
		s.file = nil
		s.encoder = nil
		return err
	}
	return nil
}

func (s *JSONLSink) Count() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.count
}

// ─────────────────────────────────────────────
//  CSV Sink
// ─────────────────────────────────────────────

// csvHeaders defines the column order for the CSV output.
var csvHeaders = []string{
	"url",
	"title",
	"h1",
	"description",
	"status_code",
	"content_type",
	"content_length",
	"load_time_ms",
	"links_count",
	"images_count",
	"depth",
	"page_type",
	"scraped_at",
	// Product fields (empty when not in product mode)
	"product_name",
	"product_brand",
	"product_sku",
	"product_price",
	"product_currency",
	"product_availability",
	"product_in_stock",
	"product_rating",
	"product_review_count",
	"product_category",
}

// CSVSink streams results as comma-separated values.
type CSVSink struct {
	path   string
	file   *os.File
	writer *csv.Writer
	mu     sync.Mutex
	count  int
}

// NewCSVSink creates a sink that writes CSV to the given file path.
func NewCSVSink(path string) *CSVSink {
	return &CSVSink{path: path}
}

func (s *CSVSink) Open() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	f, err := os.Create(s.path)
	if err != nil {
		return &CrawlError{
			Type:      ErrInternal,
			URL:       s.path,
			Message:   "failed to create CSV file",
			Cause:     err,
			Timestamp: time.Now(),
		}
	}
	s.file = f
	s.writer = csv.NewWriter(f)

	// Write header row
	if err := s.writer.Write(csvHeaders); err != nil {
		return &CrawlError{
			Type:      ErrInternal,
			URL:       s.path,
			Message:   "failed to write CSV header",
			Cause:     err,
			Timestamp: time.Now(),
		}
	}
	return nil
}

func (s *CSVSink) Write(page models.ScrapedPage) error {
	result := ToScrapedPageResult(page)

	s.mu.Lock()
	defer s.mu.Unlock()

	if s.writer == nil {
		return &CrawlError{
			Type:      ErrInternal,
			URL:       page.URL,
			Message:   "sink not opened",
			Timestamp: time.Now(),
		}
	}

	row := s.resultToRow(result)
	if err := s.writer.Write(row); err != nil {
		return &CrawlError{
			Type:      ErrInternal,
			URL:       page.URL,
			Message:   fmt.Sprintf("CSV write failed: %v", err),
			Cause:     err,
			Timestamp: time.Now(),
		}
	}
	s.count++

	// Flush every 100 rows to keep memory bounded
	if s.count%100 == 0 {
		s.writer.Flush()
	}
	return nil
}

func (s *CSVSink) Flush() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.writer != nil {
		s.writer.Flush()
		return s.writer.Error()
	}
	return nil
}

func (s *CSVSink) Close() error {
	if err := s.Flush(); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.file != nil {
		err := s.file.Close()
		s.file = nil
		s.writer = nil
		return err
	}
	return nil
}

func (s *CSVSink) Count() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.count
}

// resultToRow converts a ScrapedPageResult to a CSV string slice.
func (s *CSVSink) resultToRow(r ScrapedPageResult) []string {
	row := []string{
		r.URL,
		r.Title,
		r.H1,
		r.Description,
		fmt.Sprintf("%d", r.StatusCode),
		r.ContentType,
		fmt.Sprintf("%d", r.ContentLength),
		fmt.Sprintf("%d", r.LoadTimeMs),
		fmt.Sprintf("%d", r.LinksCount),
		fmt.Sprintf("%d", r.ImagesCount),
		fmt.Sprintf("%d", r.Depth),
		r.PageType,
		r.ScrapedAt.Format(time.RFC3339),
	}

	// Product columns
	if r.Product != nil {
		p := r.Product
		row = append(row,
			p.Name,
			p.Brand,
			p.SKU,
			fmt.Sprintf("%.2f", p.Price),
			p.Currency,
			p.Availability,
			fmt.Sprintf("%t", p.InStock),
			fmt.Sprintf("%.1f", p.Rating),
			fmt.Sprintf("%d", p.ReviewCount),
			p.Category,
		)
	} else {
		// Empty product columns
		row = append(row, "", "", "", "", "", "", "", "", "", "")
	}

	return row
}

// ─────────────────────────────────────────────
//  Writer Sink  (generic io.Writer adapter)
// ─────────────────────────────────────────────

// WriterSink wraps any io.Writer (stdout, buffer, network conn) as a JSONL sink.
type WriterSink struct {
	w       io.Writer
	encoder *json.Encoder
	mu      sync.Mutex
	count   int
}

// NewWriterSink creates a JSONL sink that writes to w.
// Useful for piping to stdout: NewWriterSink(os.Stdout)
func NewWriterSink(w io.Writer) *WriterSink {
	return &WriterSink{w: w}
}

func (s *WriterSink) Open() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.encoder = json.NewEncoder(s.w)
	s.encoder.SetEscapeHTML(false)
	return nil
}

func (s *WriterSink) Write(page models.ScrapedPage) error {
	result := ToScrapedPageResult(page)

	s.mu.Lock()
	defer s.mu.Unlock()

	if s.encoder == nil {
		return &CrawlError{
			Type:      ErrInternal,
			URL:       page.URL,
			Message:   "sink not opened",
			Timestamp: time.Now(),
		}
	}
	if err := s.encoder.Encode(result); err != nil {
		return &CrawlError{
			Type:      ErrInternal,
			URL:       page.URL,
			Message:   fmt.Sprintf("encode failed: %v", err),
			Cause:     err,
			Timestamp: time.Now(),
		}
	}
	s.count++
	return nil
}

func (s *WriterSink) Flush() error {
	// If the underlying writer is a Flusher (e.g. bufio.Writer), flush it.
	s.mu.Lock()
	defer s.mu.Unlock()
	if f, ok := s.w.(interface{ Flush() error }); ok {
		return f.Flush()
	}
	return nil
}

func (s *WriterSink) Close() error {
	return s.Flush()
}

func (s *WriterSink) Count() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.count
}

// ─────────────────────────────────────────────
//  Multi Sink  (fan-out to N sinks)
// ─────────────────────────────────────────────

// MultiSink fans out every Write to all child sinks.
// If any child returns an error, it is collected but does not stop others.
type MultiSink struct {
	sinks []ResultSink
	mu    sync.Mutex
	count int
}

// NewMultiSink creates a sink that writes to all provided sinks.
func NewMultiSink(sinks ...ResultSink) *MultiSink {
	return &MultiSink{sinks: sinks}
}

func (m *MultiSink) Open() error {
	var errs []error
	for _, s := range m.sinks {
		if err := s.Open(); err != nil {
			errs = append(errs, err)
		}
	}
	return joinErrors(errs)
}

func (m *MultiSink) Write(page models.ScrapedPage) error {
	var errs []error
	for _, s := range m.sinks {
		if err := s.Write(page); err != nil {
			errs = append(errs, err)
		}
	}
	m.mu.Lock()
	m.count++
	m.mu.Unlock()
	return joinErrors(errs)
}

func (m *MultiSink) Flush() error {
	var errs []error
	for _, s := range m.sinks {
		if err := s.Flush(); err != nil {
			errs = append(errs, err)
		}
	}
	return joinErrors(errs)
}

func (m *MultiSink) Close() error {
	var errs []error
	for _, s := range m.sinks {
		if err := s.Close(); err != nil {
			errs = append(errs, err)
		}
	}
	return joinErrors(errs)
}

func (m *MultiSink) Count() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.count
}

// ─────────────────────────────────────────────
//  Helpers
// ─────────────────────────────────────────────

// joinErrors combines multiple errors into one. Returns nil if the slice is empty.
func joinErrors(errs []error) error {
	if len(errs) == 0 {
		return nil
	}
	if len(errs) == 1 {
		return errs[0]
	}
	msg := fmt.Sprintf("%d sink errors: ", len(errs))
	for i, e := range errs {
		if i > 0 {
			msg += "; "
		}
		msg += e.Error()
	}
	return fmt.Errorf("%s", msg)
}
