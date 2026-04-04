package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"strings"
	"time"

	"spiderly/internal/core"

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
		output         string
		verbose        bool
		noColor        bool
		forceRecursive bool
		headless       bool
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

			engine := core.NewCore(cfg)

			results, err := engine.Run()
			if err != nil {
				return fmt.Errorf("crawl failed: %w", err)
			}

			export := core.ToScrapedPageResults(results)

			data, err := json.MarshalIndent(export, "", "  ")
			if err != nil {
				return fmt.Errorf("failed to encode results: %w", err)
			}

			if output == "" || output == "-" {
				_, err = fmt.Fprintln(cmd.OutOrStdout(), string(data))
				return err
			}

			if err := os.WriteFile(output, data, 0o644); err != nil {
				return fmt.Errorf("failed to write output file: %w", err)
			}

			_, err = fmt.Fprintf(
				cmd.OutOrStdout(),
				"crawl complete: wrote %d pages to %s\n",
				len(export),
				output,
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
	cmd.Flags().BoolVar(&forceRecursive, "force-recursive", false, "force recursive crawling and skip sitemap discovery")
	cmd.Flags().BoolVar(&headless, "headless", false, "enable headless mode")
	cmd.Flags().BoolVarP(&verbose, "verbose", "v", false, "enable verbose logging")
	cmd.Flags().BoolVar(&noColor, "no-color", false, "disable colored console output")
	cmd.Flags().StringVarP(&output, "output", "o", "crawl.json", "output file path, use '-' for stdout")

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
