package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"strings"
	"time"

	"spiderly/internal/core"
	"spiderly/internal/engine"
	"spiderly/internal/models"
	"spiderly/internal/output"
	"spiderly/internal/report"

	"github.com/spf13/cobra"
)

func newCrawlCmd() *cobra.Command {
	var (
		sitemapURL     string
		maxPages       int
		maxDepth       int
		concurrency    int
		delay          time.Duration
		timeout        time.Duration
		outputPath     string
		verbose        bool
		noColor        bool
		forceRecursive bool
		headless       bool
		browserVisible   bool
		browserTimeout   time.Duration
		browserStrategy  string
		clickCrawl       bool
		clickFallback    bool
		maxClicksPerPage int
		minPriority    float64
		urlPattern     string

		enableChunker bool
		chunkSize     int
		maxWorkers    int

		productMode     bool
		productPattern  string
		productSitemaps []string
		newsMode        bool
		newsPattern     string
		newsSitemaps    []string
		extractSpecs    bool
		extractImages   bool
		excludePatterns []string
		proxyValues     []string

		sortBy           string
		filterCategory   string
		filterMinPrice   float64
		filterMaxPrice   float64
		filterInStock    bool
		filterHasProduct bool
		filterKeyword    string
		markdownOutput   string
	)

	cmd := &cobra.Command{
		Use:   "crawl <url>",
		Short: "Crawl a website using Spiderly core engine",
		Long: `Crawl a website using Spiderly core pipeline.

Supports:
- recursive crawling
- sitemap-based crawling
- product extraction mode
- news extraction mode
- chunked sitemap processing
- URL filtering and exclusion rules
- HTTP proxy support (CLI or SPIDERLY_PROXY)

Examples:
  spiderly crawl https://example.com
  spiderly crawl https://shop.com --product-mode --product-pattern "/product/"
  spiderly crawl https://example.com --sitemap-url https://example.com/sitemap.xml
  spiderly crawl https://example.com --force-recursive
  spiderly crawl https://shop.com --product-mode --extract-specs --extract-images
  spiderly crawl https://example.com --chunked --chunk-size 100 --max-workers 8
  spiderly crawl https://news.example.com --news-mode --news-pattern "/news|article/"
  spiderly crawl https://example.com --proxy http://127.0.0.1:8080
  SPIDERLY_PROXY=http://p1:8080,http://p2:8080 spiderly crawl https://example.com --chunked --max-workers 4
`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			targetURL := args[0]
			resolvedProxies := resolveProxyList(proxyValues, os.Getenv("SPIDERLY_PROXY"))

			if productMode && newsMode {
				return fmt.Errorf("--product-mode and --news-mode cannot both be enabled")
			}

			if clickFallback {
				clickCrawl = true
				headless = true
				browserStrategy = "fallback"
			} else if clickCrawl && !headless {
				headless = true
			}

			if browserTimeout <= 0 {
				browserTimeout = timeout
			}
			if browserStrategy == "" {
				browserStrategy = "always"
			}

			var compiledProductPattern *regexp.Regexp
			if productPattern != "" {
				re, err := regexp.Compile(productPattern)
				if err != nil {
					return fmt.Errorf("invalid --product-pattern regex: %w", err)
				}
				compiledProductPattern = re
			}

			var compiledNewsPattern *regexp.Regexp
			if newsPattern != "" {
				re, err := regexp.Compile(newsPattern)
				if err != nil {
					return fmt.Errorf("invalid --news-pattern regex: %w", err)
				}
				compiledNewsPattern = re
			}

			cfg := core.CoreConfig{
				TargetURL:              targetURL,
				SitemapURL:             sitemapURL,
				MaxPages:               maxPages,
				MaxDepth:               maxDepth,
				Concurrency:            concurrency,
				Delay:                  delay,
				Timeout:                timeout,
				MinPriority:            minPriority,
				URLPattern:             urlPattern,
				ForceRecursive:         forceRecursive,
				Headless:               headless,
				BrowserVisible:         browserVisible,
				BrowserTimeout:         browserTimeout,
				BrowserStrategy:        browserStrategy,
				ClickCrawl:             clickCrawl,
				MaxClicksPerPage:       maxClicksPerPage,
				Verbose:                verbose,
				NoColor:                noColor,
				EnableChunker:          enableChunker,
				ChunkSize:              chunkSize,
				MaxWorkers:             maxWorkers,
				ProductMode:            productMode,
				ProductSitemaps:        productSitemaps,
				NewsMode:               newsMode,
				NewsSitemaps:           newsSitemaps,
				ExtractSpecs:           extractSpecs,
				ExtractImages:          extractImages,
				ExcludePatterns:        excludePatterns,
				Proxies:                resolvedProxies,
				ProductPattern:         productPattern,
				NewsPattern:            newsPattern,
				CompiledProductPattern: compiledProductPattern,
				CompiledNewsPattern:    compiledNewsPattern,
			}

			coreEngine := core.NewCore(cfg)

			results, err := coreEngine.Run()
			if err != nil {
				return fmt.Errorf("crawl failed: %w", err)
			}

			// 1. Build crawl tree & enrich pages
			tree := engine.BuildTree(results, targetURL)

			// 2. Apply filtering if requested
			var filterOpts engine.FilterOptions
			hasFilter := false
			if filterCategory != "" {
				filterOpts.Category = filterCategory
				hasFilter = true
			}
			if filterMinPrice > 0 {
				filterOpts.MinPrice = &filterMinPrice
				hasFilter = true
			}
			if filterMaxPrice > 0 {
				filterOpts.MaxPrice = &filterMaxPrice
				hasFilter = true
			}
			if filterInStock {
				filterOpts.InStockOnly = true
				hasFilter = true
			}
			if filterHasProduct {
				filterOpts.HasProduct = true
				hasFilter = true
			}
			if filterKeyword != "" {
				filterOpts.Keywords = []string{filterKeyword}
				hasFilter = true
			}

			if hasFilter {
				tree = engine.FilterTree(tree, filterOpts)
			}

			// 3. Sort nodes
			sortMode := engine.ParseSortMode(sortBy)
			sortedNodes := engine.SortNodes(tree.FlattenDFS(), sortMode, tree)

			processedPages := make([]models.ScrapedPage, 0, len(sortedNodes))
			for _, n := range sortedNodes {
				if n.Page != nil {
					processedPages = append(processedPages, *n.Page)
				}
			}

			// 4. Generate Markdown report if requested
			if markdownOutput != "" {
				prodSummary := tree.GetProductSummary()
				crawlReport := report.CrawlReport{
					Meta: report.Meta{
						StartURL:  targetURL,
						CrawlTime: time.Now().Format("2006-01-02 15:04:05"),
					},
					Statistics: report.Statistics{
						TotalPages:    len(processedPages),
						TotalProducts: prodSummary.TotalProducts,
						InStockCount:  prodSummary.InStockCount,
						MaxDepth:      maxDepth,
					},
					Categories:    tree.GetCategorySummary(),
					ProductStats:  &prodSummary,
					HierarchyTree: tree.Root,
					TreeASCII:     tree.RenderASCIITree(),
					Pages:         make([]report.Page, len(processedPages)),
				}
				for i, p := range processedPages {
					crawlReport.Pages[i] = report.Page{
						URL:            p.URL,
						Title:          p.Title,
						StatusCode:     p.StatusCode,
						Depth:          p.Depth,
						ParentURL:      p.ParentURL,
						Category:       p.Category,
						Breadcrumbs:    p.Breadcrumbs,
						PageType:       p.PageType,
						Product:        p.Product,
						StructuredData: p.StructuredData,
					}
				}
				if err := output.WriteMarkdownFile(markdownOutput, crawlReport, fmt.Sprintf("Spiderly Crawl Report: %s", targetURL)); err != nil {
					return fmt.Errorf("failed to write markdown report: %w", err)
				}
				_, _ = fmt.Fprintf(cmd.OutOrStdout(), "wrote markdown report to %s\n", markdownOutput)
			}

			export := core.ToScrapedPageResults(processedPages)

			data, err := json.MarshalIndent(export, "", "  ")
			if err != nil {
				return fmt.Errorf("failed to encode results: %w", err)
			}

			if outputPath == "" || outputPath == "-" {
				_, err = fmt.Fprintln(cmd.OutOrStdout(), string(data))
				return err
			}

			if err := os.WriteFile(outputPath, data, 0o644); err != nil {
				return fmt.Errorf("failed to write output file: %w", err)
			}

			_, err = fmt.Fprintf(
				cmd.OutOrStdout(),
				"crawl complete: wrote %d pages to %s\n",
				len(export),
				outputPath,
			)
			return err
		},
	}

	cmd.Flags().StringVar(&sitemapURL, "sitemap-url", "", "direct sitemap URL to use instead of discovery")
	cmd.Flags().IntVar(&maxPages, "max-pages", 100, "maximum number of pages to crawl")
	cmd.Flags().IntVar(&maxDepth, "max-depth", 3, "maximum recursive crawl depth")
	cmd.Flags().IntVar(&concurrency, "concurrency", 5, "number of concurrent requests")
	cmd.Flags().DurationVar(&delay, "delay", 200*time.Millisecond, "delay between requests")
	cmd.Flags().DurationVar(&timeout, "timeout", 30*time.Second, "request timeout")
	cmd.Flags().Float64Var(&minPriority, "min-priority", 0, "minimum sitemap priority filter")
	cmd.Flags().StringVar(&urlPattern, "url-pattern", "", "regex filter for sitemap URLs")
	cmd.Flags().BoolVarP(&forceRecursive, "force-recursive", "r", false, "force recursive crawling and skip sitemap discovery")
	cmd.Flags().BoolVar(&headless, "headless", false, "enable headless browser rendering via ChromeDP")
	cmd.Flags().BoolVar(&browserVisible, "browser-visible", false, "show browser window instead of headless (useful for debugging)")
	cmd.Flags().DurationVar(&browserTimeout, "browser-timeout", 0, "browser rendering and navigation timeout (defaults to --timeout)")
	cmd.Flags().StringVar(&browserStrategy, "browser-strategy", "always", "browser rendering strategy: 'always' (render all pages in ChromeDP) or 'fallback' (Colly fast path, ChromeDP when JS is required)")
	cmd.Flags().BoolVar(&clickCrawl, "click-crawl", false, "enable click-based link discovery for interactive elements (buttons, SPA nav, load more)")
	cmd.Flags().BoolVar(&clickFallback, "click-fallback", false, "convenience shortcut: enables headless + click-crawl with fallback strategy")
	cmd.Flags().IntVar(&maxClicksPerPage, "max-clicks-per-page", 10, "maximum interactive elements to click per page")
	cmd.Flags().BoolVarP(&verbose, "verbose", "v", false, "enable verbose logging")
	cmd.Flags().BoolVar(&noColor, "no-color", false, "disable colored console output")
	cmd.Flags().StringVarP(&outputPath, "output", "o", "crawl.json", "output file path, use '-' for stdout")

	cmd.Flags().StringVar(&sortBy, "sort-by", "hierarchy", "sort pages by: hierarchy, category, price, price-desc, title, depth, date, rating")
	cmd.Flags().StringVar(&filterCategory, "filter-category", "", "filter pages matching category")
	cmd.Flags().Float64Var(&filterMinPrice, "filter-min-price", 0, "filter products with minimum price")
	cmd.Flags().Float64Var(&filterMaxPrice, "filter-max-price", 0, "filter products with maximum price")
	cmd.Flags().BoolVar(&filterInStock, "filter-in-stock", false, "filter only in-stock products")
	cmd.Flags().BoolVar(&filterHasProduct, "filter-has-product", false, "filter only pages containing products")
	cmd.Flags().StringVar(&filterKeyword, "filter-keyword", "", "filter pages matching keyword in title, body, or product name")
	cmd.Flags().StringVarP(&markdownOutput, "markdown", "m", "", "file path to write hierarchical Markdown report")

	cmd.Flags().BoolVar(&enableChunker, "chunked", false, "enable chunked sitemap processing")
	cmd.Flags().IntVar(&chunkSize, "chunk-size", 50, "number of URLs per chunk when chunker is enabled")
	cmd.Flags().IntVar(&maxWorkers, "max-workers", 4, "maximum number of chunk workers")

	cmd.Flags().BoolVar(&productMode, "product-mode", false, "enable product extraction mode")
	cmd.Flags().StringVar(&productPattern, "product-pattern", "", "regex pattern for product URLs")
	cmd.Flags().StringSliceVar(&productSitemaps, "product-sitemaps", nil, "preferred sitemap keywords for product mode, e.g. pdp,product")
	cmd.Flags().BoolVar(&newsMode, "news-mode", false, "enable news extraction mode")
	cmd.Flags().StringVar(&newsPattern, "news-pattern", "", "regex pattern for news/article URLs")
	cmd.Flags().StringSliceVar(&newsSitemaps, "news-sitemaps", nil, "preferred sitemap keywords for news mode, e.g. news,press")
	cmd.Flags().BoolVar(&extractSpecs, "extract-specs", false, "extract product specifications")
	cmd.Flags().BoolVar(&extractImages, "extract-images", false, "extract product images")
	cmd.Flags().StringSliceVar(&excludePatterns, "exclude-patterns", nil, "regex patterns to exclude URLs")
	cmd.Flags().StringSliceVar(&proxyValues, "proxy", nil, "proxy URL(s), supports repeated/comma-separated values, e.g. --proxy http://p1:8080 --proxy http://p2:8080")

	cmd.Example = strings.TrimSpace(`
  spiderly crawl https://example.com

  spiderly crawl https://example.com \
    --max-pages 200 \
    --max-depth 5 \
    --concurrency 10

  spiderly crawl https://example.com \
    --sitemap-url https://example.com/sitemap.xml \
    --min-priority 0.5 \
    --url-pattern "/blog/"

  spiderly crawl https://shop.com \
    --product-mode \
    --product-pattern "/product/" \
    --extract-specs \
    --extract-images

  spiderly crawl https://shop.com \
    --product-mode \
    --product-sitemaps pdp,product \
    --exclude-patterns "/cart/","/account/"

  spiderly crawl https://news.example.com \
    --news-mode \
    --news-pattern "/news|article/" \
    --news-sitemaps news,press,headline

  spiderly crawl https://example.com \
    --chunked \
    --chunk-size 100 \
    --max-workers 8

  spiderly crawl https://app.example.com --headless

  spiderly crawl https://spa.example.com --headless --click-crawl --max-clicks-per-page 15

  spiderly crawl https://example.com --click-fallback

  spiderly crawl https://example.com \
    --proxy http://127.0.0.1:8080

  SPIDERLY_PROXY=http://p1:8080,http://p2:8080 spiderly crawl https://example.com \
    --chunked \
    --max-workers 4
`)

	return cmd
}

func resolveProxyList(cliValues []string, envValue string) []string {
	if proxies := normalizeProxyList(cliValues); len(proxies) > 0 {
		return proxies
	}

	if strings.TrimSpace(envValue) == "" {
		return nil
	}

	return normalizeProxyList([]string{envValue})
}

func normalizeProxyList(values []string) []string {
	seen := make(map[string]struct{})
	normalized := make([]string, 0)

	for _, value := range values {
		for _, candidate := range strings.Split(value, ",") {
			proxy := strings.TrimSpace(candidate)
			if proxy == "" {
				continue
			}
			if _, exists := seen[proxy]; exists {
				continue
			}
			seen[proxy] = struct{}{}
			normalized = append(normalized, proxy)
		}
	}

	if len(normalized) == 0 {
		return nil
	}

	return normalized
}
