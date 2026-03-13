package core

import (
	"fmt"
	"sync"
	"sync/atomic"
	"time"
)

// ─────────────────────────────────────────────
//  Metrics — Thin wrapper over MetricsCollector
// ─────────────────────────────────────────────

// Metrics is the type referenced by Core.metrics.
// It delegates to MetricsCollector and exposes the method names
// that core.go expects (IncrementDuplicatesSkipped, etc.).
type Metrics struct {
	*MetricsCollector
}

// NewMetrics creates a Metrics backed by a fresh MetricsCollector.
func NewMetrics() *Metrics {
	return &Metrics{
		MetricsCollector: NewMetricsCollector(),
	}
}

// IncrementDuplicatesSkipped is the name core.go calls.
// Delegates to MetricsCollector.RecordDuplicate.
func (m *Metrics) IncrementDuplicatesSkipped() {
	m.MetricsCollector.RecordDuplicate()
}

// IncrementRobotsBlocked is the name core.go calls.
// Delegates to MetricsCollector.RecordRobotsBlocked.
func (m *Metrics) IncrementRobotsBlocked() {
	m.MetricsCollector.RecordRobotsBlocked()
}

// Snapshot returns a MetricsSnapshot (same as MetricsCollector.Snapshot).
// Explicitly re-declared so the return type is unambiguous through the wrapper.
func (m *Metrics) Snapshot() MetricsSnapshot {
	return m.MetricsCollector.Snapshot()
}

// ─────────────────────────────────────────────
//  Metrics Collector
// ─────────────────────────────────────────────

// MetricsCollector tracks all crawl metrics in a thread-safe manner.
// It uses atomic operations for hot-path counters and a mutex for
// maps/slices that are read less frequently.
type MetricsCollector struct {
	// ── Atomic counters (hot path) ──
	pagesScraped  atomic.Int64
	pagesSkipped  atomic.Int64
	errorsTotal   atomic.Int64
	retries       atomic.Int64
	bytesReceived atomic.Int64
	linksFound    atomic.Int64
	robotsBlocked atomic.Int64
	duplicates    atomic.Int64

	// ── Latency tracking ──
	latencyMu     sync.Mutex
	latencies     []time.Duration // per-page latencies for percentile calc
	totalLatency  atomic.Int64    // nanoseconds, for fast avg calc

	// ── Status code distribution ──
	statusMu    sync.Mutex
	statusCodes map[int]*atomic.Int64

	// ── Content type distribution ──
	contentMu    sync.Mutex
	contentTypes map[string]*atomic.Int64

	// ── Error classification ──
	errorMu     sync.Mutex
	errorCounts map[ErrorCategory]*atomic.Int64

	// ── Depth distribution ──
	depthMu    sync.Mutex
	depthCounts map[int]*atomic.Int64

	// ── Timing ──
	startTime time.Time
	endTime   time.Time
	started   atomic.Bool
	finished  atomic.Bool

	// ── Rate tracking (sliding window) ──
	rateMu       sync.Mutex
	rateWindow   []time.Time
	windowSize   time.Duration
}

// NewMetricsCollector creates a new metrics collector with sensible defaults.
func NewMetricsCollector() *MetricsCollector {
	return &MetricsCollector{
		latencies:    make([]time.Duration, 0, 1024),
		statusCodes:  make(map[int]*atomic.Int64),
		contentTypes: make(map[string]*atomic.Int64),
		errorCounts:  make(map[ErrorCategory]*atomic.Int64),
		depthCounts:  make(map[int]*atomic.Int64),
		rateWindow:   make([]time.Time, 0, 256),
		windowSize:   30 * time.Second,
	}
}

// ─────────────────────────────────────────────
//  Lifecycle
// ─────────────────────────────────────────────

// Start marks the beginning of metrics collection.
func (m *MetricsCollector) Start() {
	if m.started.CompareAndSwap(false, true) {
		m.startTime = time.Now()
	}
}

// Finish marks the end of metrics collection.
func (m *MetricsCollector) Finish() {
	if m.finished.CompareAndSwap(false, true) {
		m.endTime = time.Now()
	}
}

// Duration returns the elapsed time. If still running, returns time since start.
func (m *MetricsCollector) Duration() time.Duration {
	if !m.started.Load() {
		return 0
	}
	if m.finished.Load() {
		return m.endTime.Sub(m.startTime)
	}
	return time.Since(m.startTime)
}

// ─────────────────────────────────────────────
//  Recording Methods (hot path — lock-free where possible)
// ─────────────────────────────────────────────

