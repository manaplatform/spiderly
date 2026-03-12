// internal/chunker/chunker.go
package chunker

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"sync/atomic"
	"time"
	"spiderly/internal/crawler"
	"spiderly/internal/models"
)

// ─────────────────────────────────────────────
//  ANSI Colors
// ─────────────────────────────────────────────

const (
	Reset         = "\033[0m"
	Bold          = "\033[1m"
	Dim           = "\033[2m"
	Red           = "\033[31m"
	Green         = "\033[32m"
	Yellow        = "\033[33m"
	Blue          = "\033[34m"
	Magenta       = "\033[35m"
	Cyan          = "\033[36m"
	White         = "\033[37m"
	BrightRed     = "\033[91m"
	BrightGreen   = "\033[92m"
	BrightYellow  = "\033[93m"
	BrightBlue    = "\033[94m"
	BrightMagenta = "\033[95m"
	BrightCyan    = "\033[96m"
	BrightWhite   = "\033[97m"
)

// ─────────────────────────────────────────────
//  Configuration
// ─────────────────────────────────────────────

// Config holds chunker configuration
// Config holds chunker configuration
type Config struct {
	// Chunking settings
	ChunkSize  int // URLs per chunk (default: 50)
	MaxWorkers int // Max parallel workers/processes (default: 4)

	// Crawler settings (applied to each worker)
	Concurrency int           // Concurrent requests per worker
	Delay       time.Duration // Delay between requests
	Timeout     time.Duration // Request timeout
	Headless    bool          // Use headless browser

	// Product extraction settings
	ProductMode    bool   // Enable product extraction mode
	ProductPattern string // URL pattern to identify product pages
	ExtractSpecs   bool   // Extract product specifications
	ExtractImages  bool   // Extract product images

	// UI settings
	Verbose bool
	NoColor bool
}


// DefaultConfig returns sensible defaults
func DefaultConfig() Config {
	return Config{
		ChunkSize:   50,
		MaxWorkers:  4,
		Concurrency: 5,
		Delay:       200 * time.Millisecond,
		Timeout:     30 * time.Second,
		Verbose:     false,
		NoColor:     false,
	}
}

// ─────────────────────────────────────────────
//  Chunk Struct
// ─────────────────────────────────────────────

// Chunk represents a batch of URLs to process
type Chunk struct {
	ID      int
	URLs    []string
	Entries []models.SitemapEntry
}

// ─────────────────────────────────────────────
//  Worker Result
// ─────────────────────────────────────────────

// WorkerResult holds results from a single worker
type WorkerResult struct {
	ChunkID   int
	Pages     []models.ScrapedPage
	Errors    []WorkerError
	Duration  time.Duration
	StartTime time.Time
	EndTime   time.Time
}

// WorkerError tracks errors per URL
type WorkerError struct {
	URL     string
	Error   string
	ChunkID int
}

// ─────────────────────────────────────────────
//  Progress Tracking
// ─────────────────────────────────────────────

// Progress tracks overall chunker progress
type Progress struct {
	TotalURLs       int64
	ProcessedURLs   int64
	TotalChunks     int
	CompletedChunks int32
	ActiveWorkers   int32
	TotalErrors     int64
	StartTime       time.Time
}

// ─────────────────────────────────────────────
//  Chunker Struct
// ─────────────────────────────────────────────

// Chunker orchestrates parallel chunk processing
type Chunker struct {
	config   Config
	baseURL  string
	chunks   []Chunk
	progress *Progress
	results  []WorkerResult
	mu       sync.RWMutex
	ctx      context.Context
	cancel   context.CancelFunc
	logger   *ChunkerLogger
	
	// Callbacks
	onPageScraped    func(page models.ScrapedPage, chunkID int)
	onChunkComplete  func(result WorkerResult)
	onError          func(err WorkerError)
}

// ─────────────────────────────────────────────
//  Constructor
// ─────────────────────────────────────────────

// New creates a new Chunker instance
func New(cfg Config) *Chunker {
	ctx, cancel := context.WithCancel(context.Background())
	
	return &Chunker{
		config:  cfg,
		chunks:  make([]Chunk, 0),
		results: make([]WorkerResult, 0),
		progress: &Progress{
			StartTime: time.Now(),
		},
		ctx:    ctx,
		cancel: cancel,
		logger: NewChunkerLogger(cfg.NoColor, cfg.Verbose),
	}
}

// ─────────────────────────────────────────────
//  Callbacks
// ─────────────────────────────────────────────

// OnPageScraped sets callback for each scraped page
func (c *Chunker) OnPageScraped(fn func(page models.ScrapedPage, chunkID int)) {
	c.onPageScraped = fn
}

// OnChunkComplete sets callback when a chunk finishes
func (c *Chunker) OnChunkComplete(fn func(result WorkerResult)) {
	c.onChunkComplete = fn
}

// OnError sets callback for errors
func (c *Chunker) OnError(fn func(err WorkerError)) {
	c.onError = fn
}

// ─────────────────────────────────────────────
//  Chunking Logic
// ─────────────────────────────────────────────

