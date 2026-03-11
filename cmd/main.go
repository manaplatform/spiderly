// cmd/spiderly/main.go
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"math"
	"os"
	"sort"
	"strings"
	"time"

	"spiderly/internal/core"
)

func main() {
	// ─────────────────────────────────────────
	//  Basic Flags
	// ─────────────────────────────────────────
	targetURL := flag.String("url", "", "Target URL to crawl")
	maxPages := flag.Int("pages", 100, "Maximum number of pages to scrape")
	maxDepth := flag.Int("depth", 10, "Maximum crawl depth")
	concurrency := flag.Int("concurrency", 5, "Concurrent requests per worker")
	timeout := flag.Duration("timeout", 30*time.Second, "Request timeout")
	delay := flag.Duration("delay", 200*time.Millisecond, "Delay between requests")
	outputFile := flag.String("output", "", "Path for JSON output file")
	markdownFile := flag.String("markdown", "", "Path for Markdown output file")
	forceRecursive := flag.Bool("recursive", false, "Force recursive crawl (skip sitemap)")
	verbose := flag.Bool("verbose", false, "Enable verbose logging")
	noColor := flag.Bool("no-color", false, "Disable colored output")

	// ─────────────────────────────────────────
	//  Chunker Flags
	// ─────────────────────────────────────────
	chunked := flag.Bool("chunked", false, "Enable parallel chunked processing")
	chunkSize := flag.Int("chunk-size", 50, "URLs per chunk")
	workers := flag.Int("workers", 4, "Number of parallel workers")

	// ─────────────────────────────────────────
	//  Product Mode Flags
	// ─────────────────────────────────────────
	productMode := flag.Bool("product-mode", false, "Enable product-only crawl mode (auto-enables chunked)")
	sitemapURL := flag.String("sitemap", "", "Direct sitemap URL to use instead of auto-discovery")
	minPriority := flag.Float64("min-priority", 0, "Minimum sitemap priority to include (0.0 - 1.0)")
	urlPattern := flag.String("url-pattern", "", "Regex pattern to filter sitemap URLs")

	flag.Parse()

	// ─────────────────────────────────────────
	//  Validate Required Flags
	// ─────────────────────────────────────────
	if *targetURL == "" {
		printUsage()
		os.Exit(1)
	}

	// ─────────────────────────────────────────
	//  Product Mode: Auto-enable chunked
	// ─────────────────────────────────────────
	if *productMode {
		if !*chunked {
			*chunked = true
		}
		if *maxPages == 100 {
			// Raise default for product mode – users usually want everything
			*maxPages = 100000
		}
		if *chunkSize == 50 {
			*chunkSize = 200
		}
		if *workers == 4 {
			*workers = 8
		}
	}

	// ─────────────────────────────────────────
	//  Build CoreConfig
	// ─────────────────────────────────────────
	cfg := core.CoreConfig{
		TargetURL:      *targetURL,
		SitemapURL:     *sitemapURL,
		MaxPages:       *maxPages,
		MaxDepth:       *maxDepth,
		Concurrency:    *concurrency,
		Timeout:        *timeout,
		Delay:          *delay,
		MinPriority:    *minPriority,
		URLPattern:     *urlPattern,
		ForceRecursive: *forceRecursive,
		Verbose:        *verbose,
		NoColor:        *noColor,
		EnableChunker:  *chunked,
		ChunkSize:      *chunkSize,
		MaxWorkers:     *workers,
		ProductMode:    *productMode,
	}

	e := core.NewCore(cfg)

	// ─────────────────────────────────────────
	//  Run Crawl
	// ─────────────────────────────────────────
	results, err := e.Run()
	if err != nil {
		fmt.Printf("\n❌ Crawl failed: %v\n", err)
		os.Exit(1)
	}

	// Convert internal models → exported structs
	exported := core.ToScrapedPageResults(results)

	// ─────────────────────────────────────────
	//  Console Summary (always printed)
	// ─────────────────────────────────────────
	printSummary(exported)

	// ─────────────────────────────────────────
	//  Save Outputs
	// ─────────────────────────────────────────
	if *outputFile != "" {
		if err := saveJSON(exported, *outputFile); err != nil {
			fmt.Printf("⚠️  Failed to save JSON: %v\n", err)
		} else {
			fmt.Printf("📁 JSON saved to %s\n", *outputFile)
		}
	}

	if *markdownFile != "" {
		if err := saveMarkdown(exported, *markdownFile, *targetURL); err != nil {
			fmt.Printf("⚠️  Failed to save Markdown: %v\n", err)
		} else {
			fmt.Printf("📁 Markdown saved to %s\n", *markdownFile)
		}
	}
}

