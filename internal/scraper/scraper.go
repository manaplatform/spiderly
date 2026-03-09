package scraper

import (
	"context"
	"log"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/chromedp/chromedp"
)

// ExtractData fetches a JS-rendered page and extracts content.
func ExtractData(url string) {
	// Create a new Chrome context
	ctx, cancel := chromedp.NewContext(context.Background())
	defer cancel()

	// Set a timeout so it doesn't hang forever
	ctx, cancel = context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	var htmlContent string

	// Navigate to the page and wait for content to load
	err := chromedp.Run(ctx,
		chromedp.Navigate(url),
		// Wait for the content to appear (adjust selector to match the site)
		chromedp.WaitVisible("body", chromedp.ByQuery),
		// Give JS extra time to render
		chromedp.Sleep(3*time.Second),
		// Get the fully rendered HTML
		chromedp.OuterHTML("html", &htmlContent),
	)
	if err != nil {
		log.Printf("Error rendering page: %s", err)
		return
	}

	log.Printf("Rendered HTML length: %d", len(htmlContent))

	// Now parse with goquery
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(htmlContent))
	if err != nil {
		log.Printf("Error parsing HTML: %s", err)
		return
	}

	// Try multiple selectors to find the content
	// You'll need to inspect the rendered page to find the correct selectors
	selectors := []string{
		"div.content",
		"article",
		"div.post-content",
		"div.entry-content",
		"div.article-body",
		"main",
	}

	for _, sel := range selectors {
		doc.Find(sel).Each(func(i int, s *goquery.Selection) {
			title := s.Find("h1").Text()
			content := s.Find("p").Text()

			if strings.TrimSpace(title) != "" || strings.TrimSpace(content) != "" {
				log.Printf("Selector: %s", sel)
				log.Printf("Title: %s", strings.TrimSpace(title))
				log.Printf("Content: %.500s...", strings.TrimSpace(content))
			}
		})
	}
}
