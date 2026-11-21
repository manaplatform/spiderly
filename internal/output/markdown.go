package output

import (
	"fmt"
	"os"
	"strings"

	"spiderly/internal/report"
)

// RenderMarkdown produces a comprehensive, hierarchical Markdown report.
func RenderMarkdown(r report.CrawlReport, title string) (string, error) {
	var b strings.Builder

	if title == "" {
		title = "Spiderly Crawl & Site Hierarchy Report"
	}

	// 1. Header & Metadata
	fmt.Fprintf(&b, "# %s\n\n", title)
	fmt.Fprintf(&b, "- **Target URL**: `%s`\n", r.Meta.StartURL)
	if r.Meta.CrawlTime != "" {
		fmt.Fprintf(&b, "- **Crawl Time**: %s\n", r.Meta.CrawlTime)
	}
	if r.Meta.Duration != "" {
		fmt.Fprintf(&b, "- **Duration**: %s\n", r.Meta.Duration)
	}
	fmt.Fprintf(&b, "- **Total Pages Crawled**: %d\n", r.Statistics.TotalPages)
	if r.Statistics.TotalProducts > 0 {
		fmt.Fprintf(&b, "- **Products Discovered**: %d (%d in stock)\n", r.Statistics.TotalProducts, r.Statistics.InStockCount)
	}
	if r.Statistics.MaxDepth > 0 {
		fmt.Fprintf(&b, "- **Max Crawl Depth**: %d\n", r.Statistics.MaxDepth)
	}
	b.WriteString("\n---\n\n")

	// 2. Visual Site Hierarchy Tree
	if r.TreeASCII != "" {
		b.WriteString("## 🌲 Site Hierarchy Tree\n\n")
		b.WriteString("```text\n")
		b.WriteString(r.TreeASCII)
		b.WriteString("```\n\n")
	}

	// 3. Category & Section Breakdown
	if len(r.Categories) > 0 {
		b.WriteString("## 📁 Category & Section Breakdown\n\n")
		b.WriteString("| Category | Total Pages | Products | Price Range | Avg Price |\n")
		b.WriteString("|:---|:---:|:---:|:---:|:---:|\n")
		for _, cat := range r.Categories {
			priceRange := "—"
			avgPrice := "—"
			if cat.ProductCount > 0 && cat.MaxPrice > 0 {
				priceRange = fmt.Sprintf("%.2f – %.2f", cat.MinPrice, cat.MaxPrice)
				avgPrice = fmt.Sprintf("%.2f", cat.AvgPrice)
			}
			fmt.Fprintf(&b, "| **%s** | %d | %d | %s | %s |\n",
				cat.Name, cat.TotalPages, cat.ProductCount, priceRange, avgPrice)
		}
		b.WriteString("\n")
	}

	// 4. Structured Product Catalog
	var products []report.Page
	for _, p := range r.Pages {
		if p.Product != nil && (p.Product.Name != "" || p.Product.Price > 0 || p.Product.SKU != "") {
			products = append(products, p)
		}
	}

	if len(products) > 0 {
		b.WriteString("## 🛍️ Structured Product Catalog (JSON-LD Extracted)\n\n")
		b.WriteString("| Product Name | Category | Price | Status | Rating | SKU | Link |\n")
		b.WriteString("|:---|:---|:---:|:---:|:---:|:---:|:---|\n")
		for _, p := range products {
			prod := p.Product
			name := prod.Name
			if name == "" {
				name = p.Title
			}
			cat := p.Category
			if cat == "" {
				cat = prod.Category
			}
			if cat == "" {
				cat = "General"
			}

			priceStr := "—"
			if prod.Price > 0 {
				curr := prod.Currency
				if curr == "" {
					curr = "USD"
				}
				priceStr = fmt.Sprintf("%.2f %s", prod.Price, curr)
			}

			status := "—"
			if prod.Availability != "" {
				status = prod.Availability
			} else if prod.InStock {
				status = "InStock"
			}

			ratingStr := "—"
			if prod.Rating > 0 {
				ratingStr = fmt.Sprintf("★ %.1f", prod.Rating)
				if prod.ReviewCount > 0 {
					ratingStr += fmt.Sprintf(" (%d)", prod.ReviewCount)
				}
			}

			skuStr := prod.SKU
			if skuStr == "" {
				skuStr = "—"
			}

			fmt.Fprintf(&b, "| **%s** | %s | %s | %s | %s | `%s` | [View](%s) |\n",
				escapeMarkdown(name), escapeMarkdown(cat), priceStr, status, ratingStr, skuStr, p.URL)
		}
		b.WriteString("\n")
	}

	// 5. Crawled Pages Index
	if len(r.Pages) > 0 {
		b.WriteString("## 📄 Crawled Pages Index\n\n")
		b.WriteString("| Status | Depth | Category | Title / URL | Parent Page |\n")
		b.WriteString("|:---:|:---:|:---|:---|:---|\n")
		for _, p := range r.Pages {
			title := p.Title
			if title == "" {
				title = p.URL
			}
			cat := p.Category
			if cat == "" {
				cat = "—"
			}
			parent := p.ParentURL
			if parent == "" {
				parent = "(Root)"
			} else if len(parent) > 40 {
				parent = "..." + parent[len(parent)-37:]
			}

			fmt.Fprintf(&b, "| `%d` | %d | %s | [%s](%s) | `%s` |\n",
				p.StatusCode, p.Depth, cat, escapeMarkdown(title), p.URL, parent)
		}
		b.WriteString("\n")
	}

	return b.String(), nil
}

func WriteMarkdownFile(path string, r report.CrawlReport, title string) error {
	s, err := RenderMarkdown(r, title)
	if err != nil {
		return err
	}
	return os.WriteFile(path, []byte(s), 0o644)
}

func escapeMarkdown(s string) string {
	s = strings.ReplaceAll(s, "|", "\\|")
	s = strings.ReplaceAll(s, "\n", " ")
	return strings.TrimSpace(s)
}