// RecordPage records a successfully scraped page.
func (m *MetricsCollector) RecordPage(statusCode int, contentType string, contentLength int64, latency time.Duration, depth int) {
	m.pagesScraped.Add(1)
	m.bytesReceived.Add(contentLength)
	m.totalLatency.Add(int64(latency))

	// Record latency for percentile calculations
	m.latencyMu.Lock()
	m.latencies = append(m.latencies, latency)
	m.latencyMu.Unlock()

	// Status code
	m.recordStatusCode(statusCode)

	// Content type
	if contentType != "" {
		m.recordContentType(contentType)
	}

	// Depth
	m.recordDepth(depth)

	// Rate window
	m.rateMu.Lock()
	m.rateWindow = append(m.rateWindow, time.Now())
	m.rateMu.Unlock()
}

// RecordSkip records a skipped page (duplicate, robots-blocked, etc.).
func (m *MetricsCollector) RecordSkip() {
	m.pagesSkipped.Add(1)
}

// RecordError records a crawl error by category.
func (m *MetricsCollector) RecordError(category ErrorCategory) {
	m.errorsTotal.Add(1)

	m.errorMu.Lock()
	counter, exists := m.errorCounts[category]
	if !exists {
		counter = &atomic.Int64{}
		m.errorCounts[category] = counter
	}
	m.errorMu.Unlock()

	counter.Add(1)
}

// RecordRetry records a retry attempt.
func (m *MetricsCollector) RecordRetry() {
	m.retries.Add(1)
}

// RecordLinksFound records discovered links.
func (m *MetricsCollector) RecordLinksFound(count int) {
	m.linksFound.Add(int64(count))
}

// RecordRobotsBlocked records a URL blocked by robots.txt.
func (m *MetricsCollector) RecordRobotsBlocked() {
	m.robotsBlocked.Add(1)
}

// RecordDuplicate records a duplicate URL that was skipped.
func (m *MetricsCollector) RecordDuplicate() {
	m.duplicates.Add(1)
}

// ─────────────────────────────────────────────
//  Internal recorders (map-based, need locks)
// ─────────────────────────────────────────────

func (m *MetricsCollector) recordStatusCode(code int) {
	m.statusMu.Lock()
	counter, exists := m.statusCodes[code]
	if !exists {
		counter = &atomic.Int64{}
		m.statusCodes[code] = counter
	}
	m.statusMu.Unlock()

	counter.Add(1)
}

func (m *MetricsCollector) recordContentType(ct string) {
	m.contentMu.Lock()
	counter, exists := m.contentTypes[ct]
	if !exists {
		counter = &atomic.Int64{}
		m.contentTypes[ct] = counter
	}
	m.contentMu.Unlock()

	counter.Add(1)
}

func (m *MetricsCollector) recordDepth(depth int) {
	m.depthMu.Lock()
	counter, exists := m.depthCounts[depth]
	if !exists {
		counter = &atomic.Int64{}
		m.depthCounts[depth] = counter
	}
	m.depthMu.Unlock()

	counter.Add(1)
}

// ─────────────────────────────────────────────
//  Read Methods (snapshot-safe)
// ─────────────────────────────────────────────

// PagesScraped returns the total number of successfully scraped pages.
func (m *MetricsCollector) PagesScraped() int64 {
	return m.pagesScraped.Load()
}

// PagesSkipped returns the total number of skipped pages.
func (m *MetricsCollector) PagesSkipped() int64 {
	return m.pagesSkipped.Load()
}

// ErrorsTotal returns the total number of errors.
func (m *MetricsCollector) ErrorsTotal() int64 {
	return m.errorsTotal.Load()
}

// Retries returns the total number of retry attempts.
func (m *MetricsCollector) Retries() int64 {
	return m.retries.Load()
}

// BytesReceived returns the total bytes received.
func (m *MetricsCollector) BytesReceived() int64 {
	return m.bytesReceived.Load()
}

// LinksFound returns the total number of discovered links.
func (m *MetricsCollector) LinksFound() int64 {
	return m.linksFound.Load()
}

// RobotsBlocked returns the number of URLs blocked by robots.txt.
func (m *MetricsCollector) RobotsBlocked() int64 {
	return m.robotsBlocked.Load()
}

// Duplicates returns the number of duplicate URLs skipped.
func (m *MetricsCollector) Duplicates() int64 {
	return m.duplicates.Load()
}

// ─────────────────────────────────────────────
//  Computed Metrics
// ─────────────────────────────────────────────

// PagesPerSecond returns the current crawl rate.
func (m *MetricsCollector) PagesPerSecond() float64 {
	dur := m.Duration().Seconds()
	if dur == 0 {
		return 0
	}
	return float64(m.pagesScraped.Load()) / dur
}

