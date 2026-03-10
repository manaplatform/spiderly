package scraper

import (
	"context"
	"fmt"
	"time"

	"spiderly/internal/crawler"
	"spiderly/internal/models"
	"spiderly/internal/web"
)

type Extractor struct {
	crawler *crawler.Crawler
	server  *web.Server
	config  crawler.Config
}

func NewExtractor(config crawler.Config, port int) *Extractor {

	server := web.NewServer(port)
	c := crawler.NewCrawler(config)

	ext := &Extractor{
		crawler: c,
		server:  server,
		config:  config,
	}

	ext.bindCallbacks()

	return ext
}

func (e *Extractor) bindCallbacks() {

	// NEWS
	e.crawler.SetNewsCallback(func(news models.News) {
		e.server.BroadcastNews(news)
	})

	// LOGS
	e.crawler.SetLogCallback(func(level, msg string) {

		e.server.BroadcastLog(level, msg)

		// also print to console
		fmt.Printf("[%s] %s\n", level, msg)
	})

	// STATS
	e.crawler.SetStatsCallback(func(stats models.CrawlStats) {
		e.server.BroadcastStats(stats)
	})

	// LINKS
	e.crawler.SetLinkCallback(func(link models.DiscoveredLink) {
		e.server.BroadcastLink(link)
	})

	// PROGRESS
	e.crawler.SetProgressCallback(func(p float64) {
		e.server.BroadcastProgress(p)
	})
}

func (e *Extractor) Start(ctx context.Context, startURL string) error {

	// Start dashboard
	go func() {

		fmt.Printf("\n🖥️  Dashboard running at:\n")
		fmt.Printf("   http://localhost:%d\n\n", e.server.GetPort())

		if err := e.server.Start(); err != nil {
			fmt.Println("dashboard error:", err)
		}

	}()

	// small delay so dashboard is ready
	time.Sleep(1 * time.Second)

	fmt.Println("🚀 Spiderly crawler starting...")

	return e.crawler.Start(ctx, startURL)
}