// ─────────────────────────────────────────────
//  Usage / Help
// ─────────────────────────────────────────────

func printUsage() {
	fmt.Println()
	fmt.Println("  🕷️  SPIDERLY - High Performance Web Crawler")
	fmt.Println()
	fmt.Println("  Usage: spiderly -url <target> [options]")
	fmt.Println()
	fmt.Println("  Basic Options:")
	fmt.Println("    -url string         Target URL to crawl (required)")
	fmt.Println("    -pages int          Maximum pages to scrape (default: 100)")
	fmt.Println("    -depth int          Maximum crawl depth (default: 10)")
	fmt.Println("    -concurrency int    Concurrent requests per worker (default: 5)")
	fmt.Println("    -timeout duration   Request timeout (default: 30s)")
	fmt.Println("    -delay duration     Delay between requests (default: 200ms)")
	fmt.Println()
	fmt.Println("  Sitemap Options:")
	fmt.Println("    -sitemap string     Direct sitemap URL (skip auto-discovery)")
	fmt.Println("    -min-priority float Minimum sitemap priority filter (0.0 - 1.0)")
	fmt.Println("    -url-pattern string Regex to filter sitemap URLs")
	fmt.Println()
	fmt.Println("  Product Mode (e-commerce):")
	fmt.Println("    -product-mode           Enable product-only crawl mode")
	fmt.Println("    -product-pattern string Custom regex for product URLs")
	fmt.Println()
	fmt.Println("  Chunker Options (Parallel Processing):")
	fmt.Println("    -chunked            Enable parallel chunked processing")
	fmt.Println("    -chunk-size int     URLs per chunk (default: 50, product-mode: 200)")
	fmt.Println("    -workers int        Number of parallel workers (default: 4, product-mode: 8)")
	fmt.Println()
	fmt.Println("  Output Options:")
	fmt.Println("    -output string      Path for JSON output file")
	fmt.Println("    -markdown string    Path for Markdown output file")
	fmt.Println()
	fmt.Println("  Other Options:")
	fmt.Println("    -recursive          Force recursive crawl (skip sitemap)")
	fmt.Println("    -verbose            Enable verbose logging")
	fmt.Println("    -no-color           Disable colored output")
	fmt.Println()
	fmt.Println("  Examples:")
	fmt.Println()
	fmt.Println("    # Basic crawl")
	fmt.Println("    spiderly -url example.com")
	fmt.Println()
	fmt.Println("    # High-performance chunked crawl")
	fmt.Println("    spiderly -url example.com -chunked -workers 8")
	fmt.Println()
	fmt.Println("    # Product mode — crawl ALL products from sitemap .xml.gz")
	fmt.Println("    spiderly -url https://www.technolife.com -product-mode -output products.json")
	fmt.Println()
	fmt.Println("    # Product mode with custom pattern")
	fmt.Println("    spiderly -url https://www.technolife.com -product-mode \\")
	fmt.Println("      -product-pattern \"/product/|/p/\" -workers 10 -pages 50000")
	fmt.Println()
	fmt.Println("    # Direct sitemap with filters")
	fmt.Println("    spiderly -url example.com -sitemap https://example.com/sitemap.xml \\")
	fmt.Println("      -min-priority 0.5 -url-pattern \"/blog/\"")
	fmt.Println()
	fmt.Println("    # Full output")
	fmt.Println("    spiderly -url example.com -pages 500 -chunked \\")
	fmt.Println("      -output results.json -markdown report.md -verbose")
	fmt.Println()
}

// ─────────────────────────────────────────────
//  JSON Export
// ─────────────────────────────────────────────

func saveJSON(pages []core.ScrapedPageResult, path string) error {
	data, err := json.MarshalIndent(pages, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal JSON: %w", err)
	}
	return os.WriteFile(path, data, 0644)
}

