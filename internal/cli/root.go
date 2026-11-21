package cli

import (
	"fmt"
	"os"
	"strings"
	"time"

	"spiderly/internal/core"
	"spiderly/internal/engine"
	"spiderly/internal/output"
	"spiderly/internal/report"

	"github.com/spf13/cobra"
)

var (
	version = "v0.0.1"
	commit  = "none"
	date    = "2026.11.21"
)

func SetBuildInfo(v, c, d string) {
	version = v
	commit = c
	date = d
}

func Execute() error {
	return newRootCmd().Execute()
}

func newRootCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "spiderly",
		Short: "Spiderly is a website crawler and reporting tool",
		Long: `Spiderly crawls websites and produces structured reports.

Examples:
  spiderly crawl https://example.com
  spiderly report --input crawl.json --format markdown
  spiderly run https://example.com
  spiderly version`,
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) > 0 {
				runCmd := newRunCmd()
				runCmd.SetArgs(args)
				return runCmd.Execute()
			}

			printWelcomeBanner()
			return cmd.Help()
		},
	}

	cmd.AddCommand(newCrawlCmd())
	cmd.AddCommand(newReportCmd())
	cmd.AddCommand(newRunCmd())
	cmd.AddCommand(newVersionCmd())

	return cmd
}

func buildVersionString() string {
	return fmt.Sprintf("version=%s commit=%s date=%s", version, commit, date)
}

func newRunCmd() *cobra.Command {
	var (
		format           string
		outputFile       string
		sitemapURL       string
		maxPages         int
		maxDepth         int
		concurrency      int
		delay            time.Duration
		timeout          time.Duration
		forceRecursive   bool
		headless         bool
		browserVisible   bool
		browserTimeout   time.Duration
		browserStrategy  string
		clickCrawl       bool
		clickFallback    bool
		maxClicksPerPage int
		verbose          bool
		noColor          bool
		proxyValues      []string
	)

	cmd := &cobra.Command{
		Use:   "run <url>",
		Short: "Run crawl + report in one command",
		Long:  "Run performs a crawl and then generates a report with a polished terminal experience.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			targetURL := args[0]
			resolvedProxies := resolveProxyList(proxyValues, os.Getenv("SPIDERLY_PROXY"))

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

			printRunHeader(targetURL)

			step("Validating target", targetURL)
			cfg := core.CoreConfig{
				TargetURL:        targetURL,
				SitemapURL:       sitemapURL,
				MaxPages:         maxPages,
				MaxDepth:         maxDepth,
				Concurrency:      concurrency,
				Delay:            delay,
				Timeout:          timeout,
				ForceRecursive:   forceRecursive,
				Headless:         headless,
				BrowserVisible:   browserVisible,
				BrowserTimeout:   browserTimeout,
				BrowserStrategy:  browserStrategy,
				ClickCrawl:       clickCrawl,
				MaxClicksPerPage: maxClicksPerPage,
				Verbose:          verbose,
				NoColor:          noColor,
				Proxies:          resolvedProxies,
			}

			coreEngine := core.NewCore(cfg)

			step("Starting crawler", fmt.Sprintf("target=%s headless=%v", targetURL, headless))
			results, err := coreEngine.Run()
			if err != nil {
				return fmt.Errorf("crawl failed: %w", err)
			}

			step("Analyzing results", fmt.Sprintf("processing %d crawled pages", len(results)))
			tree := engine.BuildTree(results, targetURL)
			dfsNodes := tree.FlattenDFS()
			prodStats := tree.GetProductSummary()

			crawlReport := report.CrawlReport{
				Meta: report.Meta{
					StartURL:  targetURL,
					CrawlTime: time.Now().Format("2006-01-02 15:04:05"),
				},
				Statistics: report.Statistics{
					TotalPages:    len(dfsNodes),
					TotalProducts: prodStats.TotalProducts,
					InStockCount:  prodStats.InStockCount,
					MaxDepth:      maxDepth,
				},
				Categories:    tree.GetCategorySummary(),
				ProductStats:  &prodStats,
				HierarchyTree: tree.Root,
				TreeASCII:     tree.RenderASCIITree(),
				Pages:         make([]report.Page, len(dfsNodes)),
			}
			for i, n := range dfsNodes {
				p := n.Page
				if p == nil {
					continue
				}
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

			step("Rendering report", fmt.Sprintf("format=%s output=%s", format, outputFile))
			var reportContent string
			switch strings.ToLower(format) {
			case "json":
				reportContent, err = output.RenderJSON(crawlReport, true)
			default:
				reportContent, err = output.RenderMarkdown(crawlReport, fmt.Sprintf("Spiderly Crawl Report: %s", targetURL))
			}
			if err != nil {
				return fmt.Errorf("render report: %w", err)
			}

			if outputFile == "" || outputFile == "-" {
				fmt.Fprintln(cmd.OutOrStdout(), reportContent)
			} else {
				if err := os.WriteFile(outputFile, []byte(reportContent), 0o644); err != nil {
					return fmt.Errorf("write report: %w", err)
				}
			}

			printRunSuccess(targetURL, format, outputFile, len(results))
			return nil
		},
	}

	cmd.Flags().StringVarP(&format, "format", "f", "markdown", "Report format: markdown|json")
	cmd.Flags().StringVarP(&outputFile, "output", "o", "report.md", "Output report file, use '-' for stdout")
	cmd.Flags().StringVar(&sitemapURL, "sitemap-url", "", "direct sitemap URL to use instead of discovery")
	cmd.Flags().IntVar(&maxPages, "max-pages", 100, "maximum number of pages to crawl")
	cmd.Flags().IntVar(&maxDepth, "max-depth", 3, "maximum recursive crawl depth")
	cmd.Flags().IntVar(&concurrency, "concurrency", 5, "number of concurrent requests")
	cmd.Flags().DurationVar(&delay, "delay", 200*time.Millisecond, "delay between requests")
	cmd.Flags().DurationVar(&timeout, "timeout", 30*time.Second, "request timeout")
	cmd.Flags().BoolVar(&forceRecursive, "force-recursive", false, "force recursive crawling and skip sitemap discovery")
	cmd.Flags().BoolVar(&headless, "headless", false, "enable headless browser rendering via ChromeDP")
	cmd.Flags().BoolVar(&browserVisible, "browser-visible", false, "show browser window instead of headless (useful for debugging)")
	cmd.Flags().DurationVar(&browserTimeout, "browser-timeout", 0, "browser rendering and navigation timeout (defaults to --timeout)")
	cmd.Flags().StringVar(&browserStrategy, "browser-strategy", "always", "browser rendering strategy: 'always' (render all pages in ChromeDP) or 'fallback' (Colly fast path, ChromeDP when JS is required)")
	cmd.Flags().BoolVar(&clickCrawl, "click-crawl", false, "enable click-based link discovery for interactive elements (buttons, SPA nav, load more)")
	cmd.Flags().BoolVar(&clickFallback, "click-fallback", false, "convenience shortcut: enables headless + click-crawl with fallback strategy")
	cmd.Flags().IntVar(&maxClicksPerPage, "max-clicks-per-page", 10, "maximum interactive elements to click per page")
	cmd.Flags().BoolVarP(&verbose, "verbose", "v", false, "enable verbose logging")
	cmd.Flags().BoolVar(&noColor, "no-color", false, "disable colored console output")
	cmd.Flags().StringSliceVar(&proxyValues, "proxy", nil, "proxy URL(s), supports repeated/comma-separated values")

	return cmd
}