// CurrentRate returns pages/sec over the sliding window for real-time display.
func (m *MetricsCollector) CurrentRate() float64 {
	m.rateMu.Lock()
	defer m.rateMu.Unlock()

	now := time.Now()
	cutoff := now.Add(-m.windowSize)

	// Prune old entries
	start := 0
	for start < len(m.rateWindow) && m.rateWindow[start].Before(cutoff) {
		start++
	}
	m.rateWindow = m.rateWindow[start:]

	if len(m.rateWindow) == 0 {
		return 0
	}

	windowDur := now.Sub(m.rateWindow[0]).Seconds()
	if windowDur == 0 {
		return float64(len(m.rateWindow))
	}
	return float64(len(m.rateWindow)) / windowDur
}

// AvgLatency returns the average page load latency.
func (m *MetricsCollector) AvgLatency() time.Duration {
	pages := m.pagesScraped.Load()
	if pages == 0 {
		return 0
	}
	return time.Duration(m.totalLatency.Load() / pages)
}

// LatencyPercentile returns the p-th percentile latency (p in 0-100).
// This acquires a lock and copies the slice, so use sparingly (e.g., at summary time).
func (m *MetricsCollector) LatencyPercentile(p float64) time.Duration {
	m.latencyMu.Lock()
	n := len(m.latencies)
	if n == 0 {
		m.latencyMu.Unlock()
		return 0
	}

	// Copy to avoid holding lock during sort
	sorted := make([]time.Duration, n)
	copy(sorted, m.latencies)
	m.latencyMu.Unlock()

	// Simple insertion sort is fine for summary-time usage
	for i := 1; i < len(sorted); i++ {
		key := sorted[i]
		j := i - 1
		for j >= 0 && sorted[j] > key {
			sorted[j+1] = sorted[j]
			j--
		}
		sorted[j+1] = key
	}

	idx := int(float64(n-1) * p / 100.0)
	if idx >= n {
		idx = n - 1
	}
	return sorted[idx]
}

// ErrorRate returns the error rate as a percentage (0-100).
func (m *MetricsCollector) ErrorRate() float64 {
	total := m.pagesScraped.Load() + m.errorsTotal.Load()
	if total == 0 {
		return 0
	}
	return float64(m.errorsTotal.Load()) / float64(total) * 100
}

// SuccessRate returns the success rate as a percentage (0-100).
func (m *MetricsCollector) SuccessRate() float64 {
	return 100 - m.ErrorRate()
}

// ─────────────────────────────────────────────
//  Snapshot — Full metrics at a point in time
// ─────────────────────────────────────────────

// MetricsSnapshot is an immutable snapshot of all metrics at a point in time.
type MetricsSnapshot struct {
	// Counters
	PagesScraped  int64 `json:"pages_scraped"`
	PagesSkipped  int64 `json:"pages_skipped"`
	ErrorsTotal   int64 `json:"errors_total"`
	Retries       int64 `json:"retries"`
	BytesReceived int64 `json:"bytes_received"`
	LinksFound    int64 `json:"links_found"`
	RobotsBlocked int64 `json:"robots_blocked"`
	Duplicates    int64 `json:"duplicates"`

	// Computed
	Duration       time.Duration  `json:"duration"`
	PagesPerSecond float64        `json:"pages_per_second"`
	CurrentRate    float64        `json:"current_rate"`
	AvgLatency     time.Duration  `json:"avg_latency"`
	P50Latency     time.Duration  `json:"p50_latency"`
	P95Latency     time.Duration  `json:"p95_latency"`
	P99Latency     time.Duration  `json:"p99_latency"`
	ErrorRate      float64        `json:"error_rate"`
	SuccessRate    float64        `json:"success_rate"`

	// Distributions
	StatusCodes  map[int]int64          `json:"status_codes"`
	ContentTypes map[string]int64       `json:"content_types"`
	ErrorCounts  map[ErrorCategory]int64 `json:"error_counts"`
	DepthCounts  map[int]int64          `json:"depth_counts"`
}