// SplitEntries divides sitemap entries into chunks
func (c *Chunker) SplitEntries(entries []models.SitemapEntry) []Chunk {
	if len(entries) == 0 {
		return nil
	}
	
	chunkSize := c.config.ChunkSize
	if chunkSize <= 0 {
		chunkSize = 50
	}
	
	numChunks := (len(entries) + chunkSize - 1) / chunkSize
	chunks := make([]Chunk, 0, numChunks)
	
	for i := 0; i < len(entries); i += chunkSize {
		end := i + chunkSize
		if end > len(entries) {
			end = len(entries)
		}
		
		chunkEntries := entries[i:end]
		urls := make([]string, len(chunkEntries))
		for j, e := range chunkEntries {
			urls[j] = e.URL
		}
		
		chunks = append(chunks, Chunk{
			ID:      len(chunks) + 1,
			URLs:    urls,
			Entries: chunkEntries,
		})
	}
	
	c.chunks = chunks
	c.progress.TotalChunks = len(chunks)
	c.progress.TotalURLs = int64(len(entries))
	
	return chunks
}

// SplitURLs divides URL list into chunks
func (c *Chunker) SplitURLs(urls []string) []Chunk {
	entries := make([]models.SitemapEntry, len(urls))
	for i, u := range urls {
		entries[i] = models.SitemapEntry{URL: u}
	}
	return c.SplitEntries(entries)
}

// ─────────────────────────────────────────────
//  Parallel Processing
// ─────────────────────────────────────────────

// Process runs all chunks in parallel with worker pool
func (c *Chunker) Process(baseURL string) ([]models.ScrapedPage, error) {
	c.baseURL = baseURL
	c.progress.StartTime = time.Now()
	
	if len(c.chunks) == 0 {
		return nil, fmt.Errorf("no chunks to process - call SplitEntries first")
	}
	
	// Show header
	c.logger.Header()
	c.logger.ChunkingInfo(len(c.chunks), int(c.progress.TotalURLs), c.config.MaxWorkers, c.config.ChunkSize)
	
	// Create worker pool
	maxWorkers := c.config.MaxWorkers
	if maxWorkers <= 0 {
		maxWorkers = 4
	}
	if maxWorkers > len(c.chunks) {
		maxWorkers = len(c.chunks)
	}
	
	// Channels
	chunkChan := make(chan Chunk, len(c.chunks))
	resultChan := make(chan WorkerResult, len(c.chunks))
	
	// Feed chunks to channel
	for _, chunk := range c.chunks {
		chunkChan <- chunk
	}
	close(chunkChan)
	
	// Start progress display
	progressDone := make(chan struct{})
	go c.progressDisplay(progressDone)
	
	// Start workers
	var wg sync.WaitGroup
	for i := 0; i < maxWorkers; i++ {
		wg.Add(1)
		go c.worker(i+1, chunkChan, resultChan, &wg)
	}
	
	// Collect results
	go func() {
		wg.Wait()
		close(resultChan)
	}()
	
	// Aggregate results
	var allPages []models.ScrapedPage
	for result := range resultChan {
		c.mu.Lock()
		c.results = append(c.results, result)
		allPages = append(allPages, result.Pages...)
		c.mu.Unlock()
		
		if c.onChunkComplete != nil {
			c.onChunkComplete(result)
		}
	}
	
	// Stop progress display
	close(progressDone)
	time.Sleep(100 * time.Millisecond) // Let final update render
	
	// Show summary
	c.logger.Summary(c.buildSummary(allPages))
	
	return allPages, nil
}

// worker processes chunks from the channel
func (c *Chunker) worker(workerID int, chunks <-chan Chunk, results chan<- WorkerResult, wg *sync.WaitGroup) {
	defer wg.Done()
	
	atomic.AddInt32(&c.progress.ActiveWorkers, 1)
	defer atomic.AddInt32(&c.progress.ActiveWorkers, -1)
	
	for chunk := range chunks {
		select {
		case <-c.ctx.Done():
			return
		default:
			result := c.processChunk(workerID, chunk)
			results <- result
			atomic.AddInt32(&c.progress.CompletedChunks, 1)
		}
	}
}