// ─────────────────────────────────────────────
//  Markdown Export (Enhanced with Product Data)
// ─────────────────────────────────────────────

func saveMarkdown(pages []core.ScrapedPageResult, path, sourceURL string) error {
	var sb strings.Builder

	now := time.Now().Format("2006-01-02 15:04:05")

	// ── Header ──────────────────────────────
	sb.WriteString("# 🕷️ Spiderly Crawl Report\n\n")
	sb.WriteString("| Field | Value |\n|---|---|\n")
	sb.WriteString(fmt.Sprintf("| **Source URL** | `%s` |\n", sourceURL))
	sb.WriteString(fmt.Sprintf("| **Generated** | %s |\n", now))
	sb.WriteString(fmt.Sprintf("| **Total Pages** | %d |\n", len(pages)))
	sb.WriteString("\n---\n\n")

	// ── Collect Statistics ──────────────────
	uniqueAuthors := map[string]bool{}
	tagFreq := map[string]int{}
	var totalSize int64
	var totalLoad int64
	statusCounts := map[int]int{}
	var maxDepth int

	// Product statistics
	var productPages []core.ScrapedPageResult
	var totalPrice float64
	var minPrice, maxPrice float64
	var priceCount int
	var inStockCount int
	brandFreq := map[string]int{}
	currencyFreq := map[string]int{}
	priceRanges := map[string]int{
		"0-1M":     0,
		"1M-10M":   0,
		"10M-50M":  0,
		"50M-100M": 0,
		"100M+":    0,
	}

	for _, p := range pages {
		if p.Author != "" {
			uniqueAuthors[p.Author] = true
		}
		if p.Keywords != "" {
			for _, kw := range strings.Split(p.Keywords, ",") {
				kw = strings.TrimSpace(kw)
				if kw != "" {
					tagFreq[kw]++
				}
			}
		}
		totalSize += p.ContentLength
		totalLoad += p.LoadTimeMs
		statusCounts[p.StatusCode]++
		if p.Depth > maxDepth {
			maxDepth = p.Depth
		}

		// Collect product stats
		if p.Product != nil {
			productPages = append(productPages, p)

			if p.Product.Price > 0 {
				priceCount++
				totalPrice += p.Product.Price

				if minPrice == 0 || p.Product.Price < minPrice {
					minPrice = p.Product.Price
				}
				if p.Product.Price > maxPrice {
					maxPrice = p.Product.Price
				}

				// Price range buckets (assuming IRR or similar large currency)
				categorizePrice(p.Product.Price, priceRanges)
			}

			if p.Product.InStock {
				inStockCount++
			}

			if p.Product.Brand != "" {
				brandFreq[p.Product.Brand]++
			}

			if p.Product.Currency != "" {
				currencyFreq[p.Product.Currency]++
			}
		}
	}

	// ── General Statistics ──────────────────
	sb.WriteString("## 📊 General Statistics\n\n")
	sb.WriteString("| Metric | Value |\n|---|---|\n")
	sb.WriteString(fmt.Sprintf("| Total pages scraped | **%d** |\n", len(pages)))
	sb.WriteString(fmt.Sprintf("| Unique authors | **%d** |\n", len(uniqueAuthors)))
	sb.WriteString(fmt.Sprintf("| Unique keywords | **%d** |\n", len(tagFreq)))
	sb.WriteString(fmt.Sprintf("| Max crawl depth | **%d** |\n", maxDepth))
	sb.WriteString(fmt.Sprintf("| Total content size | **%s** |\n", humanizeBytes(totalSize)))
	if len(pages) > 0 {
		sb.WriteString(fmt.Sprintf("| Avg load time | **%d ms** |\n", totalLoad/int64(len(pages))))
	}
	sb.WriteString("\n")

	// ── Product Summary (if products found) ─
	if len(productPages) > 0 {
		sb.WriteString("---\n\n")
		sb.WriteString("## 🛒 Product Summary\n\n")

		// Determine primary currency
		primaryCurrency := getPrimaryCurrency(currencyFreq)

		sb.WriteString("| Metric | Value |\n|---|---|\n")
		sb.WriteString(fmt.Sprintf("| Products Found | **%d** |\n", len(productPages)))
		sb.WriteString(fmt.Sprintf("| With Price Data | **%d** |\n", priceCount))
		sb.WriteString(fmt.Sprintf("| In Stock | **%d** (%.1f%%) |\n", inStockCount, float64(inStockCount)/float64(len(productPages))*100))

		if priceCount > 0 {
			avgPrice := totalPrice / float64(priceCount)
			sb.WriteString(fmt.Sprintf("| Avg Price | **%s** |\n", formatPrice(avgPrice, primaryCurrency)))
			sb.WriteString(fmt.Sprintf("| Min Price | **%s** |\n", formatPrice(minPrice, primaryCurrency)))
			sb.WriteString(fmt.Sprintf("| Max Price | **%s** |\n", formatPrice(maxPrice, primaryCurrency)))
		}

		sb.WriteString(fmt.Sprintf("| Primary Currency | **%s** |\n", primaryCurrency))
		sb.WriteString("\n")

		// ── Price Distribution ──────────────
		if priceCount > 0 {
			sb.WriteString("### 💰 Price Distribution\n\n")
			sb.WriteString("| Range | Count | Bar |\n|---|---|---|\n")

			priceRangeOrder := []string{"0-1M", "1M-10M", "10M-50M", "50M-100M", "100M+"}
			maxRangeCount := 0
			for _, count := range priceRanges {
				if count > maxRangeCount {
					maxRangeCount = count
				}
			}

			for _, rangeName := range priceRangeOrder {
				count := priceRanges[rangeName]
				if count > 0 || maxRangeCount > 0 {
					barLen := 0
					if maxRangeCount > 0 {
						barLen = int(float64(count) * 20 / float64(maxRangeCount))
					}
					bar := strings.Repeat("█", barLen)
					sb.WriteString(fmt.Sprintf("| %s | %d | %s |\n", rangeName, count, bar))
				}
			}
			sb.WriteString("\n")
		}

		// ── Top Brands ──────────────────────
		if len(brandFreq) > 0 {
			sb.WriteString("### 🏢 Top Brands\n\n")
			topBrands := sortMapByValue(brandFreq, 10)
			maxBrandCount := topBrands[0].count
			for _, b := range topBrands {
				barLen := int(float64(b.count) * 20 / float64(maxBrandCount))
				bar := strings.Repeat("▓", barLen)
				sb.WriteString(fmt.Sprintf("- **%s** (%d) %s\n", b.key, b.count, bar))
			}
			sb.WriteString("\n")
		}

		// ── Products Price Table ────────────
		sb.WriteString("### 📦 All Products with Prices\n\n")
		sb.WriteString("| # | Product | Brand | Price | Stock | Rating |\n")
		sb.WriteString("|---|---|---|---|---|---|\n")

		// Sort products by price (highest first)
		sortedProducts := sortProductsByPrice(productPages)

		for i, p := range sortedProducts {
			if p.Product == nil {
				continue
			}

			productName := truncate(getProductName(p), 40)
			brand := p.Product.Brand
			if brand == "" {
				brand = "-"
			}

			priceStr := "-"
			if p.Product.Price > 0 {
				priceStr = formatPrice(p.Product.Price, p.Product.Currency)
				if p.Product.OriginalPrice > 0 && p.Product.Discount > 0 {
					priceStr = fmt.Sprintf("%s ~~%s~~ (%.0f%% off)",
						priceStr,
						formatPrice(p.Product.OriginalPrice, p.Product.Currency),
						p.Product.Discount)
				}
			}

			stockStatus := "❌"
			if p.Product.InStock {
				stockStatus = "✅"
			}

			ratingStr := "-"
			if p.Product.Rating > 0 {
				ratingStr = fmt.Sprintf("⭐ %.1f", p.Product.Rating)
				if p.Product.ReviewCount > 0 {
					ratingStr += fmt.Sprintf(" (%d)", p.Product.ReviewCount)
				}
			}

			sb.WriteString(fmt.Sprintf("| %d | %s | %s | %s | %s | %s |\n",
				i+1,
				escapeMDTableCell(productName),
				escapeMDTableCell(brand),
				priceStr,
				stockStatus,
				ratingStr,
			))
		}
		sb.WriteString("\n")
	}

	// ── Status Code Distribution ────────────
	sb.WriteString("---\n\n")
	sb.WriteString("## 📡 HTTP Status Codes\n\n")
	if len(statusCounts) > 0 {
		sb.WriteString("| Status | Count | Bar |\n|---|---|---|\n")
		statusKeys := sortedIntKeys(statusCounts)
		for _, code := range statusKeys {
			count := statusCounts[code]
			bar := strings.Repeat("█", int(math.Ceil(float64(count)*30/float64(len(pages)))))
			emoji := statusEmoji(code)
			sb.WriteString(fmt.Sprintf("| %s %d | %d | %s |\n", emoji, code, count, bar))
		}
		sb.WriteString("\n")
	}

	// ── Top Keywords ────────────────────────
	if len(tagFreq) > 0 {
		sb.WriteString("### 🏷️ Top Keywords\n\n")
		topTags := sortMapByValue(tagFreq, 15)
		maxCount := topTags[0].count
		for _, t := range topTags {
			barLen := int(math.Ceil(float64(t.count) * 25 / float64(maxCount)))
			bar := strings.Repeat("▓", barLen)
			sb.WriteString(fmt.Sprintf("- **%s** (%d) %s\n", t.key, t.count, bar))
		}
		sb.WriteString("\n")
	}

	// ── Table of Contents ───────────────────
	sb.WriteString("---\n\n")
	sb.WriteString("## 📑 Table of Contents\n\n")
	for i, p := range pages {
		title := pageTitle(p)
		anchor := fmt.Sprintf("page-%d", i+1)
		sb.WriteString(fmt.Sprintf("%d. [%s](#%s)\n", i+1, escapeMDTableCell(title), anchor))
	}
	sb.WriteString("\n---\n\n")

	// ── Individual Pages Detail ─────────────
	sb.WriteString("## 📄 Pages Detail\n\n")

	for i, p := range pages {
		anchor := fmt.Sprintf("page-%d", i+1)
		title := pageTitle(p)

		sb.WriteString(fmt.Sprintf("<a id=\"%s\"></a>\n\n", anchor))
		sb.WriteString(fmt.Sprintf("### %d. %s %s\n\n", i+1, statusEmoji(p.StatusCode), escapeMDTableCell(title)))

		// Metadata table
		sb.WriteString("| Field | Value |\n|---|---|\n")
		sb.WriteString(fmt.Sprintf("| 🔗 URL | `%s` |\n", p.URL))
		sb.WriteString(fmt.Sprintf("| 📶 Status | %d |\n", p.StatusCode))
		sb.WriteString(fmt.Sprintf("| 📐 Depth | %d |\n", p.Depth))

		if p.PageType != "" {
			sb.WriteString(fmt.Sprintf("| 📋 Page Type | `%s` |\n", p.PageType))
		}
		if p.ContentType != "" {
			sb.WriteString(fmt.Sprintf("| 📦 Content-Type | `%s` |\n", escapeMDTableCell(p.ContentType)))
		}
		if p.ContentLength > 0 {
			sb.WriteString(fmt.Sprintf("| 📏 Size | %s |\n", humanizeBytes(p.ContentLength)))
		}
		if p.LoadTimeMs > 0 {
			sb.WriteString(fmt.Sprintf("| ⏱️ Load Time | %d ms |\n", p.LoadTimeMs))
		}
		if p.Author != "" {
			sb.WriteString(fmt.Sprintf("| ✍️ Author | %s |\n", escapeMDTableCell(p.Author)))
		}
		if p.PublishedDate != "" {
			sb.WriteString(fmt.Sprintf("| 📅 Published | %s |\n", escapeMDTableCell(p.PublishedDate)))
		}
		if p.LinksCount > 0 {
			sb.WriteString(fmt.Sprintf("| 🔗 Links | %d |\n", p.LinksCount))
		}
		if p.ImagesCount > 0 {
			sb.WriteString(fmt.Sprintf("| 🖼️ Images | %d |\n", p.ImagesCount))
		}
		if !p.ScrapedAt.IsZero() {
			sb.WriteString(fmt.Sprintf("| 🕐 Scraped | %s |\n", p.ScrapedAt.Format("2006-01-02 15:04:05")))
		}
		sb.WriteString("\n")

		// ── Product Details Section ─────────
		if p.Product != nil {
			sb.WriteString("#### 🛍️ Product Information\n\n")
			sb.WriteString("| Field | Value |\n|---|---|\n")

			if p.Product.Name != "" {
				sb.WriteString(fmt.Sprintf("| 📦 Name | **%s** |\n", escapeMDTableCell(p.Product.Name)))
			}
			if p.Product.Brand != "" {
				sb.WriteString(fmt.Sprintf("| 🏢 Brand | %s |\n", escapeMDTableCell(p.Product.Brand)))
			}
			if p.Product.SKU != "" {
				sb.WriteString(fmt.Sprintf("| 🔖 SKU | `%s` |\n", p.Product.SKU))
			}
			if p.Product.GTIN != "" {
				sb.WriteString(fmt.Sprintf("| 📊 GTIN | `%s` |\n", p.Product.GTIN))
			}
			if p.Product.MPN != "" {
				sb.WriteString(fmt.Sprintf("| 🔢 MPN | `%s` |\n", p.Product.MPN))
			}

			// Price section with highlighting
			if p.Product.Price > 0 {
				priceDisplay := formatPrice(p.Product.Price, p.Product.Currency)
				sb.WriteString(fmt.Sprintf("| 💰 **Price** | **%s** |\n", priceDisplay))

				if p.Product.OriginalPrice > 0 && p.Product.OriginalPrice > p.Product.Price {
					origDisplay := formatPrice(p.Product.OriginalPrice, p.Product.Currency)
					sb.WriteString(fmt.Sprintf("| 🏷️ Original Price | ~~%s~~ |\n", origDisplay))
				}
				if p.Product.Discount > 0 {
					sb.WriteString(fmt.Sprintf("| 📉 Discount | **%.1f%%** |\n", p.Product.Discount))
				}
			}

			// Stock status
			stockDisplay := "❌ Out of Stock"
			if p.Product.InStock {
				stockDisplay = "✅ In Stock"
			}
			if p.Product.Availability != "" {
				stockDisplay = fmt.Sprintf("%s (%s)", stockDisplay, p.Product.Availability)
			}
			sb.WriteString(fmt.Sprintf("| 📦 Stock | %s |\n", stockDisplay))

			// Rating
			if p.Product.Rating > 0 {
				stars := strings.Repeat("⭐", int(p.Product.Rating))
				ratingText := fmt.Sprintf("%s %.1f/5", stars, p.Product.Rating)
				if p.Product.ReviewCount > 0 {
					ratingText += fmt.Sprintf(" (%d reviews)", p.Product.ReviewCount)
				}
				sb.WriteString(fmt.Sprintf("| ⭐ Rating | %s |\n", ratingText))
			}

			// Category
			if p.Product.Category != "" {
				sb.WriteString(fmt.Sprintf("| 📁 Category | %s |\n", escapeMDTableCell(p.Product.Category)))
			} else if len(p.Product.Categories) > 0 {
				sb.WriteString(fmt.Sprintf("| 📁 Categories | %s |\n", escapeMDTableCell(strings.Join(p.Product.Categories, " > "))))
			}

			sb.WriteString("\n")

			// Product Description
			if p.Product.Description != "" {
				sb.WriteString("**Product Description:**\n\n")
				desc := truncate(p.Product.Description, 500)
				sb.WriteString(fmt.Sprintf("> %s\n\n", escapeMDTableCell(desc)))
			}

			// Product Images
			if len(p.Product.Images) > 0 {
				sb.WriteString("**Product Images:**\n\n")
				for j, img := range p.Product.Images {
					if j >= 5 { // Limit to 5 images
						sb.WriteString(fmt.Sprintf("- ... and %d more images\n", len(p.Product.Images)-5))
						break
					}
					sb.WriteString(fmt.Sprintf("- ![Image %d](%s)\n", j+1, img))
				}
				sb.WriteString("\n")
			}

			// Product Specifications
			if len(p.Product.Specs) > 0 {
				sb.WriteString("**Specifications:**\n\n")
				sb.WriteString("| Spec | Value |\n|---|---|\n")
				for specKey, specVal := range p.Product.Specs {
					sb.WriteString(fmt.Sprintf("| %s | %s |\n",
						escapeMDTableCell(specKey),
						escapeMDTableCell(specVal)))
				}
				sb.WriteString("\n")
			}
		}

		// H1 (if different from title)
		if p.H1 != "" && p.H1 != p.Title {
			sb.WriteString(fmt.Sprintf("**H1:** %s\n\n", escapeMDTableCell(p.H1)))
		}

		// Description
		if p.Description != "" && (p.Product == nil || p.Product.Description != p.Description) {
			sb.WriteString(fmt.Sprintf("> %s\n\n", escapeMDTableCell(p.Description)))
		}

		// Keywords
		if p.Keywords != "" {
			keywords := strings.Split(p.Keywords, ",")
			var badges []string
			for _, kw := range keywords {
				kw = strings.TrimSpace(kw)
				if kw != "" {
					badges = append(badges, fmt.Sprintf("`%s`", kw))
				}
			}
			if len(badges) > 0 {
				sb.WriteString(fmt.Sprintf("**Keywords:** %s\n\n", strings.Join(badges, " ")))
			}
		}

		// OG Image (if no product images)
		if p.OGImage != "" && (p.Product == nil || len(p.Product.Images) == 0) {
			sb.WriteString(fmt.Sprintf("**Featured Image:** ![og](%s)\n\n", p.OGImage))
		}

		// Body text preview (only for non-product pages or if no product description)
		if p.BodyText != "" && (p.Product == nil || p.Product.Description == "") {
			preview := truncate(p.BodyText, 1000)
			paragraphs := splitParagraphs(preview, 3)
			sb.WriteString("**Content Preview:**\n\n")
			for _, para := range paragraphs {
				sb.WriteString(fmt.Sprintf("%s\n\n", escapeMDTableCell(para)))
			}
		}

		sb.WriteString("[⬆ Back to top](#-spiderly-crawl-report)\n\n")
		sb.WriteString("---\n\n")
	}

	// ── Footer ──────────────────────────────
	sb.WriteString(fmt.Sprintf("*Generated by Spiderly on %s*\n", now))

	return writeFile(path, []byte(sb.String()))
}