// Snapshot returns an immutable snapshot of all current metrics.
func (m *MetricsCollector) Snapshot() MetricsSnapshot {
	snap := MetricsSnapshot{
		PagesScraped:  m.pagesScraped.Load(),
		PagesSkipped:  m.pagesSkipped.Load(),
		ErrorsTotal:   m.errorsTotal.Load(),
		Retries:       m.retries.Load(),
		BytesReceived: m.bytesReceived.Load(),
		LinksFound:    m.linksFound.Load(),
		RobotsBlocked: m.robotsBlocked.Load(),
		Duplicates:    m.duplicates.Load(),

		Duration:       m.Duration(),
		PagesPerSecond: m.PagesPerSecond(),
		CurrentRate:    m.CurrentRate(),
		AvgLatency:     m.AvgLatency(),
		P50Latency:     m.LatencyPercentile(50),
		P95Latency:     m.LatencyPercentile(95),
		P99Latency:     m.LatencyPercentile(99),
		ErrorRate:      m.ErrorRate(),
		SuccessRate:    m.SuccessRate(),

		StatusCodes:  make(map[int]int64),
		ContentTypes: make(map[string]int64),
		ErrorCounts:  make(map[ErrorCategory]int64),
		DepthCounts:  make(map[int]int64),
	}

	// Copy status codes
	m.statusMu.Lock()
	for code, counter := range m.statusCodes {
		snap.StatusCodes[code] = counter.Load()
	}
	m.statusMu.Unlock()

	// Copy content types
	m.contentMu.Lock()
	for ct, counter := range m.contentTypes {
		snap.ContentTypes[ct] = counter.Load()
	}
	m.contentMu.Unlock()

	// Copy error counts
	m.errorMu.Lock()
	for cat, counter := range m.errorCounts {
		snap.ErrorCounts[cat] = counter.Load()
	}
	m.errorMu.Unlock()

	// Copy depth counts
	m.depthMu.Lock()
	for depth, counter := range m.depthCounts {
		snap.DepthCounts[depth] = counter.Load()
	}
	m.depthMu.Unlock()

	return snap
}

// ─────────────────────────────────────────────
//  Conversion to legacy SummaryStats
// ─────────────────────────────────────────────

// ToSummaryStats converts a snapshot to the legacy SummaryStats format
// used by the logger for backward compatibility.
func (snap MetricsSnapshot) ToSummaryStats() SummaryStats {
	statusCodes := make(map[int]int, len(snap.StatusCodes))
	for code, count := range snap.StatusCodes {
		statusCodes[code] = int(count)
	}

	return SummaryStats{
		PagesScraped:   int(snap.PagesScraped),
		Errors:         int(snap.ErrorsTotal),
		Duration:       snap.Duration,
		PagesPerSecond: snap.PagesPerSecond,
		TotalSize:      snap.BytesReceived,
		StatusCodes:    statusCodes,
	}
}

// ─────────────────────────────────────────────
//  String representation (for debugging)
// ─────────────────────────────────────────────

// String returns a human-readable summary of current metrics.
func (m *MetricsCollector) String() string {
	snap := m.Snapshot()
	return fmt.Sprintf(
		"Metrics{pages=%d, errors=%d, skipped=%d, dupes=%d, robots_blocked=%d, "+
			"bytes=%s, rate=%.1f p/s, avg_latency=%s, p95=%s, err_rate=%.1f%%, duration=%s}",
		snap.PagesScraped,
		snap.ErrorsTotal,
		snap.PagesSkipped,
		snap.Duplicates,
		snap.RobotsBlocked,
		humanizeBytes(snap.BytesReceived),
		snap.PagesPerSecond,
		snap.AvgLatency.Round(time.Millisecond),
		snap.P95Latency.Round(time.Millisecond),
		snap.ErrorRate,
		snap.Duration.Round(time.Millisecond),
	)
}
// IncrementLinksDiscovered is the name callbacks.go calls.
// Delegates to MetricsCollector.RecordLinksFound.
func (m *Metrics) IncrementLinksDiscovered() {
	m.MetricsCollector.RecordLinksFound(1)
}
// IncrementPagesScraped is the name execute.go calls in the chunked crawl callback.
// Delegates to MetricsCollector.pagesScraped atomic increment.
func (m *Metrics) IncrementPagesScraped() {
	m.MetricsCollector.pagesScraped.Add(1)
}

// RecordStatus is the name execute.go calls to track HTTP status codes.
// Delegates to MetricsCollector.recordStatusCode.
func (m *Metrics) RecordStatus(code int) {
	m.MetricsCollector.recordStatusCode(code)
}

// AddBytes is the name execute.go calls to accumulate received bytes.
// Delegates to MetricsCollector.bytesReceived atomic add.
func (m *Metrics) AddBytes(n int64) {
	m.MetricsCollector.bytesReceived.Add(n)
}
// IncrementErrors is used by the chunked crawl error callback.
func (m *Metrics) IncrementErrors() {
	m.MetricsCollector.errorsTotal.Add(1)
}
