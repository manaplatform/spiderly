// internal/crawler/crawler_test.go
package crawler

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"spiderly/internal/models"
)

func TestCrawler_HeadlessAlways(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		switch r.URL.Path {
		case "/":
			fmt.Fprint(w, `<!DOCTYPE html>
<html>
<head><title>JS Home</title></head>
<body>
<div id="app">Initial</div>
<script>
	document.getElementById('app').innerHTML = '<p>Rendered by ChromeDP</p><a href="/page2">To Page 2</a>';
</script>
</body>
</html>`)
		case "/page2":
			fmt.Fprint(w, `<!DOCTYPE html>
<html>
<head><title>Page Two</title></head>
<body>
<p>Welcome to Page 2</p>
</body>
</html>`)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	cfg := DefaultConfig()
	cfg.Headless = true
	cfg.BrowserStrategy = "always"
	cfg.Timeout = 10 * time.Second
	cfg.MaxPages = 5
	cfg.MaxDepth = 2

	crwl, err := NewCrawler(cfg)
	if err != nil {
		t.Skipf("Chrome not available: %v", err)
		return
	}

	results, err := crwl.Crawl(server.URL)
	if err != nil {
		t.Fatalf("Crawl failed: %v", err)
	}

	if len(results) < 2 {
		t.Fatalf("expected at least 2 pages scraped, got %d", len(results))
	}

	foundHome := false
	foundPage2 := false
	for _, page := range results {
		if strings.Contains(page.Content, "Rendered by ChromeDP") || strings.Contains(page.Title, "JS Home") {
			foundHome = true
		}
		if strings.Contains(page.URL, "/page2") {
			foundPage2 = true
		}
	}

	if !foundHome {
		t.Errorf("expected home page with rendered JS content")
	}
	if !foundPage2 {
		t.Errorf("expected page2 to be discovered from rendered DOM and crawled")
	}
}

func TestCrawler_FallbackStrategy(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		switch r.URL.Path {
		case "/":
			// Static page with link to SPA
			fmt.Fprint(w, `<!DOCTYPE html>
<html>
<head><title>Static Root</title></head>
<body>
<h1>Static Content</h1>
<p>This is a plain HTML page that does not require JS.</p>
<a href="/spa">Go to SPA</a>
</body>
</html>`)
		case "/spa":
			// SPA requiring JS
			fmt.Fprint(w, `<!DOCTYPE html>
<html>
<head><title>SPA App</title></head>
<body>
<noscript>Please enable JavaScript to view this application.</noscript>
<div id="root"></div>
<script>
	document.getElementById('root').innerHTML = '<p>Hydrated SPA Content</p>';
</script>
</body>
</html>`)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	cfg := DefaultConfig()
	cfg.Headless = true
	cfg.BrowserStrategy = "fallback"
	cfg.Timeout = 10 * time.Second
	cfg.MaxPages = 5
	cfg.MaxDepth = 2

	crwl, err := NewCrawler(cfg)
	if err != nil {
		t.Skipf("Chrome not available: %v", err)
		return
	}

	results, err := crwl.Crawl(server.URL)
	if err != nil {
		t.Fatalf("Crawl failed: %v", err)
	}

	if len(results) < 2 {
		t.Fatalf("expected 2 pages scraped, got %d", len(results))
	}

	foundSPA := false
	for _, page := range results {
		if strings.Contains(page.URL, "/spa") {
			if strings.Contains(page.Content, "Hydrated SPA Content") {
				foundSPA = true
			}
		}
	}

	if !foundSPA {
		t.Errorf("expected /spa page to be re-rendered via ChromeDP fallback and contain hydrated content")
	}
}

func TestCrawler_ClickCrawl(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		switch r.URL.Path {
		case "/":
			fmt.Fprint(w, `<!DOCTYPE html>
<html>
<head><title>Interactive Page</title></head>
<body>
<div id="content">
	<button id="more-btn" onclick="addLink()">Show More Links</button>
</div>
<script>
	function addLink() {
		var a = document.createElement('a');
		a.href = '/clicked-link';
		a.textContent = 'Click Target';
		document.getElementById('content').appendChild(a);
	}
</script>
</body>
</html>`)
		case "/clicked-link":
			fmt.Fprint(w, `<!DOCTYPE html>
<html>
<head><title>Clicked Link Target</title></head>
<body>
<p>Target page content</p>
</body>
</html>`)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	cfg := DefaultConfig()
	cfg.Headless = true
	cfg.BrowserStrategy = "always"
	cfg.ClickCrawl = true
	cfg.MaxClicksPerPage = 5
	cfg.Timeout = 10 * time.Second
	cfg.MaxPages = 5
	cfg.MaxDepth = 2

	crwl, err := NewCrawler(cfg)
	if err != nil {
		t.Skipf("Chrome not available: %v", err)
		return
	}

	results, err := crwl.Crawl(server.URL)
	if err != nil {
		t.Fatalf("Crawl failed: %v", err)
	}

	foundClickedTarget := false
	for _, page := range results {
		if strings.Contains(page.URL, "/clicked-link") {
			foundClickedTarget = true
		}
	}

	if !foundClickedTarget {
		t.Errorf("expected /clicked-link to be discovered via click-crawl and scraped")
	}
}

func TestCrawler_RecursiveHierarchyAndLineage(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		switch r.URL.Path {
		case "/":
			fmt.Fprint(w, `<!DOCTYPE html>
<html>
<head><title>Home Root</title></head>
<body>
<h1>Welcome to Shop</h1>
<a href="/category/electronics">Electronics</a>
</body>
</html>`)
		case "/category/electronics":
			fmt.Fprint(w, `<!DOCTYPE html>
<html>
<head><title>Electronics Section</title></head>
<body>
<h1>Electronics</h1>
<a href="/category/electronics/laptop-1">Pro Laptop</a>
</body>
</html>`)
		case "/category/electronics/laptop-1":
			fmt.Fprint(w, `<!DOCTYPE html>
<html>
<head>
<title>Pro Laptop</title>
<script type="application/ld+json">
{
  "@context": "https://schema.org",
  "@type": "Product",
  "name": "Pro Ultra Laptop",
  "sku": "LAP-999",
  "offers": {
    "@type": "Offer",
    "price": "1299.99",
    "priceCurrency": "USD",
    "availability": "https://schema.org/InStock"
  }
}
</script>
</head>
<body>
<h1>Pro Ultra Laptop</h1>
<p>Top performance.</p>
</body>
</html>`)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	cfg := DefaultConfig()
	cfg.Headless = false
	cfg.MaxDepth = 3
	cfg.MaxPages = 10
	cfg.Delay = 0

	crwl, err := NewCrawler(cfg)
	if err != nil {
		t.Fatalf("failed to create crawler: %v", err)
	}

	results, err := crwl.Crawl(server.URL)
	if err != nil {
		t.Fatalf("crawl failed: %v", err)
	}

	if len(results) < 3 {
		t.Fatalf("expected 3 pages scraped, got %d", len(results))
	}

	pagesByURL := make(map[string]models.ScrapedPage)
	for _, p := range results {
		// Store by path for easier matching
		if strings.HasSuffix(p.URL, "/category/electronics/laptop-1") {
			pagesByURL["laptop"] = p
		} else if strings.HasSuffix(p.URL, "/category/electronics") {
			pagesByURL["category"] = p
		} else {
			pagesByURL["root"] = p
		}
	}

	rootPage, ok := pagesByURL["root"]
	if !ok {
		t.Fatalf("root page not found in results")
	}
	if rootPage.Depth != 0 {
		t.Errorf("expected root page depth=0, got %d", rootPage.Depth)
	}

	catPage, ok := pagesByURL["category"]
	if !ok {
		t.Fatalf("category page not found in results")
	}
	if catPage.Depth != 1 {
		t.Errorf("expected category page depth=1, got %d", catPage.Depth)
	}
	if !strings.HasPrefix(catPage.ParentURL, server.URL) {
		t.Errorf("expected category parent to be root URL %s, got %s", server.URL, catPage.ParentURL)
	}

	laptopPage, ok := pagesByURL["laptop"]
	if !ok {
		t.Fatalf("laptop page not found in results")
	}
	if laptopPage.Depth != 2 {
		t.Errorf("expected laptop page depth=2, got %d", laptopPage.Depth)
	}
	if !strings.Contains(laptopPage.ParentURL, "/category/electronics") {
		t.Errorf("expected laptop parent to be category URL, got %s", laptopPage.ParentURL)
	}

	// Verify structured data auto-extraction
	if laptopPage.StructuredData == nil {
		t.Fatalf("expected StructuredData on laptop page, got nil")
	}
	if laptopPage.StructuredData.Name != "Pro Ultra Laptop" {
		t.Errorf("expected StructuredData name 'Pro Ultra Laptop', got %q", laptopPage.StructuredData.Name)
	}
	if laptopPage.StructuredData.Price != 1299.99 {
		t.Errorf("expected StructuredData price 1299.99, got %f", laptopPage.StructuredData.Price)
	}
	if laptopPage.Product == nil {
		t.Fatalf("expected Product on laptop page, got nil")
	}
	if laptopPage.Product.Name != "Pro Ultra Laptop" {
		t.Errorf("expected Product name 'Pro Ultra Laptop', got %q", laptopPage.Product.Name)
	}
}
