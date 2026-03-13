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
		sitemapURL      string
		maxPages        int
		maxDepth        int
		concurrency     int
		delay           time.Duration
		timeout         time.Duration
		output          string
		verbose         bool
		noColor         bool
		forceRecursive  bool
		headless        bool
		minPriority     float64
		urlPattern      string

		enableChunker   bool
		chunkSize       int
		maxWorkers      int

		productMode     bool
		productPattern  string
		productSitemaps []string
		extractSpecs    bool
		extractImages   bool
		excludePatterns []string
	)

	cmd := &cobra.Command{
		Use:   "crawl <url>",
		Short: "Crawl a website using Spiderly core engine",
		Long: `Crawl a website using Spiderly core pipeline.

Supports:
- recursive crawling
- sitemap-based crawling
- product extraction mode
- chunked sitemap processing
- URL filtering and exclusion rules

Examples:
  spiderly crawl https://example.com
  spiderly crawl https://shop.com --product-mode --product-pattern "/product/"
  spiderly crawl https://example.com --sitemap-url https://example.com/sitemap.xml
  spiderly crawl https://example.com --force-recursive
  spiderly crawl https://shop.com --product-mode --extract-specs --extract-images
  spiderly crawl https://example.com --chunked --chunk-size 100 --max-workers 8
`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			targetURL := args[0]

			var compiledProductPattern *regexp.Regexp
			if productPattern != "" {
				re, err := regexp.Compile(productPattern)
				if err != nil {
					return fmt.Errorf("invalid --product-pattern regex: %w", err)
				}
				compiledProductPattern = re
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
				ExtractSpecs:           extractSpecs,
				ExtractImages:          extractImages,
				ExcludePatterns:        excludePatterns,
				ProductPattern:         productPattern,
				CompiledProductPattern: compiledProductPattern,
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
	cmd.Flags().BoolVar(&extractSpecs, "extract-specs", false, "extract product specifications")
	cmd.Flags().BoolVar(&extractImages, "extract-images", false, "extract product images")
	cmd.Flags().StringSliceVar(&excludePatterns, "exclude-patterns", nil, "regex patterns to exclude URLs")

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

  spiderly crawl https://example.com \
    --chunked \
    --chunk-size 100 \
    --max-workers 8
`)

	return cmd
}