// ─────────────────────────────────────────────
//  Product Helper Functions
// ─────────────────────────────────────────────

func categorizePrice(price float64, ranges map[string]int) {
	switch {
	case price < 1_000_000:
		ranges["0-1M"]++
	case price < 10_000_000:
		ranges["1M-10M"]++
	case price < 50_000_000:
		ranges["10M-50M"]++
	case price < 100_000_000:
		ranges["50M-100M"]++
	default:
		ranges["100M+"]++
	}
}

func formatPrice(price float64, currency string) string {
	if currency == "" {
		currency = "IRR"
	}

	// Format with thousand separators
	priceStr := formatNumber(price)

	// Currency symbol mapping
	symbols := map[string]string{
		"IRR": "﷼",
		"USD": "$",
		"EUR": "€",
		"GBP": "£",
		"IRT": "تومان",
	}

	symbol, ok := symbols[currency]
	if !ok {
		symbol = currency
	}

	return fmt.Sprintf("%s %s", priceStr, symbol)
}

func formatNumber(n float64) string {
	// Simple thousand separator
	str := fmt.Sprintf("%.0f", n)
	result := ""
	for i, c := range str {
		if i > 0 && (len(str)-i)%3 == 0 {
			result += ","
		}
		result += string(c)
	}
	return result
}

