package core

import (
	"context"
	"fmt"
	"net/url"
	"regexp"
	"strings"
	"sync"
	"time"

	"spiderly/internal/chunker"
	"spiderly/internal/crawler"
	"spiderly/internal/models"
	"spiderly/internal/sitemap"
)

// ─────────────────────────────────────────────
//  ANSI Color Codes
// ─────────────────────────────────────────────

const (
	Reset      = "\033[0m"
	Bold       = "\033[1m"
	Dim        = "\033[2m"
	Italic     = "\033[3m"
	Underline  = "\033[4m"
	
	// Colors
	Black      = "\033[30m"
	Red        = "\033[31m"
	Green      = "\033[32m"
	Yellow     = "\033[33m"
	Blue       = "\033[34m"
	Magenta    = "\033[35m"
	Cyan       = "\033[36m"
	White      = "\033[37m"
	
	// Bright Colors
	BrightRed     = "\033[91m"
	BrightGreen   = "\033[92m"
	BrightYellow  = "\033[93m"
	BrightBlue    = "\033[94m"
	BrightMagenta = "\033[95m"
	BrightCyan    = "\033[96m"
	BrightWhite   = "\033[97m"
	
	// Background
	BgBlue     = "\033[44m"
	BgMagenta  = "\033[45m"
	BgCyan     = "\033[46m"
)

// ─────────────────────────────────────────────
//  Configuration
// ─────────────────────────────────────────────

// CoreConfig holds all configuration for the crawl core
// CoreConfig holds all configuration for the crawl core
type CoreConfig struct {
	TargetURL  string
	SitemapURL string

	MaxPages    int
	MaxDepth    int
	Concurrency int
	Delay       time.Duration
	Timeout     time.Duration

	MinPriority float64
	URLPattern  string

	ForceRecursive bool
	Headless       bool
	Verbose        bool
	NoColor        bool

	// Chunker settings
	EnableChunker bool
	ChunkSize     int
	MaxWorkers    int

	// Product mode settings
	ProductMode      bool     // Enable product extraction
	ProductSitemaps  []string // Filter to specific sitemap types (e.g., ["pdp"])
	ExtractSpecs     bool     // Extract product specifications
	ExtractImages    bool     // Extract all product images
}


// ─────────────────────────────────────────────
//  Core Struct
// ─────────────────────────────────────────────

// Core is the main orchestrator for Spiderly
type Core struct {
	config    CoreConfig
	crawler   *crawler.Crawler
	chunker   *chunker.Chunker
	stats     *models.CrawlStats
	results   []models.ScrapedPage
	mu        sync.RWMutex
	ctx       context.Context
	cancel    context.CancelFunc
	startTime time.Time
	logger    *Logger
}



// ─────────────────────────────────────────────
//  Constructors
// ─────────────────────────────────────────────


func New(targetURL string, maxPages int) *Core {
	return NewCore(CoreConfig{
		TargetURL:     targetURL,
		MaxPages:      maxPages,
		MaxDepth:      10,
		Concurrency:   5,
		Delay:         200 * time.Millisecond,
		Timeout:       30 * time.Second,
		Verbose:       false,
		EnableChunker: false,
		ChunkSize:     50,
		MaxWorkers:    4,
	})
}
// NewChunked creates a Core with chunker enabled
func NewChunked(targetURL string, maxPages, chunkSize, workers int) *Core {
	return NewCore(CoreConfig{
		TargetURL:     targetURL,
		MaxPages:      maxPages,
		MaxDepth:      10,
		Concurrency:   5,
		Delay:         200 * time.Millisecond,
		Timeout:       30 * time.Second,
		Verbose:       false,
		EnableChunker: true,
		ChunkSize:     chunkSize,
		MaxWorkers:    workers,
	})
}
func NewCore(cfg CoreConfig) *Core {
	ctx, cancel := context.WithCancel(context.Background())

	return &Core{
		config:    cfg,
		stats:     &models.CrawlStats{},
		results:   make([]models.ScrapedPage, 0),
		ctx:       ctx,
		cancel:    cancel,
		startTime: time.Now(),
		logger:    NewLogger(cfg.NoColor, cfg.Verbose),
	}
}
// [ApplyConfig remains similar but add chunker fields...]