func printWelcomeBanner() {
	fmt.Println()
	fmt.Println(spiderBanner())
	fmt.Println("  Spiderly - Website Crawler and Reporting Tool")
	fmt.Println("  " + strings.Repeat("─", 56))
	fmt.Printf("  Build : %s\n", buildVersionString())
	fmt.Println("  Ready to crawl the web.")
	fmt.Println("  " + strings.Repeat("─", 56))
	fmt.Println()
}

func printRunHeader(target string) {
	fmt.Println()
	fmt.Println(spiderBanner())
	fmt.Println("  Spiderly Run Mode")
	fmt.Println("  " + strings.Repeat("─", 56))
	fmt.Printf("  Target : %s\n", target)
	fmt.Printf("  Build  : %s\n", buildVersionString())
	fmt.Println("  " + strings.Repeat("─", 56))
	fmt.Println()
}

func step(title, detail string) {
	fmt.Printf("  [*] %-20s %s\n", title, detail)
}

func printRunSuccess(target, format, output string, pageCount int) {
	fmt.Println()
	fmt.Printf("  [✓] Crawl completed successfully (%d pages)\n", pageCount)
	fmt.Printf("  [✓] Target : %s\n", target)
	fmt.Printf("  [✓] Format : %s\n", format)
	fmt.Printf("  [✓] Output : %s\n", output)
	fmt.Println()
	fmt.Println("  Your web has been neatly trapped in the Spiderly report.")
	fmt.Println()
}

func spiderBanner() string {
	return `
                          .     .
                         / \   / \
                        /   \ /   \
                       /  .--.--.  \
                      /  /  ()  \  \
                     /  /   ||   \  \
                    /__/    ||    \__\
                   '---.   /  \   .---'
                        \ /    \ /
                         '      '

     ███████╗██████╗ ██╗██████╗ ███████╗██████╗ ██╗  ██╗   ██╗
     ██╔════╝██╔══██╗██║██╔══██╗██╔════╝██╔══██╗██║  ╚██╗ ██╔╝
     ███████╗██████╔╝██║██║  ██║█████╗  ██████╔╝██║   ╚████╔╝
     ╚════██║██╔═══╝ ██║██║  ██║██╔══╝  ██╔══██╗██║    ╚██╔╝
     ███████║██║     ██║██████╔╝███████╗██║  ██║███████╗ ██║
     ╚══════╝╚═╝     ╚═╝╚═════╝ ╚══════╝╚═╝  ╚═╝╚══════╝ ╚═╝
`
}
