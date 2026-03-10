package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"spiderly/internal/crawler"
	"spiderly/internal/scraper"
)


func main() {
	// Parse command-line flags
	startURL := flag.String("url", "", "Starting URL to crawl (required)")
	maxDepth := flag.Int("depth", 2, "Maximum crawl depth")
	maxPages := flag.Int("pages", 10, "Maximum number of pages to crawl")
	timeout := flag.Int("timeout", 30, "Request timeout in seconds")
	followExternal := flag.Bool("external", false, "Follow external links")
	delay := flag.Int("delay", 500, "Delay between requests in milliseconds")
	port := flag.Int("port", 8080, "Dashboard web server port")
	
	// Sitemap mode flags
	sitemapMode := flag.Bool("sitemap", false, "Enable sitemap mode (discover and parse sitemaps)")
	sitemapURL := flag.String("sitemap-url", "", "Direct sitemap URL to parse (implies -sitemap)")
	minPriority := flag.Float64("min-priority", 0, "Minimum sitemap URL priority (0-1)")
	urlPattern := flag.String("url-pattern", "", "Regex pattern to filter sitemap URLs")

	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, `
╔═══════════════════════════════════════════════════════════════╗
║                     🕷️  SPIDERLY v1.0                         ║
║              Advanced Web Crawler with Dashboard              ║
╚═══════════════════════════════════════════════════════════════╝

Usage: spiderly [options]

BASIC OPTIONS:
  -url string        Starting URL to crawl (required)
  -depth int         Maximum crawl depth (default: 2)
  -pages int         Maximum pages to crawl (default: 10)
  -timeout int       Request timeout in seconds (default: 30)
  -delay int         Delay between requests in ms (default: 500)
  -external          Follow external links (default: false)
  -port int          Dashboard port (default: 8080)

SITEMAP MODE OPTIONS:
  -sitemap           Enable sitemap discovery mode
  -sitemap-url       Parse a specific sitemap URL directly
  -min-priority      Filter URLs by minimum priority (0.0-1.0)
  -url-pattern       Regex pattern to filter URLs

EXAMPLES:
  # Recursive crawl mode
  spiderly -url "https://example.com" -depth 2 -pages 20

  # Sitemap discovery mode
  spiderly -url "https://example.com" -sitemap -pages 50

  # Direct sitemap parsing
  spiderly -sitemap-url "https://example.com/sitemap.xml" -pages 100

  # Filtered sitemap crawl (news articles with high priority)
  spiderly -url "https://example.com" -sitemap -min-priority 0.8 -url-pattern "/news/"

`)
		flag.PrintDefaults()
	}

	flag.Parse()

	// Validate required flags
	if *startURL == "" && *sitemapURL == "" {
		fmt.Println("❌ Error: -url or -sitemap-url is required")
		flag.Usage()
		os.Exit(1)
	}

	// If sitemap-url is provided, enable sitemap mode
	if *sitemapURL != "" {
		*sitemapMode = true
	}

	// Use sitemap-url as start URL if url not provided
	effectiveURL := *startURL
	if effectiveURL == "" && *sitemapURL != "" {
		effectiveURL = *sitemapURL
	}

	// Create crawler configuration
	config := crawler.Config{
		MaxDepth:       *maxDepth,
		MaxPages:       *maxPages,
		Timeout:        time.Duration(*timeout) * time.Second,
		FollowExternal: *followExternal,
		Delay:          time.Duration(*delay) * time.Millisecond,
		SitemapMode:    *sitemapMode,
		SitemapURL:     *sitemapURL,
		MinPriority:    *minPriority,
		URLPattern:     *urlPattern,
	}

	// Create scraper orchestrator
	ext := scraper.NewExtractor(config, *port)

	// Setup graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigChan
		fmt.Println("\n🛑 Shutting down gracefully...")
		cancel()
	}()

	// Start the extractor
	if err := ext.Start(ctx, effectiveURL); err != nil {
		fmt.Printf("❌ Error: %v\n", err)
		os.Exit(1)
	}
}
