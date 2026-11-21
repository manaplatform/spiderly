// internal/cli/cli_test.go
package cli

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"spiderly/internal/core"
)

func TestCLICrawl_RecursiveHierarchyAndMarkdown(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		switch r.URL.Path {
		case "/":
			fmt.Fprint(w, `<!DOCTYPE html>
<html>
<head><title>E-Commerce Home</title></head>
<body>
<nav aria-label="breadcrumb"><ol><li>Home</li></ol></nav>
<a href="/category/gear">Gear Category</a>
<a href="/category/apparel">Apparel Category</a>
</body>
</html>`)
		case "/category/gear":
			fmt.Fprint(w, `<!DOCTYPE html>
<html>
<head><title>Gear Category</title></head>
<body>
<nav aria-label="breadcrumb"><ol><li>Home</li><li>Gear</li></ol></nav>
<a href="/category/gear/backpack">Pro Backpack</a>
</body>
</html>`)
		case "/category/gear/backpack":
			fmt.Fprint(w, `<!DOCTYPE html>
<html>
<head>
<title>Pro Backpack</title>
<script type="application/ld+json">
{
  "@context": "https://schema.org",
  "@type": "Product",
  "name": "Ultra Trail Backpack 30L",
  "sku": "BP-30L",
  "brand": "TrailCorp",
  "offers": {
    "@type": "Offer",
    "price": "149.95",
    "priceCurrency": "USD",
    "availability": "https://schema.org/InStock"
  },
  "aggregateRating": {
    "@type": "AggregateRating",
    "ratingValue": 4.9,
    "reviewCount": 85
  }
}
</script>
</head>
<body>
<h1>Ultra Trail Backpack 30L</h1>
</body>
</html>`)
		case "/category/apparel":
			fmt.Fprint(w, `<!DOCTYPE html>
<html>
<head><title>Apparel Category</title></head>
<body>
<nav aria-label="breadcrumb"><ol><li>Home</li><li>Apparel</li></ol></nav>
<a href="/category/apparel/jacket">Winter Jacket</a>
</body>
</html>`)
		case "/category/apparel/jacket":
			fmt.Fprint(w, `<!DOCTYPE html>
<html>
<head>
<title>Winter Jacket</title>
<script type="application/ld+json">
{
  "@context": "https://schema.org",
  "@type": "Product",
  "name": "Alpine Down Jacket",
  "sku": "JKT-DOWN",
  "brand": "PeakWear",
  "offers": {
    "@type": "Offer",
    "price": "299.00",
    "priceCurrency": "USD",
    "availability": "https://schema.org/OutOfStock"
  }
}
</script>
</head>
<body>
<h1>Alpine Down Jacket</h1>
</body>
</html>`)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	tmpDir := t.TempDir()
	jsonOut := filepath.Join(tmpDir, "crawl.json")
	mdOut := filepath.Join(tmpDir, "report.md")

	crawlCmd := newCrawlCmd()
	crawlCmd.SetArgs([]string{
		server.URL,
		"-r",
		"--max-depth", "3",
		"--delay", "0s",
		"-o", jsonOut,
		"-m", mdOut,
	})

	if err := crawlCmd.Execute(); err != nil {
		t.Fatalf("crawlCmd failed: %v", err)
	}

	// 1. Verify JSON output
	data, err := os.ReadFile(jsonOut)
	if err != nil {
		t.Fatalf("failed to read json output: %v", err)
	}

	var results []core.ScrapedPageResult
	if err := json.Unmarshal(data, &results); err != nil {
		t.Fatalf("failed to unmarshal crawl.json: %v", err)
	}

	if len(results) < 5 {
		t.Errorf("expected at least 5 pages, got %d", len(results))
	}

	foundBackpack := false
	for _, p := range results {
		if strings.Contains(p.URL, "/category/gear/backpack") {
			foundBackpack = true
			if p.Depth != 2 {
				t.Errorf("expected backpack depth 2, got %d", p.Depth)
			}
			if !strings.Contains(p.ParentURL, "/category/gear") {
				t.Errorf("expected parent to be /category/gear, got %s", p.ParentURL)
			}
			if p.Product == nil || p.Product.Name != "Ultra Trail Backpack 30L" {
				t.Errorf("expected Product info for backpack, got %+v", p.Product)
			}
			if p.StructuredData == nil || p.StructuredData.Price != 149.95 {
				t.Errorf("expected StructuredData price 149.95, got %+v", p.StructuredData)
			}
		}
	}
	if !foundBackpack {
		t.Errorf("backpack product page not found in crawl.json")
	}

	// 2. Verify Markdown output
	mdData, err := os.ReadFile(mdOut)
	if err != nil {
		t.Fatalf("failed to read report.md: %v", err)
	}
	mdContent := string(mdData)

	if !strings.Contains(mdContent, "Site Hierarchy Tree") {
		t.Errorf("expected Site Hierarchy Tree in report.md")
	}
	if !strings.Contains(mdContent, "Ultra Trail Backpack 30L") {
		t.Errorf("expected Ultra Trail Backpack 30L in report.md")
	}
	if !strings.Contains(mdContent, "Category & Section Breakdown") {
		t.Errorf("expected Category & Section Breakdown in report.md")
	}
	if !strings.Contains(mdContent, "Structured Product Catalog") {
		t.Errorf("expected Structured Product Catalog table in report.md")
	}

	// 3. Test report CLI command with filtering & sorting
	reportMdOut := filepath.Join(tmpDir, "filtered_report.md")
	reportCmd := newReportCmd()
	reportCmd.SetArgs([]string{
		"--input", jsonOut,
		"--out", reportMdOut,
		"--format", "markdown",
		"--sort-by", "price",
		"--filter-in-stock",
	})

	if err := reportCmd.Execute(); err != nil {
		t.Fatalf("reportCmd failed: %v", err)
	}

	filteredData, err := os.ReadFile(reportMdOut)
	if err != nil {
		t.Fatalf("failed to read filtered_report.md: %v", err)
	}
	filteredContent := string(filteredData)

	if !strings.Contains(filteredContent, "Ultra Trail Backpack 30L") {
		t.Errorf("expected in-stock backpack in filtered report")
	}
	if strings.Contains(filteredContent, "Alpine Down Jacket") {
		t.Errorf("out-of-stock jacket should be excluded by --filter-in-stock")
	}
}