func getPrimaryCurrency(freq map[string]int) string {
	if len(freq) == 0 {
		return "IRR"
	}

	maxCount := 0
	primary := "IRR"
	for currency, count := range freq {
		if count > maxCount {
			maxCount = count
			primary = currency
		}
	}
	return primary
}

func getProductName(p core.ScrapedPageResult) string {
	if p.Product != nil && p.Product.Name != "" {
		return p.Product.Name
	}
	if p.Title != "" {
		return p.Title
	}
	if p.H1 != "" {
		return p.H1
	}
	return p.URL
}

func sortProductsByPrice(products []core.ScrapedPageResult) []core.ScrapedPageResult {
	sorted := make([]core.ScrapedPageResult, len(products))
	copy(sorted, products)

	sort.Slice(sorted, func(i, j int) bool {
		priceI := float64(0)
		priceJ := float64(0)
		if sorted[i].Product != nil {
			priceI = sorted[i].Product.Price
		}
		if sorted[j].Product != nil {
			priceJ = sorted[j].Product.Price
		}
		return priceI > priceJ // Highest price first
	})

	return sorted
}

// ─────────────────────────────────────────────
//  Console Summary
// ─────────────────────────────────────────────

func printSummary(pages []core.ScrapedPageResult) {
	fmt.Println()
	fmt.Println("╔══════════════════════════════════════════════════════════╗")
	fmt.Println("║               🕷️  Spiderly — Crawl Summary              ║")
	fmt.Println("╠══════════════════════════════════════════════════════════╣")
	fmt.Printf("║  Total pages scraped: %-34d ║\n", len(pages))
	fmt.Println("╠══════════════════════════════════════════════════════════╣")

	statusCounts := map[int]int{}
	var totalSize int64
	var totalLoad int64

	for _, p := range pages {
		statusCounts[p.StatusCode]++
		totalSize += p.ContentLength
		totalLoad += p.LoadTimeMs
	}

	// Status summary
	statusKeys := sortedIntKeys(statusCounts)
	for _, code := range statusKeys {
		emoji := statusEmoji(code)
		fmt.Printf("║  %s HTTP %d: %-38d ║\n", emoji, code, statusCounts[code])
	}

	if len(pages) > 0 {
		avgLoad := totalLoad / int64(len(pages))
		fmt.Printf("║  📏 Total size: %-40s ║\n", humanizeBytes(totalSize))
		fmt.Printf("║  ⏱️  Avg load time: %-36s ║\n", fmt.Sprintf("%d ms", avgLoad))
	}

	fmt.Println("╠══════════════════════════════════════════════════════════╣")

	// Show first 20 pages
	limit := 20
	if len(pages) < limit {
		limit = len(pages)
	}

	for i := 0; i < limit; i++ {
		p := pages[i]
		title := pageTitle(p)
		if len(title) > 42 {
			title = title[:39] + "..."
		}
		status := fmt.Sprintf("[%d]", p.StatusCode)
		fmt.Printf("║  %-6s %-49s ║\n", status, title)

		if p.Author != "" || p.PublishedDate != "" {
			meta := ""
			if p.Author != "" {
				meta += "✍️ " + p.Author
			}
			if p.PublishedDate != "" {
				if meta != "" {
					meta += "  |  "
				}
				meta += "📅 " + p.PublishedDate
			}
			if len(meta) > 53 {
				meta = meta[:50] + "..."
			}
			fmt.Printf("║         %-48s ║\n", meta)
		}
	}

	if len(pages) > limit {
		fmt.Printf("║  ... and %d more pages %-33s ║\n", len(pages)-limit, "")
	}

	fmt.Println("╚══════════════════════════════════════════════════════════╝")
	fmt.Println()
}