func (c *Core) ApplyConfig(cfg CoreConfig) {
	if cfg.TargetURL != "" {
		c.config.TargetURL = cfg.TargetURL
	}
	if cfg.SitemapURL != "" {
		c.config.SitemapURL = cfg.SitemapURL
	}
	if cfg.MaxPages > 0 {
		c.config.MaxPages = cfg.MaxPages
	}
	if cfg.MaxDepth > 0 {
		c.config.MaxDepth = cfg.MaxDepth
	}
	if cfg.Concurrency > 0 {
		c.config.Concurrency = cfg.Concurrency
	}
	if cfg.Delay > 0 {
		c.config.Delay = cfg.Delay
	}
	if cfg.Timeout > 0 {
		c.config.Timeout = cfg.Timeout
	}
	if cfg.MinPriority > 0 {
		c.config.MinPriority = cfg.MinPriority
	}
	if cfg.URLPattern != "" {
		c.config.URLPattern = cfg.URLPattern
	}
	if cfg.ChunkSize > 0 {
		c.config.ChunkSize = cfg.ChunkSize
	}
	if cfg.MaxWorkers > 0 {
		c.config.MaxWorkers = cfg.MaxWorkers
	}

	c.config.ForceRecursive = cfg.ForceRecursive
	c.config.Headless = cfg.Headless
	c.config.Verbose = cfg.Verbose
	c.config.NoColor = cfg.NoColor
	c.config.EnableChunker = cfg.EnableChunker
	
	c.logger = NewLogger(c.config.NoColor, c.config.Verbose)
}

// ─────────────────────────────────────────────
//  Logger - Beautiful Console Output
// ─────────────────────────────────────────────

type Logger struct {
	noColor   bool
	verbose   bool
	mu        sync.Mutex
	pageCount int
	errorCount int
	startTime time.Time
}

func NewLogger(noColor, verbose bool) *Logger {
	return &Logger{
		noColor:   noColor,
		verbose:   verbose,
		startTime: time.Now(),
	}
}

func (l *Logger) color(c, text string) string {
	if l.noColor {
		return text
	}
	return c + text + Reset
}

func (l *Logger) Header() {
	l.mu.Lock()
	defer l.mu.Unlock()
	
	fmt.Println()
	fmt.Println(l.color(BrightCyan+Bold, "  ╔═══════════════════════════════════════════════════════════════╗"))
	fmt.Println(l.color(BrightCyan+Bold, "  ║") + l.color(BrightMagenta+Bold, "     🕷️  SPIDERLY - High Performance Web Crawler              ") + l.color(BrightCyan+Bold, "║"))
	fmt.Println(l.color(BrightCyan+Bold, "  ╚═══════════════════════════════════════════════════════════════╝"))
	fmt.Println()
}

func (l *Logger) Phase(phase, message string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	
	icon := l.getPhaseIcon(phase)
	phaseText := l.color(BrightYellow+Bold, fmt.Sprintf(" %s %s ", icon, strings.ToUpper(phase)))
	fmt.Printf("\n%s %s\n", phaseText, l.color(White, message))
	fmt.Println(l.color(Dim, "  "+strings.Repeat("─", 60)))
}

func (l *Logger) getPhaseIcon(phase string) string {
	icons := map[string]string{
		"init":       "🚀",
		"discovery":  "🔍",
		"sitemap":    "🗺️ ",
		"crawling":   "🕸️ ",
		"complete":   "✨",
		"error":      "💥",
	}
	if icon, ok := icons[phase]; ok {
		return icon
	}
	return "📌"
}

func (l *Logger) Info(format string, args ...interface{}) {
	l.mu.Lock()
	defer l.mu.Unlock()
	
	msg := fmt.Sprintf(format, args...)
	fmt.Printf("  %s %s\n", l.color(Blue, "ℹ"), l.color(White, msg))
}

