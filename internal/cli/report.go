package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"

	"spiderly/internal/engine"
	"spiderly/internal/models"
	"spiderly/internal/output"
	"spiderly/internal/report"

	"github.com/spf13/cobra"
)

func newReportCmd() *cobra.Command {
	var (
		input            string
		format           string
		out              string
		title            string
		sortBy           string
		filterCategory   string
		filterMinPrice   float64
		filterMaxPrice   float64
		filterInStock    bool
		filterHasProduct bool
		filterKeyword    string
	)

	cmd := &cobra.Command{
		Use:   "report",
		Short: "Generate a smart hierarchical report from crawl JSON",
		Long: `Generate rich visual Markdown or structured JSON reports from a crawl output file.
Supports hierarchical site trees, category breakdowns, sorting, and multi-criteria filtering.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if strings.TrimSpace(input) == "" {
				return errors.New("missing --input")
			}
			if strings.TrimSpace(format) == "" {
				format = "markdown"
			}

			data, err := os.ReadFile(input)
			if err != nil {
				return fmt.Errorf("read input: %w", err)
			}

			// Attempt 1: unmarshal as CrawlReport
			var rep report.CrawlReport
			var pages []models.ScrapedPage
			var startURL string

			if err := json.Unmarshal(data, &rep); err == nil && len(rep.Pages) > 0 {
				startURL = rep.Meta.StartURL
				for _, p := range rep.Pages {
					pages = append(pages, models.ScrapedPage{
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
					})
				}
			} else {
				// Attempt 2: unmarshal as []ScrapedPage / []ScrapedPageResult
				var scrapedPages []models.ScrapedPage
				if err := json.Unmarshal(data, &scrapedPages); err != nil {
					return fmt.Errorf("decode input JSON: %w", err)
				}
				pages = scrapedPages
				if len(pages) > 0 {
					startURL = pages[0].URL
				}
			}

			if len(pages) == 0 {
				return fmt.Errorf("no pages found in input file")
			}

			// Build tree & post-crawl engine
			tree := engine.BuildTree(pages, startURL)

			// Apply filtering if specified
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

			// Sort nodes
			sortMode := engine.ParseSortMode(sortBy)
			sortedNodes := engine.SortNodes(tree.FlattenDFS(), sortMode, tree)

			processedPages := make([]report.Page, 0, len(sortedNodes))
			maxDepth := 0
			for _, n := range sortedNodes {
				if n.Depth > maxDepth {
					maxDepth = n.Depth
				}
				p := n.Page
				if p == nil {
					continue
				}
				processedPages = append(processedPages, report.Page{
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
				})
			}

			prodStats := tree.GetProductSummary()
			finalReport := report.CrawlReport{
				Meta: report.Meta{
					StartURL: startURL,
				},
				Statistics: report.Statistics{
					TotalPages:    len(processedPages),
					TotalProducts: prodStats.TotalProducts,
					InStockCount:  prodStats.InStockCount,
					MaxDepth:      maxDepth,
				},
				Categories:    tree.GetCategorySummary(),
				ProductStats:  &prodStats,
				HierarchyTree: tree.Root,
				TreeASCII:     tree.RenderASCIITree(),
				Pages:         processedPages,
			}

			switch strings.ToLower(format) {
			case "markdown", "md":
				if title == "" {
					title = fmt.Sprintf("Spiderly Crawl Report: %s", startURL)
				}
				content, err := output.RenderMarkdown(finalReport, title)
				if err != nil {
					return err
				}
				if out == "" || out == "-" {
					fmt.Print(content)
					return nil
				}
				return os.WriteFile(out, []byte(content), 0o644)

			case "json":
				content, err := output.RenderJSON(finalReport, true)
				if err != nil {
					return err
				}
				if out == "" || out == "-" {
					fmt.Print(content)
					return nil
				}
				return os.WriteFile(out, []byte(content), 0o644)

			default:
				return fmt.Errorf("unsupported format: %s", format)
			}
		},
	}

	cmd.Flags().StringVar(&input, "input", "", "Input crawl JSON file")
	cmd.Flags().StringVar(&format, "format", "markdown", "Output format: markdown|json")
	cmd.Flags().StringVar(&out, "out", "", "Output file path, defaults to stdout")
	cmd.Flags().StringVar(&title, "title", "", "Report title")
	cmd.Flags().StringVar(&sortBy, "sort-by", "hierarchy", "Sort pages by: hierarchy, category, price, price-desc, title, depth, date, rating")
	cmd.Flags().StringVar(&filterCategory, "filter-category", "", "Filter pages matching category")
	cmd.Flags().Float64Var(&filterMinPrice, "filter-min-price", 0, "Filter products with minimum price")
	cmd.Flags().Float64Var(&filterMaxPrice, "filter-max-price", 0, "Filter products with maximum price")
	cmd.Flags().BoolVar(&filterInStock, "filter-in-stock", false, "Filter only in-stock products")
	cmd.Flags().BoolVar(&filterHasProduct, "filter-has-product", false, "Filter only pages containing products")
	cmd.Flags().StringVar(&filterKeyword, "filter-keyword", "", "Filter pages matching keyword")

	return cmd
}
