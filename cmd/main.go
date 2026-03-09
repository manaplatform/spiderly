package main

import (
	"flag"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"spiderly/internal/scraper"
)

func main() {
	// Command line flags
	urlFlag := flag.String("url", "", "آدرس URL هدف (مثال: example.com)")
	depthFlag := flag.Int("depth", 2, "حداکثر عمق خزش")
	pagesFlag := flag.Int("pages", 10, "حداکثر تعداد صفحات")
	timeoutFlag := flag.Int("timeout", 30, "زمان انتظار (ثانیه)")
	portFlag := flag.Int("port", 8080, "پورت داشبورد وب")
	externalFlag := flag.Bool("external", false, "دنبال کردن لینک‌های خارجی")

	flag.Usage = func() {
		fmt.Println(`
  ╔══════════════════════════════════════════════════╗
  ║           🕷️  SPIDERLY - Web Crawler             ║
  ╚══════════════════════════════════════════════════╝

  Usage: spiderly -url <URL> [options]

  Options:`)
		flag.PrintDefaults()
		fmt.Println(`
  Examples:
    spiderly -url example.com
    spiderly -url https://news.site.com -depth 3 -pages 20
    spiderly -url blog.example.com -port 9090`)
	}

	flag.Parse()

	if *urlFlag == "" {
		fmt.Println("\n  ❌ Error: Please provide a URL with -url flag")
		flag.Usage()
		os.Exit(1)
	}

	// Prepare URL
	targetURL := strings.TrimSpace(*urlFlag)
	if !strings.HasPrefix(targetURL, "http://") && !strings.HasPrefix(targetURL, "https://") {
		targetURL = "https://" + targetURL
	}

	// Create scraper configuration
	config := scraper.ScraperConfig{
		MaxDepth:       *depthFlag,
		MaxPages:       *pagesFlag,
		Timeout:        time.Duration(*timeoutFlag) * time.Second,
		WaitTime:       2 * time.Second,
		FollowExternal: *externalFlag,
		WebPort:        *portFlag,
	}

	// Handle graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigChan
		fmt.Println("\n\n  🛑 Shutting down gracefully...")
		os.Exit(0)
	}()

	// Create and run scraper
	s := scraper.NewScraper(config)
	_, err := s.Run(targetURL)
	if err != nil {
		fmt.Printf("\n  ❌ Error: %v\n", err)
		os.Exit(1)
	}

	// Keep server alive after crawling finishes
	fmt.Println("\n  ✅ Crawling finished! Dashboard is still available.")
	fmt.Println("  Press Ctrl+C to exit.\n")

	// Block forever until Ctrl+C
	select {}
}