func (l *Logger) Success(format string, args ...interface{}) {
	l.mu.Lock()
	defer l.mu.Unlock()
	
	msg := fmt.Sprintf(format, args...)
	fmt.Printf("  %s %s\n", l.color(BrightGreen, "✓"), l.color(Green, msg))
}

func (l *Logger) Warning(format string, args ...interface{}) {
	l.mu.Lock()
	defer l.mu.Unlock()
	
	msg := fmt.Sprintf(format, args...)
	fmt.Printf("  %s %s\n", l.color(Yellow, "⚠"), l.color(Yellow, msg))
}

func (l *Logger) Error(format string, args ...interface{}) {
	l.mu.Lock()
	defer l.mu.Unlock()
	
	msg := fmt.Sprintf(format, args...)
	fmt.Printf("  %s %s\n", l.color(BrightRed, "✗"), l.color(Red, msg))
}

func (l *Logger) Verbose(format string, args ...interface{}) {
	if !l.verbose {
		return
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	
	msg := fmt.Sprintf(format, args...)
	fmt.Printf("  %s %s\n", l.color(Dim, "›"), l.color(Dim, msg))
}

func (l *Logger) PageScraped(url, title string, statusCode int, loadTime int64) {
	l.mu.Lock()
	l.pageCount++
	count := l.pageCount
	l.mu.Unlock()
	
	// Truncate URL and title for display
	displayURL := truncateString(url, 50)
	displayTitle := truncateString(title, 35)
	if displayTitle == "" {
		displayTitle = "(no title)"
	}
	
	// Status color
	statusColor := l.getStatusColor(statusCode)
	statusStr := l.color(statusColor, fmt.Sprintf("[%d]", statusCode))
	
	// Format line
	countStr := l.color(Cyan, fmt.Sprintf("#%-4d", count))
	timeStr := l.color(Dim, fmt.Sprintf("%4dms", loadTime))
	
	l.mu.Lock()
	defer l.mu.Unlock()
	fmt.Printf("  %s %s %s %s %s\n",
		countStr,
		statusStr,
		timeStr,
		l.color(BrightWhite, displayTitle),
		l.color(Dim, displayURL),
	)
}

func (l *Logger) PageError(url string, err error) {
	l.mu.Lock()
	l.errorCount++
	l.mu.Unlock()
	
	displayURL := truncateString(url, 50)
	errMsg := truncateString(err.Error(), 40)
	
	l.mu.Lock()
	defer l.mu.Unlock()
	fmt.Printf("  %s %s %s\n",
		l.color(Red, "✗ ERR"),
		l.color(Dim, displayURL),
		l.color(Red, errMsg),
	)
}

func (l *Logger) LinkDiscovered(url string, depth int) {
	if !l.verbose {
		return
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	
	displayURL := truncateString(url, 60)
	fmt.Printf("  %s %s %s\n",
		l.color(Dim, "  └─"),
		l.color(Dim, fmt.Sprintf("d%d", depth)),
		l.color(Dim, displayURL),
	)
}

func (l *Logger) getStatusColor(code int) string {
	switch {
	case code >= 200 && code < 300:
		return BrightGreen
	case code >= 300 && code < 400:
		return BrightYellow
	case code >= 400 && code < 500:
		return Yellow
	case code >= 500:
		return BrightRed
	default:
		return White
	}
}

func (l *Logger) SitemapStats(total, filtered, sitemapCount int) {
	l.mu.Lock()
	defer l.mu.Unlock()
	
	fmt.Printf("\n  %s %s\n",
		l.color(Magenta, "📊"),
		l.color(BrightWhite+Bold, "Sitemap Analysis"),
	)
	fmt.Printf("     %s Sitemaps found:  %s\n", l.color(Dim, "├─"), l.color(Cyan, fmt.Sprintf("%d", sitemapCount)))
	fmt.Printf("     %s Total URLs:      %s\n", l.color(Dim, "├─"), l.color(Cyan, fmt.Sprintf("%d", total)))
	fmt.Printf("     %s After filtering: %s\n", l.color(Dim, "└─"), l.color(BrightGreen, fmt.Sprintf("%d", filtered)))
	fmt.Println()
}

func (l *Logger) Progress(current, total int, phase string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	
	if total == 0 {
		return
	}
	
	percent := float64(current) / float64(total) * 100
	barWidth := 30
	filled := int(float64(barWidth) * float64(current) / float64(total))
	
	bar := strings.Repeat("█", filled) + strings.Repeat("░", barWidth-filled)
	
	fmt.Printf("\r  %s %s %s %s",
		l.color(Cyan, fmt.Sprintf("%3.0f%%", percent)),
		l.color(BrightBlue, bar),
		l.color(White, fmt.Sprintf("%d/%d", current, total)),
		l.color(Dim, phase),
	)
	
	if current >= total {
		fmt.Println()
	}
}

func (l *Logger) Summary(stats SummaryStats) {
	l.mu.Lock()
	defer l.mu.Unlock()
	
	duration := stats.Duration.Round(time.Millisecond)
	
	fmt.Println()
	fmt.Println(l.color(BrightCyan, "  ╔═══════════════════════════════════════════════════════════════╗"))
	fmt.Println(l.color(BrightCyan, "  ║") + l.color(BrightGreen+Bold, "                    ✨ CRAWL COMPLETE ✨                       ") + l.color(BrightCyan, "║"))
	fmt.Println(l.color(BrightCyan, "  ╠═══════════════════════════════════════════════════════════════╣"))
	
	fmt.Printf(l.color(BrightCyan, "  ║")+"  📄 Pages Scraped:    %-40s"+l.color(BrightCyan, "║")+"\n", l.color(BrightGreen+Bold, fmt.Sprintf("%d", stats.PagesScraped)))
	fmt.Printf(l.color(BrightCyan, "  ║")+"  ❌ Errors:           %-40s"+l.color(BrightCyan, "║")+"\n", l.color(Yellow, fmt.Sprintf("%d", stats.Errors)))
	fmt.Printf(l.color(BrightCyan, "  ║")+"  ⏱️  Duration:         %-40s"+l.color(BrightCyan, "║")+"\n", l.color(Cyan, duration.String()))
	fmt.Printf(l.color(BrightCyan, "  ║")+"  ⚡ Speed:            %-40s"+l.color(BrightCyan, "║")+"\n", l.color(Cyan, fmt.Sprintf("%.1f pages/sec", stats.PagesPerSecond)))
	
	if stats.TotalSize > 0 {
		fmt.Printf(l.color(BrightCyan, "  ║")+"  📦 Total Size:       %-40s"+l.color(BrightCyan, "║")+"\n", l.color(Cyan, humanizeBytes(stats.TotalSize)))
	}
	
	fmt.Println(l.color(BrightCyan, "  ╠═══════════════════════════════════════════════════════════════╣"))
	
	// HTTP Status breakdown
	fmt.Println(l.color(BrightCyan, "  ║") + l.color(White+Bold, "  HTTP Status Breakdown:                                      ") + l.color(BrightCyan, "║"))
	for code, count := range stats.StatusCodes {
		emoji := statusEmoji(code)
		color := l.getStatusColor(code)
		fmt.Printf(l.color(BrightCyan, "  ║")+"     %s %s: %-46s"+l.color(BrightCyan, "║")+"\n", emoji, l.color(color, fmt.Sprintf("%d", code)), l.color(color, fmt.Sprintf("%d", count)))
	}
	
	fmt.Println(l.color(BrightCyan, "  ╚═══════════════════════════════════════════════════════════════╝"))
	fmt.Println()
}

type SummaryStats struct {
	PagesScraped   int
	Errors         int
	Duration       time.Duration
	PagesPerSecond float64
	TotalSize      int64
	StatusCodes    map[int]int
}

// ─────────────────────────────────────────────
//  Exported Result Type (with Product Support)
// ─────────────────────────────────────────────

// ProductResult holds product-specific data for export
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

func ToScrapedPageResults(pages []models.ScrapedPage) []ScrapedPageResult {
	results := make([]ScrapedPageResult, len(pages))
	for i, p := range pages {
		results[i] = ScrapedPageResult{
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
		
		// Map product data if present
		if p.Product != nil {
			results[i].Product = &ProductResult{
				Name:          p.Product.Name,
				Brand:         p.Product.Brand,
				SKU:           p.Product.SKU,
				GTIN:          p.Product.GTIN,
				MPN:           p.Product.MPN,
				Price:         p.Product.Price,
				Currency:      p.Product.Currency,
				OriginalPrice: p.Product.OriginalPrice,
				Discount:      p.Product.Discount,
				Availability:  p.Product.Availability,
				InStock:       p.Product.InStock,
				Rating:        p.Product.Rating,
				ReviewCount:   p.Product.ReviewCount,
				Category:      p.Product.Category,
				Categories:    p.Product.Categories,
				Images:        p.Product.Images,
				Description:   p.Product.Description,
				Specs:         p.Product.Specs,
			}
		}
	}
	return results
}




// ─────────────────────────────────────────────
//  Run — Main Pipeline
// ─────────────────────────────────────────────

func (c *Core) Run() ([]models.ScrapedPage, error) {
	c.startTime = time.Now()

	// Validate target URL
	targetURL, err := c.validateTargetURL()
	if err != nil {
		return nil, fmt.Errorf("invalid target URL: %w", err)
	}

	// If chunker is disabled, show header
	if !c.config.EnableChunker {
		c.logger.Header()
		c.logger.Phase("init", fmt.Sprintf("Target: %s", targetURL))
		c.logger.Info("Max pages: %d | Concurrency: %d | Timeout: %s", 
			c.config.MaxPages, c.config.Concurrency, c.config.Timeout)
	}

	// Determine crawl strategy
	strategy, sitemapURLs, err := c.determineCrawlStrategy(targetURL)
	if err != nil {
		c.logger.Verbose("Strategy determination warning: %v", err)
	}

	// Execute crawl based on strategy
	switch strategy {
	case "sitemap":
		if c.config.EnableChunker {
			return c.executeChunkedSitemapCrawl(targetURL, sitemapURLs)
		}
		return c.executeSitemapCrawl(targetURL, sitemapURLs)
	case "recursive":
		return c.executeRecursiveCrawl(targetURL)
	default:
		return c.executeRecursiveCrawl(targetURL)
	}
}



// ─────────────────────────────────────────────
//  Chunked Sitemap Crawl
// ─────────────────────────────────────────────

func (c *Core) executeChunkedSitemapCrawl(baseURL string, entries []models.SitemapEntry) ([]models.ScrapedPage, error) {
	// Limit entries to MaxPages
	if c.config.MaxPages > 0 && len(entries) > c.config.MaxPages {
		entries = entries[:c.config.MaxPages]
	}
	
	// Create chunker
	c.chunker = chunker.New(chunker.Config{
		ChunkSize:   c.config.ChunkSize,
		MaxWorkers:  c.config.MaxWorkers,
		Concurrency: c.config.Concurrency,
		Delay:       c.config.Delay,
		Timeout:     c.config.Timeout,
		Headless:    c.config.Headless,
		Verbose:     c.config.Verbose,
		NoColor:     c.config.NoColor,
	})
	
	// Set callbacks
	c.chunker.OnPageScraped(func(page models.ScrapedPage, chunkID int) {
		c.mu.Lock()
		c.stats.PagesScraped++
		c.mu.Unlock()
	})
	
	c.chunker.OnError(func(err chunker.WorkerError) {
		c.mu.Lock()
		c.stats.Errors++
		c.mu.Unlock()
	})
	
	// Split into chunks
	c.chunker.SplitEntries(entries)
	
	// Process all chunks in parallel
	results, err := c.chunker.Process(baseURL)
	if err != nil {
		return nil, err
	}
	
	c.mu.Lock()
	c.results = results
	c.mu.Unlock()
	
	return results, nil
}
// ─────────────────────────────────────────────
//  URL Validation
// ─────────────────────────────────────────────

func (c *Core) validateTargetURL() (string, error) {
	if c.config.SitemapURL != "" {
		parsed, err := url.Parse(c.config.SitemapURL)
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("%s://%s", parsed.Scheme, parsed.Host), nil
	}

	if c.config.TargetURL == "" {
		return "", fmt.Errorf("no target URL specified")
	}

	targetURL := c.config.TargetURL
	if !strings.HasPrefix(targetURL, "http://") && !strings.HasPrefix(targetURL, "https://") {
		targetURL = "https://" + targetURL
	}

	parsed, err := url.Parse(targetURL)
	if err != nil {
		return "", err
	}

	if parsed.Host == "" {
		return "", fmt.Errorf("URL must have a valid host")
	}

	return targetURL, nil
}

func (c *Core) fetchSitemapEntries(sitemapURL string) ([]models.SitemapEntry, error) {
	parser := sitemap.NewParser(c.config.Timeout, c.config.Verbose)
	return parser.ParseSitemap(sitemapURL)
}

func (c *Core) filterSitemapEntries(entries []models.SitemapEntry) []models.SitemapEntry {
	var filtered []models.SitemapEntry

	var urlRegex *regexp.Regexp
	if c.config.URLPattern != "" {
		var err error
		urlRegex, err = regexp.Compile(c.config.URLPattern)
		if err != nil {
			c.logger.Warning("Invalid URL pattern regex: %v", err)
			urlRegex = nil
		}
	}

	for _, entry := range entries {
		if c.config.MinPriority > 0 && entry.Priority < c.config.MinPriority {
			continue
		}
		if urlRegex != nil && !urlRegex.MatchString(entry.URL) {
			continue
		}
		filtered = append(filtered, entry)
	}

	return filtered
}

// ─────────────────────────────────────────────
//  Crawl Execution
// ─────────────────────────────────────────────

func (c *Core) executeSitemapCrawl(baseURL string, entries []models.SitemapEntry) ([]models.ScrapedPage, error) {
	c.logger.Phase("crawling", fmt.Sprintf("Sitemap mode: %d URLs to process", len(entries)))

	urls := make([]string, 0, len(entries))
	for _, entry := range entries {
		urls = append(urls, entry.URL)
	}

	if c.config.MaxPages > 0 && len(urls) > c.config.MaxPages {
		urls = urls[:c.config.MaxPages]
		c.logger.Info("Limited to %d pages (max pages setting)", c.config.MaxPages)
	}

	c.crawler = crawler.NewCrawler(crawler.Config{
		MaxPages:    c.config.MaxPages,
		MaxDepth:    1,
		Concurrency: c.config.Concurrency,
		Delay:       c.config.Delay,
		Timeout:     c.config.Timeout,
		Headless:    c.config.Headless,
		SitemapMode: true,
	})

	c.setupCrawlerCallbacks()

	for _, u := range urls {
		c.crawler.QueueURL(u, 0)
	}

	results, err := c.crawler.Crawl(baseURL)
	if err != nil {
		return nil, err
	}

	c.mu.Lock()
	c.results = results
	c.mu.Unlock()

	c.finalizeCrawl()

	return results, nil
}

func (c *Core) executeRecursiveCrawl(targetURL string) ([]models.ScrapedPage, error) {
	c.logger.Phase("crawling", fmt.Sprintf("Recursive mode from %s", targetURL))

	c.crawler = crawler.NewCrawler(crawler.Config{
		MaxPages:    c.config.MaxPages,
		MaxDepth:    c.config.MaxDepth,
		Concurrency: c.config.Concurrency,
		Delay:       c.config.Delay,
		Timeout:     c.config.Timeout,
		Headless:    c.config.Headless,
		SitemapMode: false,
	})

	c.setupCrawlerCallbacks()

	results, err := c.crawler.Crawl(targetURL)
	if err != nil {
		return nil, err
	}

	c.mu.Lock()
	c.results = results
	c.mu.Unlock()

	c.finalizeCrawl()

	return results, nil
}

// ─────────────────────────────────────────────
//  Callbacks
// ─────────────────────────────────────────────

func (c *Core) setupCrawlerCallbacks() {
	if c.crawler == nil {
		return
	}

	c.crawler.OnPageScraped(func(page models.ScrapedPage) {
		c.mu.Lock()
		c.stats.PagesScraped++
		c.mu.Unlock()

		c.logger.PageScraped(page.URL, page.Title, page.StatusCode, page.LoadTimeMs)
	})

	c.crawler.OnError(func(url string, err error) {
		c.mu.Lock()
		c.stats.Errors++
		c.mu.Unlock()

		c.logger.PageError(url, err)
	})

	c.crawler.OnLinkDiscovered(func(link models.DiscoveredLink) {
		c.logger.LinkDiscovered(link.URL, link.Depth)
	})
}

// ─────────────────────────────────────────────
//  Finalization & Utilities
// ─────────────────────────────────────────────

func (c *Core) finalizeCrawl() {
	duration := time.Since(c.startTime)

	c.mu.RLock()
	stats := *c.stats
	results := c.results
	c.mu.RUnlock()

	// Calculate summary stats
	statusCodes := make(map[int]int)
	var totalSize int64
	for _, r := range results {
		statusCodes[r.StatusCode]++
		totalSize += r.ContentLength
	}

	pagesPerSec := float64(stats.PagesScraped) / duration.Seconds()
	if duration.Seconds() == 0 {
		pagesPerSec = float64(stats.PagesScraped)
	}

	c.logger.Phase("complete", "Crawl finished successfully")
	c.logger.Summary(SummaryStats{
		PagesScraped:   stats.PagesScraped,
		Errors:         stats.Errors,
		Duration:       duration,
		PagesPerSecond: pagesPerSec,
		TotalSize:      totalSize,
		StatusCodes:    statusCodes,
	})
}

// ─────────────────────────────────────────────
//  Public Accessors & Lifecycle
// ─────────────────────────────────────────────

func (c *Core) Stop() {
	c.cancel()
	if c.crawler != nil {
		c.crawler.Stop()
	}
}

func (c *Core) GetResults() []models.ScrapedPage {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.results
}

func (c *Core) GetStats() models.CrawlStats {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return *c.stats
}

// ─────────────────────────────────────────────
//  Helper Functions
// ─────────────────────────────────────────────

func truncateString(s string, maxLen int) string {
	s = strings.TrimSpace(s)
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}

func humanizeBytes(b int64) string {
	if b == 0 {
		return "0 B"
	}
	units := []string{"B", "KB", "MB", "GB"}
	f := float64(b)
	i := 0
	for f >= 1024 && i < len(units)-1 {
		f /= 1024
		i++
	}
	if i == 0 {
		return fmt.Sprintf("%d B", b)
	}
	return fmt.Sprintf("%.1f %s", f, units[i])
}

func statusEmoji(code int) string {
	switch {
	case code >= 200 && code < 300:
		return "✅"
	case code >= 300 && code < 400:
		return "↗️"
	case code >= 400 && code < 500:
		return "⚠️"
	case code >= 500:
		return "🔴"
	default:
		return "❓"
	}
}
// ─────────────────────────────────────────────
//  Crawl Strategy (updated for Product Mode)
// ─────────────────────────────────────────────

func (c *Core) determineCrawlStrategy(targetURL string) (string, []models.SitemapEntry, error) {
	if c.config.ForceRecursive {
		c.logger.Info("Forced recursive mode - skipping sitemap discovery")
		return "recursive", nil, nil
	}

	// ── Direct sitemap URL provided ──
	if c.config.SitemapURL != "" {
		c.logger.Verbose("Direct sitemap URL provided: %s", c.config.SitemapURL)
		entries, err := c.fetchSitemapEntries(c.config.SitemapURL)
		if err != nil {
			return "recursive", nil, fmt.Errorf("failed to fetch provided sitemap: %w", err)
		}
		if len(entries) > 0 {
			filteredEntries := c.filterSitemapEntries(entries)
			if c.config.ProductMode {
				filteredEntries = c.filterProductEntries(filteredEntries)
			}
			if len(filteredEntries) > 0 {
				return "sitemap", filteredEntries, nil
			}
			c.logger.Warning("All entries filtered out — falling back to unfiltered")
			return "sitemap", entries, nil
		}
		return "recursive", nil, fmt.Errorf("provided sitemap was empty")
	}

	// ── Auto-discovery ──
	c.logger.Phase("discovery", "Searching for sitemaps...")

	parser := sitemap.NewParser(c.config.Timeout, c.config.Verbose)

	var sitemapURLs []string
	var err error

	// In product mode, try to discover only product-related sitemaps first
	if c.config.ProductMode {
		c.logger.Info("Product mode enabled — prioritizing product sitemaps")

		// Build filter list from config + defaults
		filters := []string{"pdp", "product"}
		if len(c.config.ProductSitemaps) > 0 {
			filters = c.config.ProductSitemaps
		}

		sitemapURLs, err = parser.DiscoverSitemapsFiltered(targetURL, filters)
		if err != nil {
			c.logger.Verbose("Filtered sitemap discovery error: %v", err)
		}

		if len(sitemapURLs) == 0 {
			c.logger.Warning("No product-specific sitemaps found — trying all sitemaps")
			sitemapURLs, err = parser.DiscoverSitemaps(targetURL)
			if err != nil {
				c.logger.Verbose("Sitemap discovery error: %v", err)
			}
		} else {
			c.logger.Success("Found %d product sitemap(s)", len(sitemapURLs))
		}
	} else {
		sitemapURLs, err = parser.DiscoverSitemaps(targetURL)
		if err != nil {
			c.logger.Verbose("Sitemap discovery error: %v", err)
		}
	}

	if len(sitemapURLs) == 0 {
		c.logger.Warning("No sitemaps found - falling back to recursive crawl")
		return "recursive", nil, nil
	}

	c.logger.Success("Found %d sitemap(s)", len(sitemapURLs))

	// Parse all discovered sitemaps
	var allEntries []models.SitemapEntry
	for _, sitemapURL := range sitemapURLs {
		c.logger.Verbose("Parsing: %s", sitemapURL)
		entries, err := c.fetchSitemapEntries(sitemapURL)
		if err != nil {
			c.logger.Verbose("Failed to parse sitemap %s: %v", sitemapURL, err)
			continue
		}
		allEntries = append(allEntries, entries...)
	}

	if len(allEntries) == 0 {
		c.logger.Warning("All sitemaps were empty - falling back to recursive crawl")
		return "recursive", nil, nil
	}

	// Apply standard filters (priority, URL pattern)
	filteredEntries := c.filterSitemapEntries(allEntries)

	// Apply product mode filter on top
	if c.config.ProductMode {
		filteredEntries = c.filterProductEntries(filteredEntries)
	}

	c.logger.SitemapStats(len(allEntries), len(filteredEntries), len(sitemapURLs))

	if len(filteredEntries) == 0 {
		c.logger.Warning("All entries were filtered out — using unfiltered entries")
		filteredEntries = allEntries
	}

	return "sitemap", filteredEntries, nil
}

// ─────────────────────────────────────────────
//  Product Filtering
// ─────────────────────────────────────────────

// filterProductEntries applies product-mode heuristics to keep only product page URLs.
// It checks: (1) sitemap type tags, (2) explicit regex pattern, (3) URL heuristics.
func (c *Core) filterProductEntries(entries []models.SitemapEntry) []models.SitemapEntry {
	var productURLRegex *regexp.Regexp
	if c.config.URLPattern != "" {
		var err error
		productURLRegex, err = regexp.Compile(c.config.URLPattern)
		if err != nil {
			c.logger.Warning("Invalid product URL pattern: %v — using heuristics only", err)
			productURLRegex = nil
		}
	}

	var filtered []models.SitemapEntry
	for _, entry := range entries {
		// 1. If entry type is already tagged as "pdp" or "product", keep it
		if entry.Type == "pdp" || entry.Type == "product" {
			filtered = append(filtered, entry)
			continue
		}

		// 2. Check against explicit regex pattern
		if productURLRegex != nil {
			if productURLRegex.MatchString(entry.URL) {
				filtered = append(filtered, entry)
				continue
			}
		}

		// 3. Fall back to heuristic detection
		if sitemap.IsLikelyProductURL(entry.URL) {
			filtered = append(filtered, entry)
			continue
		}
	}

	c.logger.Verbose("Product filter: %d → %d entries", len(entries), len(filtered))
	return filtered
}
