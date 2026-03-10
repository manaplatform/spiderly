// cmd/spiderly/main.go
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"time"
	"math"
	"strings"
	"sort"
	"spiderly/internal/core"
)

func main() {
	// Flags
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
	
	// Chunker flags
	chunked := flag.Bool("chunked", false, "Enable parallel chunked processing")
	chunkSize := flag.Int("chunk-size", 50, "URLs per chunk")
	workers := flag.Int("workers", 4, "Number of parallel workers")
	
	flag.Parse()

	if *targetURL == "" {
		printUsage()
		os.Exit(1)
	}

	// Create core with config
	e := core.NewCore(core.CoreConfig{
		TargetURL:      *targetURL,
		MaxPages:       *maxPages,
		MaxDepth:       *maxDepth,
		Concurrency:    *concurrency,
		Timeout:        *timeout,
		Delay:          *delay,
		ForceRecursive: *forceRecursive,
		Verbose:        *verbose,
		NoColor:        *noColor,
		EnableChunker:  *chunked,
		ChunkSize:      *chunkSize,
		MaxWorkers:     *workers,
	})

	// Run crawl
	results, err := e.Run()
	if err != nil {
		fmt.Printf("\n❌ Crawl failed: %v\n", err)
		os.Exit(1)
	}

	// Convert to exported format
	exported := core.ToScrapedPageResults(results)

	// Save outputs
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

func printUsage() {
	fmt.Println()
	fmt.Println("  🕷️  SPIDERLY - High Performance Web Crawler")
	fmt.Println()
	fmt.Println("  Usage: spiderly -url <target> [options]")
	fmt.Println()
	fmt.Println("  Basic Options:")
	fmt.Println("    -url string        Target URL to crawl (required)")
	fmt.Println("    -pages int         Maximum pages to scrape (default: 100)")
	fmt.Println("    -depth int         Maximum crawl depth (default: 10)")
	fmt.Println("    -concurrency int   Concurrent requests per worker (default: 5)")
	fmt.Println("    -timeout duration  Request timeout (default: 30s)")
	fmt.Println("    -delay duration    Delay between requests (default: 200ms)")
	fmt.Println()
	fmt.Println("  Chunker Options (Parallel Processing):")
	fmt.Println("    -chunked           Enable parallel chunked processing")
	fmt.Println("    -chunk-size int    URLs per chunk (default: 50)")
	fmt.Println("    -workers int       Number of parallel workers (default: 4)")
	fmt.Println()
	fmt.Println("  Output Options:")
	fmt.Println("    -output string     Path for JSON output file")
	fmt.Println("    -markdown string   Path for Markdown output file")
	fmt.Println()
	fmt.Println("  Other Options:")
	fmt.Println("    -recursive         Force recursive crawl (skip sitemap)")
	fmt.Println("    -verbose           Enable verbose logging")
	fmt.Println("    -no-color          Disable colored output")
	fmt.Println()
	fmt.Println("  Examples:")
	fmt.Println("    spiderly -url example.com")
	fmt.Println("    spiderly -url example.com -chunked -workers 8")
	fmt.Println("    spiderly -url example.com -pages 500 -chunked -chunk-size 100 -workers 5")
	fmt.Println("    spiderly -url example.com -output results.json -verbose")
	fmt.Println()
}

func saveJSON(pages []core.ScrapedPageResult, path string) error {
	data, err := json.MarshalIndent(pages, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal JSON: %w", err)
	}
	return os.WriteFile(path, data, 0644)
}

// [saveMarkdown and helper functions remain the same as before...]


// ─────────────────────────────────────────────
//  Markdown Export
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

	// ── Statistics ──────────────────────────
	sb.WriteString("## 📊 Statistics\n\n")

	uniqueAuthors := map[string]bool{}
	tagFreq := map[string]int{}
	var totalSize int64
	var totalLoad int64
	statusCounts := map[int]int{}
	var maxDepth int

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
	}

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

	// ── Status Code Distribution ────────────
	if len(statusCounts) > 0 {
		sb.WriteString("### HTTP Status Codes\n\n")
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

	// ── Individual Pages ────────────────────
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

		// H1
		if p.H1 != "" && p.H1 != p.Title {
			sb.WriteString(fmt.Sprintf("**H1:** %s\n\n", escapeMDTableCell(p.H1)))
		}

		// Description
		if p.Description != "" {
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

		// OG Image
		if p.OGImage != "" {
			sb.WriteString(fmt.Sprintf("**Featured Image:** ![og](%s)\n\n", p.OGImage))
		}

		// Body text preview
		if p.BodyText != "" {
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
