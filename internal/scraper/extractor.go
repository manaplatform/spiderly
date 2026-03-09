package scraper

import (
	"strconv"
	"time"

	"spiderly/internal/crawler"
	"spiderly/internal/models"
	"spiderly/internal/web"
)

// ScraperConfig holds configuration for the scraper
type ScraperConfig struct {
	MaxDepth       int
	MaxPages       int
	Timeout        time.Duration
	WaitTime       time.Duration
	FollowExternal bool
	WebPort        int
}

// DefaultConfig returns default scraper configuration
func DefaultConfig() ScraperConfig {
	return ScraperConfig{
		MaxDepth:       2,
		MaxPages:       10,
		Timeout:        30 * time.Second,
		WaitTime:       2 * time.Second,
		FollowExternal: false,
		WebPort:        8080,
	}
}

// Scraper handles the overall scraping process
type Scraper struct {
	config  ScraperConfig
	server  *web.Server
	crawler *crawler.Crawler
}

// NewScraper creates a new Scraper instance
func NewScraper(config ScraperConfig) *Scraper {
	server := web.NewServer(config.WebPort)

	crawlerConfig := crawler.Config{
		MaxDepth:       config.MaxDepth,
		MaxPages:       config.MaxPages,
		Timeout:        config.Timeout,
		WaitTime:       config.WaitTime,
		FollowExternal: config.FollowExternal,
		Selectors:      crawler.DefaultSelectors(),
	}

	return &Scraper{
		config:  config,
		server:  server,
		crawler: crawler.NewCrawler(crawlerConfig, server),
	}
}

// Run starts the web server and scraping process
func (s *Scraper) Run(startURL string) ([]models.CrawlResult, error) {
	// Start the web server
	s.server.Start()

	// Print console info
	printStartupInfo(s.config)

	// Start crawling (runs in foreground)
	results, err := s.crawler.Crawl(startURL)
	if err != nil {
		return nil, err
	}

	return results, nil
}

func printStartupInfo(config ScraperConfig) {
	println()
	println("  ╔══════════════════════════════════════════════════╗")
	println("  ║           🕷️  SPIDERLY - Web Crawler             ║")
	println("  ╠══════════════════════════════════════════════════╣")
	println("  ║                                                  ║")
	println("  ║  Dashboard: http://localhost:" + strconv.Itoa(config.WebPort) + padRight("", 18-len(strconv.Itoa(config.WebPort))) + "║")
	println("  ║  Max Depth: " + strconv.Itoa(config.MaxDepth) + padRight("", 37-len(strconv.Itoa(config.MaxDepth))) + "║")
	println("  ║  Max Pages: " + strconv.Itoa(config.MaxPages) + padRight("", 37-len(strconv.Itoa(config.MaxPages))) + "║")
	println("  ║                                                  ║")
	println("  ║  Open your browser to see the live dashboard!    ║")
	println("  ║                                                  ║")
	println("  ╚══════════════════════════════════════════════════╝")
	println()
}

func padRight(s string, n int) string {
	if n <= 0 {
		return s
	}
	result := s
	for i := 0; i < n; i++ {
		result += " "
	}
	return result
}