// ─────────────────────────────────────────────
//  Helpers
// ─────────────────────────────────────────────

func pageTitle(p core.ScrapedPageResult) string {
	if p.Title != "" {
		return p.Title
	}
	if p.H1 != "" {
		return p.H1
	}
	if p.Description != "" {
		return truncate(p.Description, 60)
	}
	return p.URL
}

func truncate(s string, maxLen int) string {
	s = strings.TrimSpace(s)
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}

func escapeMDTableCell(s string) string {
	s = strings.ReplaceAll(s, "|", "\\|")
	s = strings.ReplaceAll(s, "\n", " ")
	s = strings.ReplaceAll(s, "\r", "")
	return strings.TrimSpace(s)
}

func splitParagraphs(text string, max int) []string {
	raw := strings.Split(text, "\n")
	var out []string
	for _, line := range raw {
		line = strings.TrimSpace(line)
		if line != "" {
			out = append(out, line)
		}
		if len(out) >= max {
			break
		}
	}
	return out
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

type kv struct {
	key   string
	count int
}

func sortMapByValue(m map[string]int, limit int) []kv {
	pairs := make([]kv, 0, len(m))
	for k, v := range m {
		pairs = append(pairs, kv{k, v})
	}
	sort.Slice(pairs, func(i, j int) bool {
		return pairs[i].count > pairs[j].count
	})
	if len(pairs) > limit {
		pairs = pairs[:limit]
	}
	return pairs
}

func sortedIntKeys(m map[int]int) []int {
	keys := make([]int, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Ints(keys)
	return keys
}

func writeFile(path string, data []byte) error {
	return os.WriteFile(path, data, 0644)
}