// processChunk crawls all URLs in a single chunk
func (c *Chunker) processChunk(workerID int, chunk Chunk) WorkerResult {
	startTime := time.Now()
	
	result := WorkerResult{
		ChunkID:   chunk.ID,
		StartTime: startTime,
		Pages:     make([]models.ScrapedPage, 0, len(chunk.URLs)),
		Errors:    make([]WorkerError, 0),
	}
	
	c.logger.ChunkStart(workerID, chunk.ID, len(chunk.URLs))
	
	// Create crawler for this chunk
	crawlerCfg := crawler.Config{
		MaxPages:    len(chunk.URLs),
		MaxDepth:    1,
		Concurrency: c.config.Concurrency,
		Delay:       c.config.Delay,
		Timeout:     c.config.Timeout,
		Headless:    c.config.Headless,
		SitemapMode: true,
	}
	
	crwl := crawler.NewCrawler(crawlerCfg)
	
	// Set up callbacks
	var pagesMu sync.Mutex
	crwl.OnPageScraped(func(page models.ScrapedPage) {
		pagesMu.Lock()
		result.Pages = append(result.Pages, page)
		pagesMu.Unlock()
		
		atomic.AddInt64(&c.progress.ProcessedURLs, 1)
		
		if c.onPageScraped != nil {
			c.onPageScraped(page, chunk.ID)
		}
		
		c.logger.PageScraped(workerID, chunk.ID, page.URL, page.Title, page.StatusCode)
	})
	
	crwl.OnError(func(url string, err error) {
		workerErr := WorkerError{
			URL:     url,
			Error:   err.Error(),
			ChunkID: chunk.ID,
		}
		
		pagesMu.Lock()
		result.Errors = append(result.Errors, workerErr)
		pagesMu.Unlock()
		
		atomic.AddInt64(&c.progress.TotalErrors, 1)
		atomic.AddInt64(&c.progress.ProcessedURLs, 1)
		
		if c.onError != nil {
			c.onError(workerErr)
		}
		
		c.logger.PageError(workerID, chunk.ID, url, err)
	})
	
	// Queue all URLs
	for _, u := range chunk.URLs {
		crwl.QueueURL(u, 0)
	}
	
	// Run crawl
	_, _ = crwl.Crawl(c.baseURL)
	
	result.EndTime = time.Now()
	result.Duration = result.EndTime.Sub(startTime)
	
	c.logger.ChunkComplete(workerID, chunk.ID, len(result.Pages), len(result.Errors), result.Duration)
	
	return result
}

// ─────────────────────────────────────────────
//  Progress Display
// ─────────────────────────────────────────────

func (c *Chunker) progressDisplay(done <-chan struct{}) {
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()
	
	for {
		select {
		case <-done:
			c.logger.ProgressBar(c.progress, true)
			return
		case <-ticker.C:
			c.logger.ProgressBar(c.progress, false)
		}
	}
}

// ─────────────────────────────────────────────
//  Summary Builder
// ─────────────────────────────────────────────

type Summary struct {
	TotalPages      int
	TotalErrors     int
	TotalChunks     int
	Duration        time.Duration
	PagesPerSecond  float64
	TotalSize       int64
	StatusCodes     map[int]int
	WorkerStats     []WorkerStat
	FastestChunk    int
	SlowestChunk    int
	FastestDuration time.Duration
	SlowestDuration time.Duration
}

type WorkerStat struct {
	ChunkID  int
	Pages    int
	Errors   int
	Duration time.Duration
}

func (c *Chunker) buildSummary(pages []models.ScrapedPage) Summary {
	duration := time.Since(c.progress.StartTime)
	
	summary := Summary{
		TotalPages:  len(pages),
		TotalErrors: int(c.progress.TotalErrors),
		TotalChunks: c.progress.TotalChunks,
		Duration:    duration,
		StatusCodes: make(map[int]int),
		WorkerStats: make([]WorkerStat, 0),
	}
	
	if duration.Seconds() > 0 {
		summary.PagesPerSecond = float64(len(pages)) / duration.Seconds()
	}
	
	// Calculate status codes and size
	for _, p := range pages {
		summary.StatusCodes[p.StatusCode]++
		summary.TotalSize += p.ContentLength
	}
	
	// Worker stats
	c.mu.RLock()
	for _, r := range c.results {
		stat := WorkerStat{
			ChunkID:  r.ChunkID,
			Pages:    len(r.Pages),
			Errors:   len(r.Errors),
			Duration: r.Duration,
		}
		summary.WorkerStats = append(summary.WorkerStats, stat)
		
		if summary.FastestDuration == 0 || r.Duration < summary.FastestDuration {
			summary.FastestDuration = r.Duration
			summary.FastestChunk = r.ChunkID
		}
		if r.Duration > summary.SlowestDuration {
			summary.SlowestDuration = r.Duration
			summary.SlowestChunk = r.ChunkID
		}
	}
	c.mu.RUnlock()
	
	// Sort worker stats by chunk ID
	sort.Slice(summary.WorkerStats, func(i, j int) bool {
		return summary.WorkerStats[i].ChunkID < summary.WorkerStats[j].ChunkID
	})
	
	return summary
}

// ─────────────────────────────────────────────
//  Stop
// ─────────────────────────────────────────────

// Stop cancels all running workers
func (c *Chunker) Stop() {
	c.cancel()
}

// GetProgress returns current progress
func (c *Chunker) GetProgress() Progress {
	return Progress{
		TotalURLs:       atomic.LoadInt64(&c.progress.TotalURLs),
		ProcessedURLs:   atomic.LoadInt64(&c.progress.ProcessedURLs),
		TotalChunks:     c.progress.TotalChunks,
		CompletedChunks: atomic.LoadInt32(&c.progress.CompletedChunks),
		ActiveWorkers:   atomic.LoadInt32(&c.progress.ActiveWorkers),
		TotalErrors:     atomic.LoadInt64(&c.progress.TotalErrors),
		StartTime:       c.progress.StartTime,
	}
}

// GetResults returns all worker results
func (c *Chunker) GetResults() []WorkerResult {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.results
}
